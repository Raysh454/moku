package fetcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
	if f.indexer == nil {
		return fmt.Errorf("fetcher: index is nil")
	}

	f.logger.Info("fetcher: listing endpoints from index", logging.Field{Key: "status", Value: status}, logging.Field{Key: "limit", Value: limit})
	eps, err := f.indexer.ListEndpoints(ctx, status, limit)
	if err != nil {
		return err
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
	f.Fetch(ctx, urls, cb)

	return nil
}

// Gets and stores all given HTTP urls to file system
func (f *Fetcher) Fetch(ctx context.Context, pageUrls []string, cb utils.ProgressCallback) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, f.config.MaxConcurrency)
	snapCh := make(chan *models.Snapshot)
	batcherDone := make(chan struct{})

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

	// Fetch pages concurrently
	for _, pageUrl := range pageUrls {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)

		// TODO: Change to worker pool pattern instead of spawning goroutine per URL
		go func(pageUrl string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			defer emitProgress()

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
				return
			}

			snap := utils.NewSnapshotFromResponse(response)
			select {
			case <-ctx.Done():
				return
			case snapCh <- snap:
			}
		}(pageUrl)
	}

	wg.Wait()
	close(snapCh)
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
