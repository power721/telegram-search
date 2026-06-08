package task

import (
	"context"
	"sync"
	"time"
)

const (
	EventTaskUpdated     = "task.updated"
	EventAccountUpdated  = "account.updated"
	EventListenerUpdated = "listener.updated"
	EventActivityCreated = "activity.created"
)

type Event struct {
	Type      string    `json:"type"`
	Payload   any       `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type EventBroker struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
	buffer      int
}

func NewEventBroker() *EventBroker {
	return &EventBroker{
		subscribers: map[chan Event]struct{}{},
		buffer:      16,
	}
}

func (b *EventBroker) Subscribe(ctx context.Context) (<-chan Event, func()) {
	ch := make(chan Event, b.buffer)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subscribers, ch)
			close(ch)
			b.mu.Unlock()
		})
	}
	go func() {
		<-ctx.Done()
		unsubscribe()
	}()
	return ch, unsubscribe
}

func (b *EventBroker) Publish(event Event) {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- event:
			default:
			}
		}
	}
}
