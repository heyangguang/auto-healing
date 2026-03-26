package middleware

import (
	"context"
	"sync"
)

type asyncLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

var (
	middlewareLifecycleMu sync.Mutex
	middlewareLifecycle   *asyncLifecycle
)

func newAsyncLifecycle() *asyncLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncLifecycle{ctx: ctx, cancel: cancel}
}

func ensureMiddlewareLifecycle() *asyncLifecycle {
	middlewareLifecycleMu.Lock()
	defer middlewareLifecycleMu.Unlock()
	if middlewareLifecycle == nil || middlewareLifecycle.ctx.Err() != nil {
		middlewareLifecycle = newAsyncLifecycle()
	}
	return middlewareLifecycle
}

func (l *asyncLifecycle) Go(fn func(context.Context)) {
	if fn == nil {
		return
	}

	l.mu.Lock()
	if l.closed || l.ctx.Err() != nil {
		l.mu.Unlock()
		return
	}
	l.wg.Add(1)
	ctx := l.ctx
	l.mu.Unlock()

	go func() {
		defer l.wg.Done()
		fn(ctx)
	}()
}

func (l *asyncLifecycle) Stop() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	cancel := l.cancel
	l.mu.Unlock()

	cancel()
	l.wg.Wait()
}

func Shutdown() {
	middlewareLifecycleMu.Lock()
	lifecycle := middlewareLifecycle
	middlewareLifecycle = nil
	middlewareLifecycleMu.Unlock()
	if lifecycle == nil {
		return
	}
	lifecycle.Stop()
}
