package execution

import (
	"context"
	"sync"

	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

type asyncLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	sem    chan struct{}
}

func newAsyncLifecycle(maxWorkers int) *asyncLifecycle {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncLifecycle{
		ctx:    ctx,
		cancel: cancel,
		sem:    make(chan struct{}, maxWorkers),
	}
}

func (l *asyncLifecycle) Go(fn func(context.Context)) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
}

func (l *asyncLifecycle) Acquire(ctx context.Context) bool {
	select {
	case l.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (l *asyncLifecycle) Release() {
	<-l.sem
}

func (l *asyncLifecycle) Stop() {
	l.cancel()
	l.wg.Wait()
}

func withTenantContext(ctx context.Context, tenantID *uuid.UUID) context.Context {
	if tenantID == nil {
		return ctx
	}
	return repository.WithTenantID(ctx, *tenantID)
}
