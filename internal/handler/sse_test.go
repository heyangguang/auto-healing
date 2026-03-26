package handler

import (
	"errors"
	"net/http"
	"testing"
)

type failingSSEWriter struct {
	err error
}

func (w *failingSSEWriter) Header() http.Header { return http.Header{} }
func (w *failingSSEWriter) WriteHeader(int)     {}
func (w *failingSSEWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type noopFlusher struct{}

func (noopFlusher) Flush() {}

func TestSSEWriterWriteEventPropagatesWriteError(t *testing.T) {
	wantErr := errors.New("write failed")
	writer := &SSEWriter{
		w:       &failingSSEWriter{err: wantErr},
		flusher: noopFlusher{},
	}

	err := writer.WriteEvent("test", map[string]string{"ok": "no"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteEvent() error = %v, want %v", err, wantErr)
	}
}

func TestSSEWriterWriteDataPropagatesWriteError(t *testing.T) {
	wantErr := errors.New("write failed")
	writer := &SSEWriter{
		w:       &failingSSEWriter{err: wantErr},
		flusher: noopFlusher{},
	}

	err := writer.WriteData(map[string]string{"ok": "no"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteData() error = %v, want %v", err, wantErr)
	}
}
