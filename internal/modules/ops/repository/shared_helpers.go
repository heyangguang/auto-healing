package repository

import (
	"context"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func FillTenantID(ctx context.Context, tenantID **uuid.UUID) error {
	return platformrepo.FillTenantID(ctx, tenantID)
}

func RequireTenantID(ctx context.Context) (uuid.UUID, error) {
	return platformrepo.RequireTenantID(ctx)
}

func TenantDB(db *gorm.DB, ctx context.Context) *gorm.DB {
	return platformrepo.TenantDB(db, ctx)
}

func UpdateTenantScopedModel(db *gorm.DB, ctx context.Context, id uuid.UUID, value any, omit ...string) error {
	return platformrepo.UpdateTenantScopedModel(db, ctx, id, value, omit...)
}
