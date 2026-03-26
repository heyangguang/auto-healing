package middleware

import (
	"context"
	"sync"
)

type asyncLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var (
	middlewareLifecycleMu sync.Mutex
	middlewareLifecycle   *asyncLifecycle
)

func ensureMiddlewareLifecycle() *asyncLifecycle {
	middlewareLifecycleMu.Lock()
	defer middlewareLifecycleMu.Unlock()
	if middlewareLifecycle == nil || middlewareLifecycle.ctx.Err() != nil {
		ctx, cancel := context.WithCancel(context.Background())
		middlewareLifecycle = &asyncLifecycle{ctx: ctx, cancel: cancel}
	}
	return middlewareLifecycle
}

func (l *asyncLifecycle) Go(fn func(context.Context)) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
}

func Shutdown() {
	middlewareLifecycleMu.Lock()
	lifecycle := middlewareLifecycle
	middlewareLifecycle = nil
	middlewareLifecycleMu.Unlock()
	if lifecycle == nil {
		return
	}
	lifecycle.cancel()
	lifecycle.wg.Wait()
}
