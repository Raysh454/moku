package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/testutil"
)

func TestOrchestrator_Broker(t *testing.T) {
	o := NewOrchestrator(nil, nil, &testutil.DummyLogger{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub1 := o.Subscribe(ctx)
	sub2 := o.Subscribe(ctx)

	// Mock a job
	o.jobsMu.Lock()
	if o.jobs == nil {
		o.jobs = make(map[string]*Job)
	}
	o.jobs["job1"] = &Job{ID: "job1", Project: "p1", Website: "s1"}
	o.jobsMu.Unlock()

	ev := JobEvent{Type: JobEventStatus, Status: JobRunning}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case e := <-sub1:
			if e.JobID != "job1" || e.Project != "p1" || e.Website != "s1" {
				t.Errorf("sub1: unexpected event: %+v", e)
			}
		case <-time.After(1 * time.Second):
			t.Error("sub1: timed out waiting for event")
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case e := <-sub2:
			if e.JobID != "job1" || e.Project != "p1" || e.Website != "s1" {
				t.Errorf("sub2: unexpected event: %+v", e)
			}
		case <-time.After(1 * time.Second):
			t.Error("sub2: timed out waiting for event")
		}
	}()

	// Small delay to ensure subscribers are ready
	time.Sleep(10 * time.Millisecond)
	o.emitJobEvent("job1", ev)

	wg.Wait()
}

func TestOrchestrator_Unsubscribe(t *testing.T) {
	o := NewOrchestrator(nil, nil, &testutil.DummyLogger{})
	ctx, cancel := context.WithCancel(context.Background())

	sub := o.Subscribe(ctx)

	o.subsMu.RLock()
	count := len(o.subscribers)
	o.subsMu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 subscriber, got %d", count)
	}

	cancel() // Should trigger unsubscribe

	// Wait for goroutine to process cancellation
	time.Sleep(50 * time.Millisecond)

	o.subsMu.RLock()
	count = len(o.subscribers)
	o.subsMu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 subscribers after cancel, got %d", count)
	}

	// Verify channel is closed
	_, ok := <-sub
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestOrchestrator_SlowSubscriber(t *testing.T) {
	o := NewOrchestrator(nil, nil, &testutil.DummyLogger{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Slow subscriber (unbuffered or small buffer, and we don't read)
	_ = o.Subscribe(ctx)

	// Normal subscriber
	sub2 := o.Subscribe(ctx)

	o.jobsMu.Lock()
	if o.jobs == nil {
		o.jobs = make(map[string]*Job)
	}
	o.jobs["job1"] = &Job{ID: "job1", Project: "p1", Website: "s1"}
	o.jobsMu.Unlock()

	// Emit many events
	for i := 0; i < 200; i++ {
		o.emitJobEvent("job1", JobEvent{Type: JobEventProgress, Processed: i})
	}

	// sub2 should still receive events or at least not be blocked by sub1
	select {
	case <-sub2:
		// OK
	case <-time.After(500 * time.Millisecond):
		t.Error("normal subscriber blocked by slow subscriber")
	}
}

func TestOrchestrator_CloseClosesSubscribers(t *testing.T) {
	o := NewOrchestrator(nil, nil, &testutil.DummyLogger{})
	ctx := context.Background() // No cancelation here

	sub := o.Subscribe(ctx)

	o.Close()

	// Verify channel is closed
	select {
	case _, ok := <-sub:
		if ok {
			t.Error("expected channel to be closed after Orchestrator.Close()")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for channel to close after Orchestrator.Close()")
	}
}
