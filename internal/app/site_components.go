package app

import (
	"context"
	"fmt"

	"github.com/raysh454/moku/internal/analyzer"
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
	Tracker   tracker.Tracker
	Index     indexer.EndpointIndex
	Fetcher   *fetcher.Fetcher
	WebClient webclient.WebClient
	Analyzer  analyzer.Analyzer
	logger    logging.Logger // Add scoped logger
}

// Build components for a given website (registry.Website).
func NewSiteComponents(ctx context.Context, cfg *Config, web registry.Website, logger logging.Logger) (*SiteComponents, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Scope the logger to this site
	scopedLogger := logger.With(
		logging.Field{Key: "project", Value: web.ProjectID},
		logging.Field{Key: "site", Value: web.Slug},
	)

	a, err := assessor.NewHeuristicsAssessor(&cfg.assessorCfg, scopedLogger)
	if err != nil {
		return nil, fmt.Errorf("new assessor: %w", err)
	}

	trackerCfg := cfg.trackerCfg
	trackerCfg.StoragePath = web.StoragePath
	trackerCfg.ProjectID = web.ProjectID
	tr, err := tracker.NewSQLiteTracker(&trackerCfg, scopedLogger, a)
	if err != nil {
		return nil, fmt.Errorf("new tracker: %w", err)
	}

	db := tr.DB()
	ix := indexer.NewIndex(db, scopedLogger, utils.CanonicalizeOptions{})

	wc, err := webclient.NewWebClient(cfg.WebClientCfg, scopedLogger)
	if err != nil {
		tr.Close()
		return nil, fmt.Errorf("new webclient: %w", err)
	}

	f, err := fetcher.New(
		cfg.FetcherCfg,
		tr,
		wc,
		ix,
		scopedLogger,
	)
	if err != nil {
		tr.Close()
		_ = wc.Close()
		return nil, fmt.Errorf("new fetcher: %w", err)
	}

	an, err := analyzer.NewAnalyzer(cfg.AnalyzerCfg, analyzer.Dependencies{
		Logger:     scopedLogger,
		WebClient:  wc,
		Assessor:   a,
		HTTPClient: wc,
	})
	if err != nil {
		tr.Close()
		_ = wc.Close()
		return nil, fmt.Errorf("new analyzer: %w", err)
	}

	return &SiteComponents{
		Tracker:   tr,
		Index:     ix,
		Fetcher:   f,
		WebClient: wc,
		Analyzer:  an,
		logger:    scopedLogger,
	}, nil
}

// Close site components and release resources.
func (sc *SiteComponents) Close() error {
	var firstErr error
	if sc.Analyzer != nil {
		if err := sc.Analyzer.Close(); err != nil {
			firstErr = fmt.Errorf("close analyzer: %w", err)
		}
	}
	if sc.WebClient != nil {
		if err := sc.WebClient.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close webclient: %w", err)
		}
	}
	if sc.Tracker != nil {
		if err := sc.Tracker.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close tracker: %w", err)
		}
	}
	return firstErr
}
