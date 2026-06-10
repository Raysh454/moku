package app

import (
	"context"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/testutil"
)

func newTestBroker() *subscriberBroker {
	return newSubscriberBroker(&testutil.DummyLogger{})
}

func TestSubscriberBroker_PublishWithoutSubscribersIsANoOp(t *testing.T) {
	t.Parallel()
	b := newTestBroker()

	// Must neither panic nor block.
	b.publish(JobEvent{JobID: "job1", Type: JobEventStatus})
}

func TestSubscriberBroker_DeliversEventToEverySubscriber(t *testing.T) {
	t.Parallel()
	b := newTestBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub1 := b.subscribe(ctx)
	sub2 := b.subscribe(ctx)

	b.publish(JobEvent{JobID: "job1", Type: JobEventStatus, Status: JobRunning})

	for name, sub := range map[string]chan JobEvent{"sub1": sub1, "sub2": sub2} {
		select {
		case ev := <-sub:
			if ev.JobID != "job1" || ev.Status != JobRunning {
				t.Errorf("%s: unexpected event: %+v", name, ev)
			}
		case <-time.After(time.Second):
			t.Errorf("%s: timed out waiting for event", name)
		}
	}
}

func TestSubscriberBroker_DropsEventsInsteadOfBlockingWhenSubscriberBufferIsFull(t *testing.T) {
	t.Parallel()
	b := newTestBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stuck := b.subscribe(ctx) // never read from

	published := make(chan struct{})
	go func() {
		for i := 0; i < subscriberEventBufferSize*2; i++ {
			b.publish(JobEvent{JobID: "job1", Type: JobEventProgress, Processed: i})
		}
		close(published)
	}()

	select {
	case <-published:
		// publish never blocked on the full buffer
	case <-time.After(2 * time.Second):
		t.Fatal("publish blocked on a subscriber with a full buffer")
	}

	if got := len(stuck); got != subscriberEventBufferSize {
		t.Errorf("buffered events = %d, want %d (overflow must be dropped)", got, subscriberEventBufferSize)
	}
}

func TestSubscriberBroker_CancelingSubscriberContextUnsubscribesAndClosesChannel(t *testing.T) {
	t.Parallel()
	b := newTestBroker()
	ctx, cancel := context.WithCancel(context.Background())

	sub := b.subscribe(ctx)
	if got := b.subscriberCount(); got != 1 {
		t.Fatalf("subscriber count = %d, want 1", got)
	}

	cancel()

	select {
	case _, ok := <-sub:
		if ok {
			t.Fatal("expected closed channel after context cancellation, got event")
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber channel not closed after context cancellation")
	}
	if got := b.subscriberCount(); got != 0 {
		t.Errorf("subscriber count after cancellation = %d, want 0", got)
	}
}

func TestSubscriberBroker_CloseClosesEverySubscriberChannel(t *testing.T) {
	t.Parallel()
	b := newTestBroker()

	sub := b.subscribe(context.Background())

	b.close()

	select {
	case _, ok := <-sub:
		if ok {
			t.Error("expected channel to be closed after broker close, got event")
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for channel to close after broker close")
	}
}

func TestSubscriberBroker_SubscribeAfterCloseReturnsClosedChannel(t *testing.T) {
	t.Parallel()
	b := newTestBroker()

	b.close()

	sub := b.subscribe(context.Background())
	if _, ok := <-sub; ok {
		t.Error("expected an already-closed channel from a closed broker")
	}
}

func TestSubscriberBroker_CloseIsIdempotent(t *testing.T) {
	t.Parallel()
	b := newTestBroker()
	_ = b.subscribe(context.Background())

	// A second close must not panic (e.g. by double-closing channels).
	b.close()
	b.close()
}
