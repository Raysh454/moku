package app

import (
	"context"
	"sync"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
)

// siteComponentsFactory builds the per-site component bundle. Injected so the
// catalog can be exercised without spinning up real trackers and web clients.
type siteComponentsFactory func(ctx context.Context, web registry.Website) (*SiteComponents, error)

// siteComponentsCatalog lazily builds and caches one SiteComponents bundle
// per website. A per-site init lock guarantees a site is initialized by at
// most one goroutine at a time, while failed builds stay uncached so the
// next caller retries them.
type siteComponentsCatalog struct {
	build  siteComponentsFactory
	logger logging.Logger

	mu    sync.Mutex
	cache map[string]*SiteComponents

	initMu    sync.Mutex
	initLocks map[string]*sync.Mutex
}

func newSiteComponentsCatalog(build siteComponentsFactory, logger logging.Logger) *siteComponentsCatalog {
	return &siteComponentsCatalog{
		build:     build,
		logger:    logger,
		cache:     make(map[string]*SiteComponents),
		initLocks: make(map[string]*sync.Mutex),
	}
}

// componentsFor returns the cached components for web, building them on
// first use. Deliberately a per-site mutex rather than sync.Once: a failed
// build must remain retryable on the next call, and sync.Once would latch
// the failure.
func (c *siteComponentsCatalog) componentsFor(ctx context.Context, web *registry.Website) (*SiteComponents, error) {
	c.mu.Lock()
	if comps, ok := c.cache[web.ID]; ok {
		c.mu.Unlock()
		return comps, nil
	}
	c.mu.Unlock()

	// Use a per-site lock to prevent concurrent initialization of the same database.
	// This prevents "database is locked" errors when multiple requests hit a new site.
	c.initMu.Lock()
	lock, ok := c.initLocks[web.ID]
	if !ok {
		lock = &sync.Mutex{}
		c.initLocks[web.ID] = lock
	}
	c.initMu.Unlock()

	lock.Lock()
	defer lock.Unlock()

	// Check cache again after acquiring site-specific lock
	c.mu.Lock()
	if comps, ok := c.cache[web.ID]; ok {
		c.mu.Unlock()
		return comps, nil
	}
	c.mu.Unlock()

	comps, err := c.build(ctx, *web)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[web.ID] = comps
	c.mu.Unlock()

	// Initialization complete — remove the per-site init lock to avoid unbounded
	// growth of `initLocks`. We delete the entry while still holding the
	// site-specific lock to ensure waiting goroutines (which hold a pointer to
	// the same mutex) will still be correctly synchronized and will re-check
	// the cache when they proceed.
	c.initMu.Lock()
	delete(c.initLocks, web.ID)
	c.initMu.Unlock()

	return comps, nil
}

// get returns the cached components for a website ID, or nil when absent.
func (c *siteComponentsCatalog) get(websiteID string) *SiteComponents {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache[websiteID]
}

// put caches a pre-built component bundle for a website ID.
func (c *siteComponentsCatalog) put(websiteID string, comps *SiteComponents) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[websiteID] = comps
}

// evict removes and returns the cached components for a website ID, or nil
// when nothing was cached. The caller owns closing the returned bundle.
func (c *siteComponentsCatalog) evict(websiteID string) *SiteComponents {
	c.mu.Lock()
	defer c.mu.Unlock()
	comps := c.cache[websiteID]
	if comps != nil {
		delete(c.cache, websiteID)
	}
	return comps
}

// closeAll closes every cached component bundle and empties the cache.
func (c *siteComponentsCatalog) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, sc := range c.cache {
		if sc != nil {
			c.logger.Info("closing site components", logging.Field{Key: "website_id", Value: id})
			if err := sc.Close(); err != nil && c.logger != nil {
				c.logger.Warn("failed to close site components",
					logging.Field{Key: "website_id", Value: id},
					logging.Field{Key: "error", Value: err.Error()},
				)
			}
		}
		delete(c.cache, id)
	}
}
