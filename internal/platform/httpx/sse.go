package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewSSEWriter(c *gin.Context) (*SSEWriter, error) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	return &SSEWriter{
		w:       c.Writer,
		flusher: flusher,
	}, nil
}

type SSEEvent struct {
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

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

	if err := s.writeString("event: %s\n", event); err != nil {
		return err
	}
	if err := s.writeString("data: %s\n\n", jsonData); err != nil {
		return err
	}
	s.flusher.Flush()

	return nil
}

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

func (s *SSEWriter) WriteComment(comment string) {
	if err := s.writeString(": %s\n\n", comment); err != nil {
		return
	}
	s.flusher.Flush()
}

func (s *SSEWriter) Close() {
	_ = s.WriteEvent("close", map[string]string{"message": "stream closed"})
}

func (s *SSEWriter) writeString(format string, value interface{}) error {
	_, err := fmt.Fprintf(s.w, format, value)
	return err
}
