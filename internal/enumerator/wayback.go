// Package enumerator provides interfaces and implementations for discovering URLs from various sources.
//
// The Wayback enumerator fetches historical URLs from web archive sources including:
// - Wayback Machine (web.archive.org)
// - Common Crawl (index.commoncrawl.org)
// - VirusTotal (requires API key, currently not implemented)
//
// Example usage:
//
//	import (
//		"context"
//		"github.com/raysh454/moku/internal/enumerator"
//		"github.com/raysh454/moku/internal/logging"
//		"github.com/raysh454/moku/internal/webclient"
//	)
//
//	// Create a webclient
//	cfg := webclient.Config{Client: webclient.ClientNetHTTP}
//	logger := logging.NewStdoutLogger("app")
//	wc, _ := webclient.NewNetHTTPClient(cfg, logger, nil)
//
//	// Create and use Wayback enumerator
//	wayback := enumerator.NewWayback(wc, logger)
//	urls, _ := wayback.Enumerate(context.Background(), "https://example.com", nil)
//
//	// Use with Composite enumerator for multiple sources
//	robots := enumerator.NewRobots(wc, logger)
//	sitemap := enumerator.NewSitemap(wc, logger)
//	composite := enumerator.NewComposite(
//		[]enumerator.Enumerator{wayback, robots, sitemap},
//		logger,
//	)
//	allURLs, _ := composite.Enumerate(context.Background(), "https://example.com", nil)
package enumerator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

// waybackResponse represents a single row from the Wayback Machine CDX API.
// The API returns an array of arrays where the first row is headers.
type waybackResponse [][]string

// commonCrawlItem represents a single item from Common Crawl index API.
type commonCrawlItem struct {
	URL string `json:"url"`
}

// Wayback enumerates URLs from archive sources: Wayback Machine, Common Crawl, and VirusTotal.
type Wayback struct {
	wc             webclient.WebClient
	logger         logging.Logger
	waybackBaseURL string
	ccBaseURL      string
	vtBaseURL      string
}

// WaybackConfig holds configuration for the Wayback enumerator.
type WaybackConfig struct {
	WaybackBaseURL string // Optional, defaults to http://web.archive.org
	CCBaseURL      string // Optional, defaults to http://index.commoncrawl.org
	VTBaseURL      string // Optional, defaults to empty (not implemented)
}

// NewWayback creates a new Wayback enumerator with default configuration.
func NewWayback(wc webclient.WebClient, logger logging.Logger) *Wayback {
	return NewWaybackWithConfig(wc, logger, nil)
}

// NewWaybackWithConfig creates a new Wayback enumerator with custom configuration.
func NewWaybackWithConfig(wc webclient.WebClient, logger logging.Logger, cfg *WaybackConfig) *Wayback {
	wb := &Wayback{
		wc:     wc,
		logger: logger,
	}

	if cfg == nil {
		cfg = &WaybackConfig{}
	}

	if cfg.WaybackBaseURL != "" {
		wb.waybackBaseURL = cfg.WaybackBaseURL
	} else {
		wb.waybackBaseURL = "http://web.archive.org"
	}

	if cfg.CCBaseURL != "" {
		wb.ccBaseURL = cfg.CCBaseURL
	} else {
		wb.ccBaseURL = "http://index.commoncrawl.org"
	}

	wb.vtBaseURL = cfg.VTBaseURL

	return wb
}

