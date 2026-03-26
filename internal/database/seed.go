package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PermissionSeed 权限种子定义
type PermissionSeed struct {
	Code        string
	Name        string
	Description string
	Module      string
	Resource    string
	Action      string
}

// RoleSeed 角色种子定义
type RoleSeed struct {
	Name        string
	DisplayName string
	Description string
	IsSystem    bool
	Scope       string
	Permissions []string
}

// SyncPermissionsAndRoles 同步预置权限和角色（启动时调用）
func SyncPermissionsAndRoles() error {
	ctx := context.Background()

	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		permCodeToID, err := syncPermissions(tx)
		if err != nil {
			return err
		}
		logger.Info("权限同步完成，共 %d 个权限", len(permCodeToID))

		if err := syncSystemRoles(tx, permCodeToID); err != nil {
			return err
		}
		logger.Info("角色权限同步完成")
		return nil
	})
}

func syncPermissions(tx *gorm.DB) (map[string]uuid.UUID, error) {
	logger.Info("同步系统预置权限...")
	for _, seed := range AllPermissions {
		if err := upsertPermission(tx, seed); err != nil {
			return nil, err
		}
	}
	return loadPermissionIDs(tx)
}

func upsertPermission(tx *gorm.DB, seed PermissionSeed) error {
	var permission model.Permission
	err := tx.Where("code = ?", seed.Code).First(&permission).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Create(&model.Permission{
			Code:        seed.Code,
			Name:        seed.Name,
			Description: seed.Description,
			Module:      seed.Module,
			Resource:    seed.Resource,
			Action:      seed.Action,
		}).Error
	case err != nil:
		return err
	default:
		return tx.Model(&model.Permission{}).
			Where("code = ?", seed.Code).
			Updates(permissionFields(seed)).Error
	}
}

func permissionFields(seed PermissionSeed) map[string]interface{} {
	return map[string]interface{}{
		"name":        seed.Name,
		"description": seed.Description,
		"module":      seed.Module,
		"resource":    seed.Resource,
		"action":      seed.Action,
	}
}

func loadPermissionIDs(tx *gorm.DB) (map[string]uuid.UUID, error) {
	var allPerms []model.Permission
	if err := tx.Find(&allPerms).Error; err != nil {
		return nil, err
	}
	permCodeToID := make(map[string]uuid.UUID, len(allPerms))
	for _, permission := range allPerms {
		permCodeToID[permission.Code] = permission.ID
	}
	return permCodeToID, nil
}

func syncSystemRoles(tx *gorm.DB, permCodeToID map[string]uuid.UUID) error {
	logger.Info("同步系统预置角色...")
	for _, roleSeed := range SystemRoles {
		role, err := upsertSystemRole(tx, roleSeed)
		if err != nil {
			return err
		}
		if roleSeed.IsSystem && len(roleSeed.Permissions) > 0 {
			if err := syncRolePermissions(tx, role.ID, roleSeed.Permissions, permCodeToID); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertSystemRole(tx *gorm.DB, roleSeed RoleSeed) (*model.Role, error) {
	var role model.Role
	result := tx.Where("name = ?", roleSeed.Name).First(&role)
	scope := roleSeed.Scope
	if scope == "" {
		scope = "tenant"
	}

	switch result.Error {
	case gorm.ErrRecordNotFound:
		role = model.Role{
			Name:        roleSeed.Name,
			DisplayName: roleSeed.DisplayName,
			Description: roleSeed.Description,
			IsSystem:    roleSeed.IsSystem,
			Scope:       scope,
		}
		if err := tx.Create(&role).Error; err != nil {
			return nil, err
		}
	case nil:
		if err := tx.Model(&role).Updates(map[string]interface{}{
			"display_name": roleSeed.DisplayName,
			"description":  roleSeed.Description,
			"is_system":    roleSeed.IsSystem,
			"scope":        scope,
		}).Error; err != nil {
			return nil, err
		}
	default:
		return nil, result.Error
	}
	return &role, nil
}

func syncRolePermissions(tx *gorm.DB, roleID uuid.UUID, permissionCodes []string, permCodeToID map[string]uuid.UUID) error {
	var existingRolePerms []model.RolePermission
	if err := tx.Where("role_id = ?", roleID).Find(&existingRolePerms).Error; err != nil {
		return err
	}
	existingPermIDs := make(map[uuid.UUID]bool, len(existingRolePerms))
	for _, rolePermission := range existingRolePerms {
		existingPermIDs[rolePermission.PermissionID] = true
	}
	desiredPermIDs := make(map[uuid.UUID]bool, len(permissionCodes))

	for _, permCode := range permissionCodes {
		permID, ok := permCodeToID[permCode]
		if !ok {
			return fmt.Errorf("角色 %s 引用了不存在的权限码 %s", roleID, permCode)
		}
		desiredPermIDs[permID] = true
		if existingPermIDs[permID] {
			continue
		}
		rolePermission := model.RolePermission{
			RoleID:       roleID,
			PermissionID: permID,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rolePermission).Error; err != nil {
			return err
		}
	}
	for _, rolePermission := range existingRolePerms {
		if desiredPermIDs[rolePermission.PermissionID] {
			continue
		}
		if err := tx.Delete(&rolePermission).Error; err != nil {
			return err
		}
	}
	return nil
}
