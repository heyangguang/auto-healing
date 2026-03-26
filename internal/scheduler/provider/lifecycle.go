package provider

import (
	"context"
	"sync"

	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

type schedulerLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func newSchedulerLifecycle() *schedulerLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &schedulerLifecycle{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (l *schedulerLifecycle) Go(fn func(context.Context)) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return false
	}
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
	return true
}

func (l *schedulerLifecycle) Stop() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	l.cancel()
	l.mu.Unlock()
	l.wg.Wait()
}

func withTenantContext(ctx context.Context, tenantID *uuid.UUID) context.Context {
	if tenantID == nil {
		return ctx
	}
	return repository.WithTenantID(ctx, *tenantID)
}
