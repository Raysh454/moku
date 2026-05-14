package enumerator

import (
	"context"
	"encoding/xml"
	"strings"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

var sitemapPaths = []string{"/sitemap.xml", "/sitemap_index.xml"}

type xmlURLSet struct {
	XMLName xml.Name      `xml:"urlset"`
	URLs    []xmlURLEntry `xml:"url"`
}

type xmlURLEntry struct {
	Loc string `xml:"loc"`
}

type xmlSitemapIndex struct {
	XMLName  xml.Name          `xml:"sitemapindex"`
	Sitemaps []xmlSitemapEntry `xml:"sitemap"`
}

type xmlSitemapEntry struct {
	Loc string `xml:"loc"`
}

type Sitemap struct {
	wc     webclient.WebClient
	logger logging.Logger
}

func NewSitemap(wc webclient.WebClient, logger logging.Logger) *Sitemap {
	return &Sitemap{wc: wc, logger: logger}
}

func (s *Sitemap) Enumerate(ctx context.Context, target string, cb utils.ProgressCallback) ([]string, error) {
	root, err := utils.NewURLTools(target)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var results []string
	totalProcessed := 0

	for _, path := range sitemapPaths {
		urls, err := s.fetchSitemap(ctx, target+path, root)
		if err != nil {
			s.logWarn("failed to fetch sitemap", path, err)
			continue
		}

		for _, u := range urls {
			if _, exists := seen[u]; !exists {
				seen[u] = struct{}{}
				results = append(results, u)
			}
		}

		totalProcessed++
		if cb != nil {
			cb(totalProcessed, 0, len(sitemapPaths))
		}
	}

	return results, nil
}

func (s *Sitemap) fetchSitemap(ctx context.Context, url string, root *utils.URLTools) ([]string, error) {
	resp, err := s.wc.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, nil
	}

	body := resp.Body

	// Try parsing as sitemap index first.
	var index xmlSitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		return s.followIndex(ctx, index, root)
	}

	// Otherwise parse as urlset.
	return s.parseURLSet(body, root)
}

func (s *Sitemap) followIndex(ctx context.Context, index xmlSitemapIndex, root *utils.URLTools) ([]string, error) {
	var results []string
	for _, entry := range index.Sitemaps {
		loc := strings.TrimSpace(entry.Loc)
		if loc == "" {
			continue
		}
		urls, err := s.fetchSitemap(ctx, loc, root)
		if err != nil {
			s.logWarn("failed to fetch sub-sitemap", loc, err)
			continue
		}
		results = append(results, urls...)
	}
	return results, nil
}

func (s *Sitemap) parseURLSet(body []byte, root *utils.URLTools) ([]string, error) {
	var urlSet xmlURLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, nil
	}

	var results []string
	for _, entry := range urlSet.URLs {
		loc := strings.TrimSpace(entry.Loc)
		if loc == "" {
			continue
		}
		if !s.isSameDomain(loc, root) {
			continue
		}
		results = append(results, loc)
	}
	return results, nil
}

func (s *Sitemap) isSameDomain(rawURL string, root *utils.URLTools) bool {
	target, err := utils.NewURLTools(rawURL)
	if err != nil {
		return false
	}
	return root.DomainIsSame(target)
}

func (s *Sitemap) logWarn(msg, detail string, err error) {
	if s.logger != nil {
		s.logger.Warn(msg,
			logging.Field{Key: "detail", Value: detail},
			logging.Field{Key: "error", Value: err})
	}
}
