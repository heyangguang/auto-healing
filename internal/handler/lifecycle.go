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
}

func ensureHandlerTasks() *asyncLifecycle {
	handlerCleanupMu.Lock()
	defer handlerCleanupMu.Unlock()
	if handlerTasks == nil || handlerTasks.ctx.Err() != nil {
		ctx, cancel := context.WithCancel(context.Background())
		handlerTasks = &asyncLifecycle{ctx: ctx, cancel: cancel}
	}
	return handlerTasks
}

func goHandlerTask(fn func(context.Context)) {
	lifecycle := ensureHandlerTasks()
	lifecycle.wg.Add(1)
	go func() {
		defer lifecycle.wg.Done()
		fn(lifecycle.ctx)
	}()
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
		tasks.cancel()
		tasks.wg.Wait()
	}
}
