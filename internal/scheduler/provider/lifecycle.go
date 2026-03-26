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
}

func newSchedulerLifecycle() *schedulerLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &schedulerLifecycle{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (l *schedulerLifecycle) Go(fn func(context.Context)) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
}

func (l *schedulerLifecycle) Stop() {
	l.cancel()
	l.wg.Wait()
}

func withTenantContext(ctx context.Context, tenantID *uuid.UUID) context.Context {
	if tenantID == nil {
		return ctx
	}
	return repository.WithTenantID(ctx, *tenantID)
}
