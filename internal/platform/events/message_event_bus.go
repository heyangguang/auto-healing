package events

import (
	"sync"

	"github.com/google/uuid"
)

type MessageEvent struct {
	Type string `json:"type"`
}

type MessageEventBus struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID][]chan MessageEvent
}

var (
	globalMessageEventBus *MessageEventBus
	messageEventBusOnce   sync.Once
)

func NewMessageEventBus() *MessageEventBus {
	return &MessageEventBus{
		subscribers: make(map[uuid.UUID][]chan MessageEvent),
	}
}

func GetMessageEventBus() *MessageEventBus {
	messageEventBusOnce.Do(func() {
		globalMessageEventBus = NewMessageEventBus()
	})
	return globalMessageEventBus
}

func (eb *MessageEventBus) Subscribe(userID uuid.UUID) chan MessageEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan MessageEvent, 10)
	eb.subscribers[userID] = append(eb.subscribers[userID], ch)
	return ch
}

func (eb *MessageEventBus) Unsubscribe(userID uuid.UUID, ch chan MessageEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	channels := eb.subscribers[userID]
	for i, current := range channels {
		if current == ch {
			eb.subscribers[userID] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}

	if len(eb.subscribers[userID]) == 0 {
		delete(eb.subscribers, userID)
	}
}

func (eb *MessageEventBus) NotifyUser(userID uuid.UUID) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	event := MessageEvent{Type: "new_message"}
	for _, ch := range eb.subscribers[userID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (eb *MessageEventBus) Broadcast() {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	event := MessageEvent{Type: "new_message"}
	for _, channels := range eb.subscribers {
		for _, ch := range channels {
			select {
			case ch <- event:
			default:
			}
		}
	}
}

func (eb *MessageEventBus) GetOnlineCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	count := 0
	for _, channels := range eb.subscribers {
		count += len(channels)
	}
	return count
}
