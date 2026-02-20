package handler

import (
	"sync"

	"github.com/google/uuid"
)

// MessageEvent 站内消息事件
type MessageEvent struct {
	Type string `json:"type"` // "new_message"
}

// MessageEventBus 站内消息事件总线（全局单例）
// 按 userID 管理 SSE 订阅者，支持同一用户多客户端
type MessageEventBus struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID][]chan MessageEvent // userID -> channels
}

var (
	globalMessageEventBus *MessageEventBus
	messageEventBusOnce   sync.Once
)

// GetMessageEventBus 获取全局站内消息事件总线
func GetMessageEventBus() *MessageEventBus {
	messageEventBusOnce.Do(func() {
		globalMessageEventBus = &MessageEventBus{
			subscribers: make(map[uuid.UUID][]chan MessageEvent),
		}
	})
	return globalMessageEventBus
}

// Subscribe 订阅消息事件（返回事件通道）
func (eb *MessageEventBus) Subscribe(userID uuid.UUID) chan MessageEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan MessageEvent, 10)
	eb.subscribers[userID] = append(eb.subscribers[userID], ch)
	return ch
}

// Unsubscribe 取消订阅
func (eb *MessageEventBus) Unsubscribe(userID uuid.UUID, ch chan MessageEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	channels := eb.subscribers[userID]
	for i, c := range channels {
		if c == ch {
			eb.subscribers[userID] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}

	if len(eb.subscribers[userID]) == 0 {
		delete(eb.subscribers, userID)
	}
}

// NotifyUser 通知指定用户有新消息
func (eb *MessageEventBus) NotifyUser(userID uuid.UUID) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	event := MessageEvent{Type: "new_message"}
	for _, ch := range eb.subscribers[userID] {
		select {
		case ch <- event:
		default:
			// 缓冲区满，跳过
		}
	}
}

// Broadcast 广播新消息通知给所有在线用户
// 站内消息是广播型的（全局或定向租户），直接通知所有在线用户
// 前端收到通知后会自行调 unread-count 接口判断是否有新消息
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

// GetOnlineCount 获取当前在线连接数（调试用）
func (eb *MessageEventBus) GetOnlineCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	count := 0
	for _, channels := range eb.subscribers {
		count += len(channels)
	}
	return count
}
