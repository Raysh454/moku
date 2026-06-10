package app

import (
	"context"
	"sync"

	"github.com/raysh454/moku/internal/logging"
)

type JobEventType string

const (
	JobEventStatus   JobEventType = "status"
	JobEventProgress JobEventType = "progress"
	JobEventResult   JobEventType = "result"
)

type JobEvent struct {
	JobID     string       `json:"job_id"`
	Project   string       `json:"project,omitempty"`
	Website   string       `json:"website,omitempty"`
	Type      JobEventType `json:"type"`
	Status    JobStatus    `json:"status,omitempty"`
	Error     string       `json:"error,omitempty"`
	Processed int          `json:"processed,omitempty"`
	Failed    int          `json:"failed,omitempty"`
	Total     int          `json:"total,omitempty"`
}

// subscriberEventBufferSize is the per-subscriber channel capacity. A
// subscriber that falls more than this many events behind starts losing
// events rather than blocking publishers.
const subscriberEventBufferSize = 100

// subscriberBroker fans JobEvents out to subscribers. Every subscriber owns
// a buffered channel and sends are non-blocking: events are dropped for
// subscribers whose buffers are full.
type subscriberBroker struct {
	logger logging.Logger

	mu          sync.RWMutex
	subscribers []chan JobEvent
	closed      bool
}

func newSubscriberBroker(logger logging.Logger) *subscriberBroker {
	return &subscriberBroker{logger: logger}
}

// subscribe registers a new subscriber channel. The subscriber is
// automatically removed (and its channel closed) when ctx is done. After
// close, subscribe hands out an already-closed channel so new subscribers
// exit immediately.
func (b *subscriberBroker) subscribe(ctx context.Context) chan JobEvent {
	ch := make(chan JobEvent, subscriberEventBufferSize)

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(ch)
		return ch
	}
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.unsubscribe(ch)
	}()
	return ch
}

// unsubscribe removes ch from the subscriber list and closes it. The close
// happens under the write lock so publish can never send on a closed channel.
func (b *subscriberBroker) unsubscribe(ch chan JobEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, sub := range b.subscribers {
		if sub == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// publish delivers ev to every subscriber without blocking: subscribers
// whose buffers are full miss the event.
func (b *subscriberBroker) publish(ev JobEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.subscribers) == 0 {
		return
	}

	for _, sub := range b.subscribers {
		select {
		case sub <- ev:
		default:
			if b.logger != nil {
				b.logger.Debug("broker: subscriber buffer full, dropping event",
					logging.Field{Key: "job_id", Value: ev.JobID},
					logging.Field{Key: "event_type", Value: ev.Type},
					logging.Field{Key: "subscribers", Value: len(b.subscribers)})
			}
		}
	}
}

// close drains every subscriber channel and marks the broker closed so
// subsequent subscribe calls return an already-closed channel. Idempotent.
func (b *subscriberBroker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, sub := range b.subscribers {
		close(sub)
	}
	b.subscribers = nil
}

// subscriberCount reports how many subscribers are currently registered.
func (b *subscriberBroker) subscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
