package schedulerx

import (
	"context"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func WithTenantContext(ctx context.Context, tenantID *uuid.UUID) context.Context {
	if tenantID == nil {
		return ctx
	}
	return platformrepo.WithTenantID(ctx, *tenantID)
}
