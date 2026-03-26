package handler

import (
	"context"
	"sync"
)

var (
	handlerCleanupMu sync.Mutex
	handlerCleanups  []func()
	handlerTasks     *asyncLifecycle
)

type asyncLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func newAsyncLifecycle() *asyncLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncLifecycle{ctx: ctx, cancel: cancel}
}

func ensureHandlerTasks() *asyncLifecycle {
	handlerCleanupMu.Lock()
	defer handlerCleanupMu.Unlock()
	if handlerTasks == nil || handlerTasks.ctx.Err() != nil {
		handlerTasks = newAsyncLifecycle()
	}
	return handlerTasks
}

func goHandlerTask(fn func(context.Context)) {
	lifecycle := ensureHandlerTasks()
	lifecycle.Go(fn)
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

func registerHandlerCleanup(fn func()) {
	if fn == nil {
		return
	}
	handlerCleanupMu.Lock()
	defer handlerCleanupMu.Unlock()
	handlerCleanups = append(handlerCleanups, fn)
}

func Cleanup() {
	handlerCleanupMu.Lock()
	cleanups := handlerCleanups
	handlerCleanups = nil
	tasks := handlerTasks
	handlerTasks = nil
	handlerCleanupMu.Unlock()

	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
	if tasks != nil {
		tasks.Stop()
	}
}
