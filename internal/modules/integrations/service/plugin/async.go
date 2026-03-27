package plugin

import (
	"context"
	"sync"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

type asyncLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newAsyncLifecycle() *asyncLifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &asyncLifecycle{ctx: ctx, cancel: cancel}
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

func (s *Service) ensureLifecycle() *asyncLifecycle {
	if s.lifecycle == nil || s.lifecycle.ctx.Err() != nil {
		s.lifecycle = newAsyncLifecycle()
	}
	return s.lifecycle
}

func (s *Service) Go(fn func(context.Context)) {
	s.ensureLifecycle().Go(fn)
}

func (s *Service) Shutdown() {
	if s.lifecycle != nil {
		s.lifecycle.Stop()
	}
}

func withTenantContext(ctx context.Context, tenantID *uuid.UUID) context.Context {
	if tenantID == nil {
		return ctx
	}
	return platformrepo.WithTenantID(ctx, *tenantID)
}
