package healing

import (
	"sync"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// EventType SSE 事件类型
type EventType string

const (
	EventFlowStart    EventType = "flow_start"
	EventNodeStart    EventType = "node_start"
	EventNodeLog      EventType = "node_log"
	EventNodeComplete EventType = "node_complete"
	EventFlowComplete EventType = "flow_complete"
)

// Event 事件
type Event struct {
	Type      EventType              `json:"event"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// EventBus 事件总线，用于实际执行的 SSE 推送
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID][]chan Event // instanceID -> channels
}

var (
	globalEventBus *EventBus
	eventBusOnce   sync.Once
)

// GetEventBus 获取全局事件总线
func GetEventBus() *EventBus {
	eventBusOnce.Do(func() {
		globalEventBus = &EventBus{
			subscribers: make(map[uuid.UUID][]chan Event),
		}
	})
	return globalEventBus
}

// Subscribe 订阅实例事件
func (eb *EventBus) Subscribe(instanceID uuid.UUID) chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan Event, 100) // 缓冲区
	eb.subscribers[instanceID] = append(eb.subscribers[instanceID], ch)
	return ch
}

// Unsubscribe 取消订阅
func (eb *EventBus) Unsubscribe(instanceID uuid.UUID, ch chan Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	channels := eb.subscribers[instanceID]
	for i, c := range channels {
		if c == ch {
			eb.subscribers[instanceID] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}

	// 如果没有订阅者了，删除
	if len(eb.subscribers[instanceID]) == 0 {
		delete(eb.subscribers, instanceID)
	}
}

// Publish 发布事件
func (eb *EventBus) Publish(instanceID uuid.UUID, event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	event.Timestamp = time.Now()

	for _, ch := range eb.subscribers[instanceID] {
		select {
		case ch <- event:
		default:
			// 缓冲区满，记录警告（可能丢失关键事件）
			logger.Exec("SSE").Warn("事件缓冲区已满，丢弃事件 type=%s instance=%s", event.Type, instanceID.String()[:8])
		}
	}
}

// PublishNodeStart 发布节点开始事件
func (eb *EventBus) PublishNodeStart(instanceID uuid.UUID, nodeID, nodeType, nodeName string) {
	eb.Publish(instanceID, Event{
		Type: EventNodeStart,
		Data: map[string]interface{}{
			"node_id":   nodeID,
			"node_type": nodeType,
			"node_name": nodeName,
			"status":    "running",
		},
	})
}

// PublishNodeLog 发布节点日志事件
func (eb *EventBus) PublishNodeLog(instanceID uuid.UUID, nodeID, nodeType, level, message string, details map[string]interface{}) {
	eb.Publish(instanceID, Event{
		Type: EventNodeLog,
		Data: map[string]interface{}{
			"node_id":   nodeID,
			"node_type": nodeType,
			"level":     level,
			"message":   message,
			"details":   details,
		},
	})
}

// PublishNodeComplete 发布节点完成事件
func (eb *EventBus) PublishNodeComplete(instanceID uuid.UUID, nodeID, nodeType, status string, input map[string]interface{}, process []string, output map[string]interface{}, outputHandle string) {
	eb.Publish(instanceID, Event{
		Type: EventNodeComplete,
		Data: map[string]interface{}{
			"node_id":       nodeID,
			"node_type":     nodeType,
			"status":        status,
			"input":         input,
			"process":       process,
			"output":        output,
			"output_handle": outputHandle,
		},
	})
}

// PublishFlowStart 发布流程开始事件
func (eb *EventBus) PublishFlowStart(instanceID, flowID uuid.UUID, flowName string) {
	eb.Publish(instanceID, Event{
		Type: EventFlowStart,
		Data: map[string]interface{}{
			"instance_id": instanceID.String(),
			"flow_id":     flowID.String(),
			"flow_name":   flowName,
		},
	})
}

// PublishFlowComplete 发布流程完成事件
func (eb *EventBus) PublishFlowComplete(instanceID uuid.UUID, success bool, status, message string) {
	eb.Publish(instanceID, Event{
		Type: EventFlowComplete,
		Data: map[string]interface{}{
			"success": success,
			"status":  status,
			"message": message,
		},
	})
}
