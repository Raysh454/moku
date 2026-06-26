package webclient_test

import (
	"testing"
	"time"

	"github.com/raysh454/moku/internal/webclient"
)

func TestRecyclePolicy_RecyclesAtMaxUses(t *testing.T) {
	t.Parallel()
	p := webclient.RecyclePolicy{MaxUses: 100}

	if p.ShouldRecycle(99, time.Minute) {
		t.Error("should not recycle below MaxUses")
	}
	if !p.ShouldRecycle(100, time.Minute) {
		t.Error("should recycle at MaxUses")
	}
	if !p.ShouldRecycle(150, time.Minute) {
		t.Error("should recycle past MaxUses")
	}
}

func TestRecyclePolicy_RecyclesAtMaxAge(t *testing.T) {
	t.Parallel()
	p := webclient.RecyclePolicy{MaxAge: 30 * time.Minute}

	if p.ShouldRecycle(1, 29*time.Minute) {
		t.Error("should not recycle below MaxAge")
	}
	if !p.ShouldRecycle(1, 30*time.Minute) {
		t.Error("should recycle at MaxAge")
	}
}

func TestRecyclePolicy_ZeroLimitsMeanUnlimited(t *testing.T) {
	t.Parallel()
	p := webclient.RecyclePolicy{} // both zero

	if p.ShouldRecycle(1_000_000, 1000*time.Hour) {
		t.Error("zero limits must mean never recycle")
	}
}

func TestRecyclePolicy_EitherLimitTriggers(t *testing.T) {
	t.Parallel()
	p := webclient.RecyclePolicy{MaxUses: 100, MaxAge: 30 * time.Minute}

	if !p.ShouldRecycle(100, time.Second) {
		t.Error("use limit alone should trigger")
	}
	if !p.ShouldRecycle(1, 30*time.Minute) {
		t.Error("age limit alone should trigger")
	}
	if p.ShouldRecycle(50, time.Minute) {
		t.Error("neither limit reached should not trigger")
	}
}

func TestDefaultRecyclePolicy_HasBoundedLimits(t *testing.T) {
	t.Parallel()
	p := webclient.DefaultRecyclePolicy()
	if p.MaxUses <= 0 || p.MaxAge <= 0 {
		t.Errorf("default policy must bound both uses and age, got %+v", p)
	}
}
