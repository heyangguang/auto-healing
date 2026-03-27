package repository

import (
	"context"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TenantDB(db *gorm.DB, ctx context.Context) *gorm.DB {
	return platformrepo.TenantDB(db, ctx)
}

func TenantIDFromContextOK(ctx context.Context) (uuid.UUID, bool) {
	return platformrepo.TenantIDFromContextOK(ctx)
}

func TenantIDFromContext(ctx context.Context) uuid.UUID {
	return platformrepo.TenantIDFromContext(ctx)
}

func RequireTenantID(ctx context.Context) (uuid.UUID, error) {
	return platformrepo.RequireTenantID(ctx)
}

func FillTenantID(ctx context.Context, tenantID **uuid.UUID) error {
	return platformrepo.FillTenantID(ctx, tenantID)
}

func UpdateTenantScopedModel(db *gorm.DB, ctx context.Context, id uuid.UUID, entity interface{}, omit ...string) error {
	return platformrepo.UpdateTenantScopedModel(db, ctx, id, entity, omit...)
}
