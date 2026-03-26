package healing

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

func TestDetachContextPreservesTenantAndRemovesCancellation(t *testing.T) {
	tenantID := uuid.New()
	parent, cancel := context.WithCancel(repository.WithTenantID(context.Background(), tenantID))
	cancel()

	detached := detachContext(parent)

	gotTenantID, err := repository.RequireTenantID(detached)
	if err != nil {
		t.Fatalf("RequireTenantID() error = %v", err)
	}
	if gotTenantID != tenantID {
		t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("detached context should not be cancelled: %v", err)
	}
}
