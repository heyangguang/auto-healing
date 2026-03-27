package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *RoleRepository) GetTenantRoleByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Role, error) {
	return r.getTenantRole(ctx, tenantID, "id = ?", id)
}

func (r *RoleRepository) GetTenantRoleByName(ctx context.Context, tenantID uuid.UUID, name string) (*model.Role, error) {
	return r.getTenantRole(ctx, tenantID, "name = ?", name)
}

func (r *RoleRepository) getTenantRole(ctx context.Context, tenantID uuid.UUID, predicate string, value any) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions", rolePermissionPreloadScope("tenant")).
		Where("scope = ?", "tenant").
		Where("(tenant_id IS NULL OR tenant_id = ?)", tenantID).
		Where(predicate, value).
		First(&role).Error
	if err == gorm.ErrRecordNotFound {
		return nil, ErrRoleNotFound
	}
	return &role, err
}