// Enumerate fetches historical URLs from archive sources concurrently.
func (w *Wayback) Enumerate(ctx context.Context, target string, cb utils.ProgressCallback) ([]string, error) {
	root, err := utils.NewURLTools(target)
	if err != nil {
		return nil, err
	}

	// Extract domain for API queries
	domain := root.URL.Host

	// Channel to collect URLs from all sources
	urlChan := make(chan string, 100)
	var wg sync.WaitGroup

	// Fetch functions to run concurrently
	fetchFns := []struct {
		name string
		fn   func(context.Context, string) ([]string, error)
	}{
		{"wayback", w.fetchWaybackURLs},
		{"commoncrawl", w.fetchCommonCrawlURLs},
		{"virustotal", w.fetchVirusTotalURLs},
	}

	// Launch goroutines for each source
	for _, fetcher := range fetchFns {
		wg.Add(1)
		go func(name string, fn func(context.Context, string) ([]string, error)) {
			defer wg.Done()
			urls, err := fn(ctx, domain)
			if err != nil {
				w.logWarn("source failed", name, err)
				return
			}
			for _, u := range urls {
				select {
				case urlChan <- u:
				case <-ctx.Done():
					return
				}
			}
		}(fetcher.name, fetcher.fn)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(urlChan)
	}()

	// Deduplicate and filter URLs
	seen := make(map[string]struct{})
	var results []string

	for rawURL := range urlChan {
		// Parse and validate URL
		parsedURL, err := utils.NewURLTools(rawURL)
		if err != nil {
			continue
		}

		// Filter to same domain only (no subdomains)
		if !root.DomainIsSame(parsedURL) {
			continue
		}

		normalized := parsedURL.URL.String()
		if _, exists := seen[normalized]; !exists {
			seen[normalized] = struct{}{}
			results = append(results, normalized)
		}
	}

	// Progress callback at the end
	if cb != nil {
		cb(1, 1)
	}

	return results, nil
}

// fetchWaybackURLs queries the Wayback Machine CDX API.
func (w *Wayback) fetchWaybackURLs(ctx context.Context, domain string) ([]string, error) {
	// Use exact domain (no wildcard) to avoid subdomains
	url := fmt.Sprintf("%s/cdx/search/cdx?url=%s/*&output=json&collapse=urlkey", w.waybackBaseURL, domain)

	resp, err := w.wc.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("wayback request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wayback returned status %d", resp.StatusCode)
	}

	var wrapper waybackResponse
	if err := json.Unmarshal(resp.Body, &wrapper); err != nil {
		return nil, fmt.Errorf("wayback json parse failed: %w", err)
	}

	// First row is headers, skip it
	var urls []string
	for i, row := range wrapper {
		if i == 0 {
			continue // Skip header row
		}
		if len(row) > 2 {
			// URL is typically in index 2
			urls = append(urls, row[2])
		}
	}

	return urls, nil
}

// fetchCommonCrawlURLs queries the Common Crawl index API.
func (w *Wayback) fetchCommonCrawlURLs(ctx context.Context, domain string) ([]string, error) {
	// Use the latest index (CC-MAIN-2024-10 as example, in production this should be dynamic)
	url := fmt.Sprintf("%s/CC-MAIN-2024-10-index?url=%s/*&output=json", w.ccBaseURL, domain)

	resp, err := w.wc.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("commoncrawl request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		// Common Crawl may not have data for all domains
		if resp.StatusCode == 404 {
			return []string{}, nil
		}
		return nil, fmt.Errorf("commoncrawl returned status %d", resp.StatusCode)
	}

	// Common Crawl returns newline-delimited JSON
	var urls []string
	decoder := json.NewDecoder(strings.NewReader(string(resp.Body)))
	for {
		var item commonCrawlItem
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			// Partial results are acceptable
			break
		}
		if item.URL != "" {
			urls = append(urls, item.URL)
		}
	}

	return urls, nil
}

// fetchVirusTotalURLs queries the VirusTotal API.
// Note: This may require an API key and has rate limits.
func (w *Wayback) fetchVirusTotalURLs(ctx context.Context, domain string) ([]string, error) {
	// VirusTotal requires authentication and has rate limits.
	// For now, return empty to avoid errors. Users can extend this
	// by adding API key configuration.
	w.logInfo("virustotal source skipped (requires API key)")
	return []string{}, nil
}

func (w *Wayback) logWarn(msg, source string, err error) {
	if w.logger != nil {
		w.logger.Warn(msg,
			logging.Field{Key: "source", Value: source},
			logging.Field{Key: "error", Value: err})
	}
}

func (w *Wayback) logInfo(msg string) {
	if w.logger != nil {
		w.logger.Info(msg)
	}
}
