package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssignPermissions 为角色分配权限
func (r *RoleRepository) AssignPermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	role, err := r.loadRoleForPermissionMutation(ctx, roleID)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return errors.New("系统内置角色的权限不允许修改")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		for _, permissionID := range permissionIDs {
			if err := tx.Create(&model.RolePermission{
				ID:           uuid.New(),
				RoleID:       roleID,
				PermissionID: permissionID,
				CreatedAt:    time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RoleRepository) loadRoleForPermissionMutation(ctx context.Context, roleID uuid.UUID) (*model.Role, error) {
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		return r.GetTenantRoleByID(ctx, tenantID, roleID)
	}
	return r.GetByID(ctx, roleID)
}
