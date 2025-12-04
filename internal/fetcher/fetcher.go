package fetcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/webclient"
)

// Module: fetcher
// Fetches, Normalizes and stores pages
type Fetcher struct {
	MaxConcurrency int
	CommitSize     int
	tracker        tracker.Tracker
	wc             webclient.WebClient
	logger         logging.Logger
}

// New creates a new Fetcher with the given webclient, logger and tracker
func New(MaxConcurrency, CommitSize int, tracker tracker.Tracker, wc webclient.WebClient, logger logging.Logger) (*Fetcher, error) {
	return &Fetcher{
		MaxConcurrency: MaxConcurrency,
		CommitSize:     CommitSize,
		tracker:        tracker,
		wc:             wc,
		logger:         logger,
	}, nil
}

// Gets and stores all given HTTP urls to file system
func (f *Fetcher) Fetch(ctx context.Context, pageUrls []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, f.MaxConcurrency)
	snapCh := make(chan *tracker.Snapshot)
	batcherDone := make(chan struct{})

	// Commit snapshots goroutine
	go func() {
		defer close(batcherDone)
		batch := make([]*tracker.Snapshot, 0, f.CommitSize)
		flush := func() {
			if len(batch) > 0 {
				if _, err := f.tracker.CommitBatch(ctx, batch, "some kind of message", "^_^"); err != nil {
					if f.logger != nil {
						f.logger.Error("error while committing snapshot batch",
							logging.Field{Key: "error", Value: err})
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
				if len(batch) == f.CommitSize {
					flush()
				}
			}
		}
	}()

	// Fetch pages concurrently
	for _, pageUrl := range pageUrls {
		if ctx.Err() != nil {
			return
		}

		wg.Add(1)

		go func(pageUrl string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			response, err := f.HTTPGet(ctx, pageUrl)
			if err != nil {
				if f.logger != nil {
					f.logger.Error("error while fetching page",
						logging.Field{Key: "url", Value: pageUrl},
						logging.Field{Key: "error", Value: err})
				}
				return
			}

			snap := tracker.NewSnapshotFromResponse(response)
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

	// TODO: Normalize response body somehow?
	return resp, nil
}
