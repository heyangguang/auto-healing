package git

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

func TestDetachTenantContextRemovesCancellationAndPreservesTenant(t *testing.T) {
	tenantID := uuid.New()
	parent, cancel := context.WithCancel(repository.WithTenantID(context.Background(), tenantID))
	cancel()

	detached := detachTenantContext(parent, nil)

	if _, err := repository.RequireTenantID(detached); err != nil {
		t.Fatalf("RequireTenantID() error = %v", err)
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("detached context should not be cancelled: %v", err)
	}
}

func TestDetachTenantContextOverridesTenantWhenProvided(t *testing.T) {
	parent := repository.WithTenantID(context.Background(), uuid.New())
	overrideTenant := uuid.New()

	detached := detachTenantContext(parent, &overrideTenant)

	tenantID, err := repository.RequireTenantID(detached)
	if err != nil {
		t.Fatalf("RequireTenantID() error = %v", err)
	}
	if tenantID != overrideTenant {
		t.Fatalf("tenantID = %s, want %s", tenantID, overrideTenant)
	}
}

func TestDetachTenantContextWithoutTenantLeavesMissingTenantVisible(t *testing.T) {
	detached := detachTenantContext(context.Background(), nil)

	_, err := repository.RequireTenantID(detached)
	if !errors.Is(err, repository.ErrTenantContextRequired) {
		t.Fatalf("RequireTenantID() error = %v, want %v", err, repository.ErrTenantContextRequired)
	}
}

func TestServiceShutdownWaitsForBackgroundWorker(t *testing.T) {
	svc := &Service{lifecycle: newAsyncLifecycle()}
	started := make(chan struct{})
	stopped := make(chan struct{})

	svc.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(stopped)
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background worker did not start")
	}

	svc.Shutdown()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("background worker did not stop before Shutdown returned")
	}
}
