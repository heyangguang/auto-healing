package audit

import (
	"context"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return platformrepo.WithTenantID(ctx, tenantID)
}

func TenantIDFromContextOK(ctx context.Context) (uuid.UUID, bool) {
	return platformrepo.TenantIDFromContextOK(ctx)
}

func tenantDB(db *gorm.DB, ctx context.Context) *gorm.DB {
	return platformrepo.TenantDB(db, ctx)
}
