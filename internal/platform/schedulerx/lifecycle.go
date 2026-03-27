package schedulerx

import (
	"context"
	"sync"
)

type Lifecycle struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	stopped bool
}

func NewLifecycle() *Lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &Lifecycle{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (l *Lifecycle) Context() context.Context {
	return l.ctx
}

func (l *Lifecycle) Go(fn func(context.Context)) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped || l.ctx.Err() != nil {
		return false
	}
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		fn(l.ctx)
	}()
	return true
}

func (l *Lifecycle) Stop() {
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return
	}
	l.stopped = true
	cancel := l.cancel
	l.mu.Unlock()

	cancel()
	l.wg.Wait()
}
