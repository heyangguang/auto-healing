package healing

import (
	"context"
	"testing"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestDetachContextPreservesTenantAndRemovesCancellation(t *testing.T) {
	tenantID := uuid.New()
	parent, cancel := context.WithCancel(platformrepo.WithTenantID(context.Background(), tenantID))
	cancel()

	detached := detachContext(parent)

	gotTenantID, err := platformrepo.RequireTenantID(detached)
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
