package app

import (
	"context"
	"fmt"

	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

type SiteComponents struct {
	Tracker tracker.Tracker
	Index   indexer.EndpointIndex
	Fetcher *fetcher.Fetcher
	WebClient webclient.WebClient
}

// Build components for a given website (registry.Website).
func NewSiteComponents(ctx context.Context, cfg *Config, web registry.Website, logger logging.Logger) (*SiteComponents, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	a, err := assessor.NewHeuristicsAssessor(&cfg.assessorCfg, nil, logger)
	if err != nil {
		return nil, fmt.Errorf("new assessor: %w", err)
	}

	cfg.trackerCfg.StoragePath = web.StoragePath
	cfg.trackerCfg.ProjectID = web.ProjectID
	tr, err := tracker.NewSQLiteTracker(&cfg.trackerCfg, logger, a)
	if err != nil {
		return nil, fmt.Errorf("new tracker: %w", err)
	}

	db := tr.DB()
	ix := indexer.NewIndex(db, logger, utils.CanonicalizeOptions{})

	wc, err := webclient.NewWebClient(cfg.WebClientCfg, logger)
	if err != nil {
		tr.Close()
		return nil, fmt.Errorf("new webclient: %w", err)
	}

	f, err := fetcher.New(
		cfg.FetcherCfg,
		tr,
		wc,
		ix,
		logger,
	)
	if err != nil {
		tr.Close()
		_ = wc.Close()
		return nil, fmt.Errorf("new fetcher: %w", err)
	}

	return &SiteComponents{
		Tracker: tr,
		Index:   ix,
		Fetcher: f,
		WebClient: wc,
	}, nil
}

// Close site components and release resources.
// Calling this will close resources that the Fetcher relies on
// Any ongoing fetch operations will be stopped.
func (sc *SiteComponents) Close() error {
	var firstErr error
	if err := sc.WebClient.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close webclient: %w", err)
	}
	if err := sc.Tracker.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close tracker: %w", err)
	}
	return firstErr
}
