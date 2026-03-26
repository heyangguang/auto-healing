package healing

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
}

func newAsyncLifecycle() *asyncLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncLifecycle{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (l *asyncLifecycle) Go(fn func(context.Context)) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
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

func detachContext(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
