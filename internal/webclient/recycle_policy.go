package webclient

import "time"

// Default recycle limits cap a long-lived browser before Chromium's well-known
// unbounded memory growth degrades it. Chosen to be conservative for a
// continuous-monitoring loop.
const (
	defaultBrowserMaxUses = 200
	defaultBrowserMaxAge  = 30 * time.Minute
)

// RecyclePolicy decides when a long-lived browser should be torn down and
// recreated, to bound memory growth and reap any accumulated state. A limit of
// zero means "unlimited" on that axis.
type RecyclePolicy struct {
	// MaxUses recycles the browser after this many fetches (<=0 = unlimited).
	MaxUses int
	// MaxAge recycles the browser after this much wall-clock time (<=0 = unlimited).
	MaxAge time.Duration
}

// ShouldRecycle reports whether a browser that has served uses fetches and is
// age old has exceeded either limit.
func (p RecyclePolicy) ShouldRecycle(uses int, age time.Duration) bool {
	if p.MaxUses > 0 && uses >= p.MaxUses {
		return true
	}
	if p.MaxAge > 0 && age >= p.MaxAge {
		return true
	}
	return false
}

// DefaultRecyclePolicy returns the conservative default limits.
func DefaultRecyclePolicy() RecyclePolicy {
	return RecyclePolicy{MaxUses: defaultBrowserMaxUses, MaxAge: defaultBrowserMaxAge}
}
