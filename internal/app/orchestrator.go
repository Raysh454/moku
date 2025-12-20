package app

import (
	"context"

	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/indexer"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
)

type Orchestrator struct {
    cfg      *Config
    registry *registry.Registry
    logger   logging.Logger
	siteComponentsCache map[string]*SiteComponents
}

// NewOrchestrator ties together config, registry and logger.
func NewOrchestrator(cfg *Config, reg *registry.Registry, logger logging.Logger) *Orchestrator {
    if cfg == nil {
        cfg = DefaultConfig()
    }
    return &Orchestrator{
        cfg:      cfg,
        registry: reg,
        logger:   logger,
    }
}

func (o *Orchestrator) siteComponentsFor(ctx context.Context, web *registry.Website) (*SiteComponents, error) {
	if o.siteComponentsCache == nil {
		o.siteComponentsCache = make(map[string]*SiteComponents)
	}
	if comps, ok := o.siteComponentsCache[web.ID]; ok {
		return comps, nil
	}
	comps, err := NewSiteComponents(ctx, o.cfg, *web, o.logger)
	if err != nil {
		return nil, err
	}
	o.siteComponentsCache[web.ID] = comps
	return comps, nil
}

func (o *Orchestrator) CreateProject(ctx context.Context, slug, name, description string) (*registry.Project, error) {
    return o.registry.CreateProject(ctx, slug, name, description)
}

func (o *Orchestrator) ListProjects(ctx context.Context) ([]registry.Project, error) {
    return o.registry.ListProjects(ctx)
}

func (o *Orchestrator) CreateWebsite(ctx context.Context, projectIdentifier, slug, origin string) (*registry.Website, error) {
    return o.registry.CreateWebsite(ctx, projectIdentifier, slug, origin)
}

func (o *Orchestrator) ListWebsites(ctx context.Context, projectIdentifier string) ([]registry.Website, error) {
    return o.registry.ListWebsites(ctx, projectIdentifier)
}

func (o *Orchestrator) AddWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug string, rawURLs []string, source string) ([]string, error) {
    web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
    if err != nil {
        return nil, err
    }
    comps, err := o.siteComponentsFor(ctx, web)
    if err != nil {
        return nil, err
    }
    return comps.Index.AddEndpoints(ctx, rawURLs, source)
}

func (o *Orchestrator) EnumerateWebsite(ctx context.Context, projectIdentifier, websiteSlug string, concurrency int) ([]string, error) {
    web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
    if err != nil {
        return nil, err
    }
    comps, err := o.siteComponentsFor(ctx, web)
    if err != nil {
        return nil, err
    }

	// Currently, we only have a spider enumerator.
	// When multiple enumerators are added, we can make this configurable.
    spider := enumerator.NewSpider(concurrency, comps.WebClient, o.logger)
    targets, err := spider.Enumerate(ctx, web.Origin)
    if err != nil {
        return nil, err
    }

    _, err = comps.Index.AddEndpoints(ctx, targets, "enumerator")
    if err != nil {
        return nil, err
    }
    return targets, nil
}

func (o *Orchestrator) ListWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int) ([]indexer.Endpoint, error) {
    web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
    if err != nil {
        return nil, err
    }
    comps, err := o.siteComponentsFor(ctx, web)
    if err != nil {
        return nil, err
    }
    return comps.Index.ListEndpoints(ctx, status, limit)
}

func (o *Orchestrator) FetchWebsiteEndpoints(ctx context.Context, projectIdentifier, websiteSlug, status string, limit int) error {
    if status == "" {
        status = "*"
    }
    web, err := o.registry.GetWebsiteBySlug(ctx, projectIdentifier, websiteSlug)
    if err != nil {
        return err
    }
    comps, err := o.siteComponentsFor(ctx, web)
    if err != nil {
        return err
    }
    return comps.Fetcher.FetchFromIndex(ctx, status, limit)
}
