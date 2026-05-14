package enumerator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raysh454/moku/internal/enumerator"
	"github.com/raysh454/moku/internal/testutil"
)

func TestComposite_should_implement_Enumerator_interface(t *testing.T) {
	var _ enumerator.Enumerator = enumerator.NewComposite(nil, nil)
}

func TestComposite_should_merge_results_from_all_enumerators(t *testing.T) {
	e1 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/1", "http://a.com/2"}}
	e2 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/3"}}

	composite := enumerator.NewComposite([]enumerator.Enumerator{e1, e2}, nil)
	urls, err := composite.Enumerate(context.Background(), "http://a.com", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"http://a.com/1", "http://a.com/2", "http://a.com/3"}
	assertURLsEqual(t, urls, want)
}

func TestComposite_should_deduplicate_urls(t *testing.T) {
	e1 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/dup", "http://a.com/only1"}}
	e2 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/dup", "http://a.com/only2"}}

	composite := enumerator.NewComposite([]enumerator.Enumerator{e1, e2}, nil)
	urls, err := composite.Enumerate(context.Background(), "http://a.com", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"http://a.com/dup", "http://a.com/only1", "http://a.com/only2"}
	assertURLsEqual(t, urls, want)
}

func TestComposite_should_continue_when_one_enumerator_fails(t *testing.T) {
	e1 := &testutil.DummyEnumerator{Err: errors.New("broken")}
	e2 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/ok"}}

	composite := enumerator.NewComposite([]enumerator.Enumerator{e1, e2}, nil)
	urls, err := composite.Enumerate(context.Background(), "http://a.com", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"http://a.com/ok"}
	assertURLsEqual(t, urls, want)
}

func TestComposite_should_aggregate_progress(t *testing.T) {
	e1 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/1"}}
	e2 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/2"}}

	composite := enumerator.NewComposite([]enumerator.Enumerator{e1, e2}, nil)

	var lastProcessed, lastFailed, lastTotal int
	cb := func(processed, failed, total int) {
		lastProcessed = processed
		lastFailed = failed
		lastTotal = total
	}

	_, err := composite.Enumerate(context.Background(), "http://a.com", cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = lastFailed // Use it
	if lastProcessed == 0 {
		t.Error("expected progress callback to report processed > 0")
	}
	if lastTotal == 0 {
		t.Error("expected progress callback to report total > 0")
	}
}

func TestComposite_should_return_empty_for_no_enumerators(t *testing.T) {
	composite := enumerator.NewComposite(nil, nil)
	urls, err := composite.Enumerate(context.Background(), "http://a.com", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("expected empty slice, got %v", urls)
	}
}

func TestComposite_should_respect_context_cancellation(t *testing.T) {
	e1 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/1"}}
	e2 := &testutil.DummyEnumerator{URLs: []string{"http://a.com/2"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	composite := enumerator.NewComposite([]enumerator.Enumerator{e1, e2}, nil)
	urls, err := composite.Enumerate(ctx, "http://a.com", nil)

	// Should return early; may have partial or empty results
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With an already-cancelled context, neither enumerator should run
	if len(urls) != 0 {
		t.Errorf("expected empty results with cancelled context, got %v", urls)
	}
}
