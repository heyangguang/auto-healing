package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunHTTPServerInvokesShutdownHook(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var shutdownCalled atomic.Bool
	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NewServeMux(),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- runHTTPServer(ctx, server, time.Second, func() {
			shutdownCalled.Store(true)
		})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runHTTPServer() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runHTTPServer() did not stop in time")
	}

	if !shutdownCalled.Load() {
		t.Fatal("shutdown hook was not called")
	}
}
