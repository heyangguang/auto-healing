package healing

import "context"

func (e *FlowExecutor) ensureLifecycle() *asyncLifecycle {
	if e.lifecycle == nil || e.lifecycle.ctx.Err() != nil {
		e.lifecycle = newAsyncLifecycle()
	}
	return e.lifecycle
}

func (e *FlowExecutor) Go(fn func(context.Context)) {
	e.ensureLifecycle().Go(fn)
}

func (e *FlowExecutor) Shutdown() {
	if e.lifecycle != nil {
		e.lifecycle.Stop()
	}
}
