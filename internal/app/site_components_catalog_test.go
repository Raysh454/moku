package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/raysh454/moku/internal/registry"
	"github.com/raysh454/moku/internal/testutil"
)

func newTestCatalog(build siteComponentsFactory) *siteComponentsCatalog {
	return newSiteComponentsCatalog(build, &testutil.DummyLogger{})
}

// countingFactory returns a factory that records how many bundles it built.
func countingFactory(builds *atomic.Int32) siteComponentsFactory {
	return func(_ context.Context, _ registry.Website) (*SiteComponents, error) {
		builds.Add(1)
		return &SiteComponents{}, nil
	}
}

func TestSiteComponentsCatalog_BuildsComponentsOnFirstUse(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	c := newTestCatalog(countingFactory(&builds))
	web := &registry.Website{ID: "site-1"}

	comps, err := c.componentsFor(context.Background(), web)

	if err != nil {
		t.Fatalf("componentsFor: %v", err)
	}
	if comps == nil {
		t.Fatal("expected non-nil components")
	}
	if got := builds.Load(); got != 1 {
		t.Errorf("factory invocations = %d, want 1", got)
	}
}

func TestSiteComponentsCatalog_ReturnsCachedComponentsWithoutRebuilding(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	c := newTestCatalog(countingFactory(&builds))
	web := &registry.Website{ID: "site-1"}

	first, err := c.componentsFor(context.Background(), web)
	if err != nil {
		t.Fatalf("first componentsFor: %v", err)
	}
	second, err := c.componentsFor(context.Background(), web)
	if err != nil {
		t.Fatalf("second componentsFor: %v", err)
	}

	if first != second {
		t.Error("expected the cached bundle to be reused for the same site")
	}
	if got := builds.Load(); got != 1 {
		t.Errorf("factory invocations = %d, want 1", got)
	}
}

func TestSiteComponentsCatalog_BuildsEachSiteIndependently(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	c := newTestCatalog(countingFactory(&builds))

	compsA, err := c.componentsFor(context.Background(), &registry.Website{ID: "site-a"})
	if err != nil {
		t.Fatalf("componentsFor(site-a): %v", err)
	}
	compsB, err := c.componentsFor(context.Background(), &registry.Website{ID: "site-b"})
	if err != nil {
		t.Fatalf("componentsFor(site-b): %v", err)
	}

	if compsA == compsB {
		t.Error("expected distinct bundles per site")
	}
	if got := builds.Load(); got != 2 {
		t.Errorf("factory invocations = %d, want 2", got)
	}
}

func TestSiteComponentsCatalog_InitializesASiteExactlyOnceUnderConcurrentCallers(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	c := newTestCatalog(countingFactory(&builds))
	web := &registry.Website{ID: "site-1"}

	const callers = 16
	results := make([]*SiteComponents, callers)
	errs := make([]error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func(slot int) {
			defer wg.Done()
			<-start
			results[slot], errs[slot] = c.componentsFor(context.Background(), web)
		}(i)
	}
	close(start)
	wg.Wait()

	if got := builds.Load(); got != 1 {
		t.Errorf("factory invocations = %d, want exactly 1 under concurrent callers", got)
	}
	for i := 0; i < callers; i++ {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if results[i] != results[0] {
			t.Fatalf("caller %d received a different bundle than caller 0", i)
		}
	}
}

func TestSiteComponentsCatalog_RetriesTheBuildAfterAFailedInitialization(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	c := newTestCatalog(func(_ context.Context, _ registry.Website) (*SiteComponents, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("init failed")
		}
		return &SiteComponents{}, nil
	})
	web := &registry.Website{ID: "site-1"}

	if _, err := c.componentsFor(context.Background(), web); err == nil {
		t.Fatal("expected the first build to fail")
	}
	comps, err := c.componentsFor(context.Background(), web)

	if err != nil {
		t.Fatalf("expected the second call to retry and succeed, got %v", err)
	}
	if comps == nil {
		t.Fatal("expected non-nil components after the retry")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("factory invocations = %d, want 2 (failure must not be latched)", got)
	}
}

func TestSiteComponentsCatalog_ReleasesTheInitLockAfterASuccessfulBuild(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	c := newTestCatalog(countingFactory(&builds))
	web := &registry.Website{ID: "site-1"}

	if _, err := c.componentsFor(context.Background(), web); err != nil {
		t.Fatalf("componentsFor: %v", err)
	}

	c.initMu.Lock()
	leftover := len(c.initLocks)
	c.initMu.Unlock()
	if leftover != 0 {
		t.Errorf("init locks left after successful build = %d, want 0 (the map must not grow unboundedly)", leftover)
	}
}

func TestSiteComponentsCatalog_EvictRemovesAndReturnsTheCachedBundle(t *testing.T) {
	t.Parallel()
	var builds atomic.Int32
	c := newTestCatalog(countingFactory(&builds))
	web := &registry.Website{ID: "site-1"}
	comps, err := c.componentsFor(context.Background(), web)
	if err != nil {
		t.Fatalf("componentsFor: %v", err)
	}

	evicted := c.evict(web.ID)

	if evicted != comps {
		t.Error("expected evict to return the cached bundle")
	}
	if c.get(web.ID) != nil {
		t.Error("expected the bundle to be removed from the cache")
	}
}

func TestSiteComponentsCatalog_EvictReturnsNilForUnknownSite(t *testing.T) {
	t.Parallel()
	c := newTestCatalog(countingFactory(&atomic.Int32{}))

	if got := c.evict("unknown"); got != nil {
		t.Errorf("expected nil for unknown site, got %+v", got)
	}
}

func TestSiteComponentsCatalog_CloseAllClosesEveryCachedBundleAndEmptiesTheCache(t *testing.T) {
	t.Parallel()
	closedSites := map[string]bool{}
	var mu sync.Mutex
	c := newTestCatalog(func(_ context.Context, web registry.Website) (*SiteComponents, error) {
		id := web.ID
		return &SiteComponents{Tracker: &testutil.DummyTracker{CloseFunc: func() error {
			mu.Lock()
			closedSites[id] = true
			mu.Unlock()
			return nil
		}}}, nil
	})
	for _, id := range []string{"site-a", "site-b"} {
		if _, err := c.componentsFor(context.Background(), &registry.Website{ID: id}); err != nil {
			t.Fatalf("componentsFor(%s): %v", id, err)
		}
	}

	c.closeAll()

	mu.Lock()
	defer mu.Unlock()
	if !closedSites["site-a"] || !closedSites["site-b"] {
		t.Errorf("expected every cached bundle to be closed, got %v", closedSites)
	}
	if c.get("site-a") != nil || c.get("site-b") != nil {
		t.Error("expected the cache to be emptied")
	}
}
