package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SSEWriter SSE 写入器
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter 创建 SSE 写入器
func NewSSEWriter(c *gin.Context) (*SSEWriter, error) {
	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no") // nginx 不缓冲

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	return &SSEWriter{
		w:       c.Writer,
		flusher: flusher,
	}, nil
}

// SSEEvent SSE 事件
type SSEEvent struct {
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// WriteEvent 写入 SSE 事件
func (s *SSEWriter) WriteEvent(event string, data interface{}) error {
	eventData := map[string]interface{}{
		"event":     event,
		"data":      data,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(eventData)
	if err != nil {
		return err
	}

	// SSE 格式: event: xxx\ndata: {json}\n\n
	if err := s.writeString("event: %s\n", event); err != nil {
		return err
	}
	if err := s.writeString("data: %s\n\n", jsonData); err != nil {
		return err
	}
	s.flusher.Flush()

	return nil
}

// WriteData 写入数据（无事件类型）
func (s *SSEWriter) WriteData(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := s.writeString("data: %s\n\n", jsonData); err != nil {
		return err
	}
	s.flusher.Flush()

	return nil
}

// WriteComment 写入注释（心跳）
func (s *SSEWriter) WriteComment(comment string) {
	if err := s.writeString(": %s\n\n", comment); err != nil {
		return
	}
	s.flusher.Flush()
}

// Close 关闭连接
func (s *SSEWriter) Close() {
	// 发送关闭事件
	_ = s.WriteEvent("close", map[string]string{"message": "stream closed"})
}

func (s *SSEWriter) writeString(format string, value interface{}) error {
	_, err := fmt.Fprintf(s.w, format, value)
	return err
}
