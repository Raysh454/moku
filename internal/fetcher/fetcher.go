package fetcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/raysh454/moku/internal/filter"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/tracker/models"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

// Module: fetcher
// Fetches, Normalizes and stores pages
type Fetcher struct {
	config  Config
	tracker tracker.Tracker
	wc      webclient.WebClient
	indexer indexer.EndpointIndex
	logger  logging.Logger
}

// filteredURL holds a URL that was filtered along with its reason
type filteredURL struct {
	URL    string
	Reason string
}

// New creates a new Fetcher with the given webclient, logger and tracker
func New(cfg Config, tracker tracker.Tracker, wc webclient.WebClient, indexer indexer.EndpointIndex, logger logging.Logger) (*Fetcher, error) {
	return &Fetcher{
		config:  cfg,
		tracker: tracker,
		wc:      wc,
		indexer: indexer,
		logger:  logger,
	}, nil
}

// FetchFromIndex fetches endpoints from the index by status, updates their status,
// commits snapshots, and returns whatever you need (e.g., created version IDs).
func (f *Fetcher) FetchFromIndex(ctx context.Context, status string, limit int, cb utils.ProgressCallback) error {
	return f.FetchFromIndexWithOptions(ctx, status, limit, nil, cb)
}

// FetchFromIndexWithOptions fetches endpoints from the index with optional filtering.
// If opts is nil or opts.FilterConfig is nil, no filtering is applied.
func (f *Fetcher) FetchFromIndexWithOptions(ctx context.Context, status string, limit int, opts *FetchOptions, cb utils.ProgressCallback) error {
	if f.indexer == nil {
		return fmt.Errorf("fetcher: index is nil")
	}

	f.logger.Info("fetcher: listing endpoints from index",
		logging.Field{Key: "status", Value: status},
		logging.Field{Key: "limit", Value: limit},
		logging.Field{Key: "filtering", Value: opts != nil && opts.FilterConfig != nil})

	var eps []indexer.Endpoint
	var err error

	// Use filtered listing if filter config provided
	if opts != nil && opts.FilterConfig != nil {
		eps, err = f.indexer.ListEndpointsFiltered(ctx, status, limit, opts.FilterConfig)
	} else {
		eps, err = f.indexer.ListEndpoints(ctx, status, limit)
	}
	if err != nil {
		return err
	}

	if status == "*" || strings.EqualFold(status, "all") {
		unfiltered := make([]indexer.Endpoint, 0, len(eps))
		for _, endpoint := range eps {
			if endpoint.Status == "filtered" {
				continue
			}
			unfiltered = append(unfiltered, endpoint)
		}
		eps = unfiltered
	}

	urls := make([]string, 0, len(eps))
	for _, e := range eps {
		urls = append(urls, e.CanonicalURL)
	}

	err = f.indexer.MarkPendingBatch(ctx, urls)
	if err != nil {
		return fmt.Errorf("error marking endpoints as pending: %w", err)
	}

	f.logger.Info("starting fetch for endpoints", logging.Field{Key: "count", Value: len(urls)})

	// Pass filter options for post-fetch status code filtering
	f.FetchWithOptions(ctx, urls, opts, cb)

	return nil
}

// Fetch retrieves all given HTTP URLs and stores snapshots to the tracker.
//
// Implementation uses a worker pool pattern:
//   - Spawns exactly MaxConcurrency worker goroutines (fixed pool size)
//   - URLs are fed through a buffered channel (urlQueue) to workers
//   - Workers fetch pages and send snapshots to a separate batcher goroutine
//   - The batcher goroutine collects snapshots and commits them in batches
//
// This approach ensures a fixed number of goroutines regardless of URL count,
// minimizing memory overhead and scheduler pressure compared to spawning
// one goroutine per URL.
//
// Flow:
//  1. Start MaxConcurrency workers listening on urlQueue
//  2. Start batcher goroutine listening on snapCh
//  3. Feed URLs to urlQueue (workers process concurrently)
//  4. Close urlQueue when all URLs are sent
//  5. Workers exit when urlQueue is drained, close snapCh
//  6. Batcher finalizes commit and exits
//
// Context cancellation is respected at multiple points for graceful shutdown.
func (f *Fetcher) Fetch(ctx context.Context, pageUrls []string, cb utils.ProgressCallback) {
	f.FetchWithOptions(ctx, pageUrls, nil, cb)
}

