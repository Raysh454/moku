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

	// Commit snapshots goroutine
	go func() {
		defer close(batcherDone)
		batch := make([]*models.Snapshot, 0, f.config.CommitSize)
		flush := func() {
			if len(batch) > 0 {
				cr, err := f.tracker.CommitBatch(ctx, batch, "some kind of message", "^_^")
				if err != nil {
					if f.logger != nil {
						f.logger.Error("error while committing snapshot batch",
							logging.Field{Key: "error", Value: err})
					}
					batch = batch[:0]
					return
				}
				err = f.tracker.ScoreAndAttributeVersion(ctx, cr, f.config.ScoreTimeout)
				if err != nil {
					if f.logger != nil {
						f.logger.Error("error while scoring and attributing version to committed snapshots",
							logging.Field{Key: "error", Value: err})
					}
					batch = batch[:0]
					return
				}
				urls := make([]string, 0, len(batch))
				for _, snap := range batch {
					urls = append(urls, snap.URL)
				}
				if f.indexer != nil {
					err = f.indexer.MarkFetchedBatch(ctx, urls, cr.Version.ID, time.Now())
					if err != nil {
						if f.logger != nil {
							f.logger.Error("error while marking endpoints as fetched in indexer",
								logging.Field{Key: "error", Value: err})
						}
					}
				}
				batch = batch[:0]
			}
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case snap, ok := <-snapCh:
				if !ok {
					flush()
					return
				}
				batch = append(batch, snap)
				if len(batch) == f.config.CommitSize {
					flush()
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
