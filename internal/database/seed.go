package database

import (
	"context"

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

func syncPermissions(tx *gorm.DB) (map[string]string, error) {
	logger.Info("同步系统预置权限...")
	for _, seed := range AllPermissions {
		perm := model.Permission{
			Code:        seed.Code,
			Name:        seed.Name,
			Description: seed.Description,
			Module:      seed.Module,
			Resource:    seed.Resource,
			Action:      seed.Action,
		}

		result := tx.Where("code = ?", seed.Code).First(&model.Permission{})
		switch result.Error {
		case gorm.ErrRecordNotFound:
			if err := tx.Create(&perm).Error; err != nil {
				return nil, err
			}
		case nil:
			tx.Model(&model.Permission{}).Where("code = ?", seed.Code).Updates(map[string]interface{}{
				"name":        seed.Name,
				"description": seed.Description,
				"module":      seed.Module,
				"resource":    seed.Resource,
				"action":      seed.Action,
			})
		}
	}

	var allPerms []model.Permission
	if err := tx.Find(&allPerms).Error; err != nil {
		return nil, err
	}
	permCodeToID := make(map[string]string, len(allPerms))
	for _, permission := range allPerms {
		permCodeToID[permission.Code] = permission.ID.String()
	}
	return permCodeToID, nil
}

func syncSystemRoles(tx *gorm.DB, permCodeToID map[string]string) error {
	logger.Info("同步系统预置角色...")
	for _, roleSeed := range SystemRoles {
		role, err := upsertSystemRole(tx, roleSeed)
		if err != nil {
			return err
		}
		if roleSeed.IsSystem && roleSeed.Permissions != nil {
			syncRolePermissions(tx, role.ID, roleSeed.Permissions, permCodeToID)
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
		tx.Model(&role).Updates(map[string]interface{}{
			"description": roleSeed.Description,
			"is_system":   roleSeed.IsSystem,
			"scope":       scope,
		})
	default:
		return nil, result.Error
	}
	return &role, nil
}

func syncRolePermissions(tx *gorm.DB, roleID uuid.UUID, permissionCodes []string, permCodeToID map[string]string) {
	var existingRolePerms []model.RolePermission
	tx.Where("role_id = ?", roleID).Find(&existingRolePerms)
	existingPermIDs := make(map[string]bool, len(existingRolePerms))
	for _, rolePermission := range existingRolePerms {
		existingPermIDs[rolePermission.PermissionID.String()] = true
	}

	for _, permCode := range permissionCodes {
		permID, ok := permCodeToID[permCode]
		if !ok {
			logger.Warn("权限码 %s 未找到，跳过", permCode)
			continue
		}
		if existingPermIDs[permID] {
			continue
		}
		rolePermission := model.RolePermission{
			RoleID:       roleID,
			PermissionID: parseUUID(permID),
		}
		tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rolePermission)
	}
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