// FetchWithOptions retrieves all given HTTP URLs with optional filtering and stores snapshots.
// If opts.FilterStatusCodes is true, responses matching SkipStatusCodes are filtered out.
func (f *Fetcher) FetchWithOptions(ctx context.Context, pageUrls []string, opts *FetchOptions, cb utils.ProgressCallback) {
	var wg sync.WaitGroup
	// urlQueue is buffered to avoid blocking the main goroutine when feeding URLs.
	// Buffer size is 2x MaxConcurrency to allow some queueing without contention.
	urlQueue := make(chan string, f.config.MaxConcurrency*2)
	snapCh := make(chan *models.Snapshot)
	batcherDone := make(chan struct{})
	filterCollectorDone := make(chan struct{})

	// Channel for URLs filtered by status code (for marking in indexer)
	filteredCh := make(chan filteredURL, f.config.MaxConcurrency)

	// Collected filtered URLs (populated by collector goroutine)
	var filteredURLs []string
	var filteredReasons map[string]string

	// Create filter engine if status code filtering is enabled
	var filterEngine *filter.Engine
	if opts != nil && opts.FilterStatusCodes && opts.FilterConfig != nil {
		filterEngine = filter.NewEngine(opts.FilterConfig)

		// Start a goroutine to collect filtered URLs to avoid blocking workers
		filteredURLs = make([]string, 0)
		filteredReasons = make(map[string]string)
		go func() {
			defer close(filterCollectorDone)
			for fu := range filteredCh {
				filteredURLs = append(filteredURLs, fu.URL)
				filteredReasons[fu.URL] = fu.Reason
			}
		}()
	} else {
		// No filter engine, just close the collector done channel
		close(filterCollectorDone)
	}

	total := len(pageUrls)
	var processed int32

	emitProgress := func() {
		if cb != nil {
			p := atomic.AddInt32(&processed, 1)
			cb(int(p), total)
		}
	}

	// Commit snapshots goroutine using transaction-like API
	go func() {
		defer close(batcherDone)

		// Begin a pending commit for the entire fetch operation
		pc, err := f.tracker.BeginCommit(ctx, fmt.Sprintf("Fetch %d pages", total), "fetcher")
		if err != nil {
			if f.logger != nil {
				f.logger.Error("error while beginning commit",
					logging.Field{Key: "error", Value: err})
			}
			return
		}

		// Ensure cleanup on error
		defer func() {
			if pc.GetTransaction() != nil {
				// Transaction still active means we didn't finalize successfully
				if err := f.tracker.CancelCommit(ctx, pc); err != nil && f.logger != nil {
					f.logger.Warn("error while cancelling commit", logging.Field{Key: "error", Value: err})
				}
			}
		}()

		batch := make([]*models.Snapshot, 0, f.config.CommitSize)
		allUrls := make([]string, 0, total) // Track all URLs for final indexer update

		addBatch := func() error {
			if len(batch) == 0 {
				return nil
			}

			// Add snapshots to pending commit (doesn't create new version)
			if err := f.tracker.AddSnapshots(ctx, pc, batch); err != nil {
				if f.logger != nil {
					f.logger.Error("error while adding snapshots to pending commit",
						logging.Field{Key: "error", Value: err})
				}
				return err
			}

			// Track URLs for later indexer update
			for _, snap := range batch {
				allUrls = append(allUrls, snap.URL)
			}

			batch = batch[:0]
			return nil
		}

		// Process snapshots from channel
		for {
			select {
			case <-ctx.Done():
				if err := addBatch(); err != nil && f.logger != nil {
					f.logger.Warn("error while adding final batch on cancellation",
						logging.Field{Key: "error", Value: err})
				}
				return
			case snap, ok := <-snapCh:
				if !ok {
					// Channel closed - finalize the commit
					if err := addBatch(); err != nil {
						return
					}

					// Finalize the pending commit (creates one version for all snapshots)
					cr, err := f.tracker.FinalizeCommit(ctx, pc)
					if err != nil {
						if f.logger != nil {
							f.logger.Error("error while finalizing commit",
								logging.Field{Key: "error", Value: err})
						}
						return
					}

					// Score the entire version
					err = f.tracker.ScoreAndAttributeVersion(ctx, cr, f.config.ScoreTimeout)
					if err != nil && f.logger != nil {
						f.logger.Error("error while scoring version",
							logging.Field{Key: "error", Value: err})
					}

					// Mark all URLs as fetched in indexer (single version for all)
					if f.indexer != nil && len(allUrls) > 0 {
						err = f.indexer.MarkFetchedBatch(ctx, allUrls, cr.Version.ID, time.Now())
						if err != nil && f.logger != nil {
							f.logger.Error("error while marking endpoints as fetched",
								logging.Field{Key: "error", Value: err})
						}
					}

					if f.logger != nil {
						f.logger.Info("Fetch complete - created single version",
							logging.Field{Key: "version_id", Value: cr.Version.ID},
							logging.Field{Key: "snapshots", Value: len(cr.Snapshots)})
					}

					return
				}

				batch = append(batch, snap)
				if len(batch) == f.config.CommitSize {
					if err := addBatch(); err != nil {
						return
					}
				}
			}
		}
	}()

	// Worker pool: spawn exactly MaxConcurrency worker goroutines.
	// Each worker processes URLs from urlQueue until the channel is closed.
	for i := 0; i < f.config.MaxConcurrency; i++ {
		wg.Go(func() {
			// Process URLs until urlQueue is closed
			for pageUrl := range urlQueue {
				// Check context before processing
				if ctx.Err() != nil {
					return
				}

				emitProgress()

				response, err := f.HTTPGet(ctx, pageUrl)
				if err != nil {
					if f.logger != nil {
						f.logger.Error("error while fetching page",
							logging.Field{Key: "url", Value: pageUrl},
							logging.Field{Key: "error", Value: err})
					}
					if f.indexer != nil {
						err := f.indexer.MarkFailed(ctx, pageUrl, err.Error())
						if err != nil && f.logger != nil {
							f.logger.Error("error while marking endpoint as failed in indexer",
								logging.Field{Key: "url", Value: pageUrl},
								logging.Field{Key: "error", Value: err})
						}
					}
					continue
				}

				// Check if response status code should be filtered
				if filterEngine != nil {
					result := filterEngine.ShouldFilterStatus(response.StatusCode)
					if result.Filtered {
						if f.logger != nil {
							f.logger.Debug("filtering response by status code",
								logging.Field{Key: "url", Value: pageUrl},
								logging.Field{Key: "status", Value: response.StatusCode},
								logging.Field{Key: "reason", Value: result.Reason})
						}
						select {
						case <-ctx.Done():
							return
						case filteredCh <- filteredURL{URL: pageUrl, Reason: result.Reason}:
						}
						continue
					}
				}

				snap := utils.NewSnapshotFromResponse(response)
				select {
				case <-ctx.Done():
					return
				case snapCh <- snap:
				}
			}
		})
	}

	// Feed URLs to the worker pool via urlQueue.
	// Workers will process URLs concurrently as they become available.
	for _, pageUrl := range pageUrls {
		select {
		case <-ctx.Done():
			// Context cancelled, stop feeding URLs
			goto cleanup
		case urlQueue <- pageUrl:
			// URL sent successfully
		}
	}

cleanup:
	// Close urlQueue to signal workers that no more URLs will be sent.
	// Workers will exit when they finish processing remaining URLs.
	close(urlQueue)
	// Wait for all workers to complete
	wg.Wait()
	// Close snapCh to signal batcher that no more snapshots will be sent
	close(snapCh)
	// Close filtered channel to signal collector goroutine
	close(filteredCh)

	// Wait for filter collector goroutine to finish
	<-filterCollectorDone

	// Mark filtered URLs in indexer
	if f.indexer != nil && len(filteredURLs) > 0 {
		if err := f.indexer.MarkFilteredBatch(ctx, filteredURLs, filteredReasons); err != nil && f.logger != nil {
			f.logger.Error("error marking filtered URLs in indexer",
				logging.Field{Key: "error", Value: err})
		}
	}

	// Wait for batcher to finish committing all snapshots
	<-batcherDone
}

// Makes an HTTP GET Request to the given parameter and returns reference Page struct
func (f *Fetcher) HTTPGet(ctx context.Context, page string) (*webclient.Response, error) {
	if f.wc == nil {
		return nil, fmt.Errorf("fetcher: webclient is nil")
	}

	resp, err := f.wc.Get(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("error GETting %s: %w", page, err)
	}

	return resp, nil
}
