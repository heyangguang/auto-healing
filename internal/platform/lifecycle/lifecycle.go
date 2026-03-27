package lifecycle

import (
	"context"
	"sync"
)

var (
	cleanupMu sync.Mutex
	cleanups  []func()
	tasks     *asyncLifecycle
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

func ensureTasks() *asyncLifecycle {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	if tasks == nil || tasks.ctx.Err() != nil {
		tasks = newAsyncLifecycle()
	}
	return tasks
}

func Go(fn func(context.Context)) {
	lifecycle := ensureTasks()
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

func RegisterCleanup(fn func()) {
	if fn == nil {
		return
	}
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanups = append(cleanups, fn)
}

func Cleanup() {
	cleanupMu.Lock()
	currentCleanups := cleanups
	cleanups = nil
	currentTasks := tasks
	tasks = nil
	cleanupMu.Unlock()

	for i := len(currentCleanups) - 1; i >= 0; i-- {
		currentCleanups[i]()
	}
	if currentTasks != nil {
		currentTasks.Stop()
	}
}
