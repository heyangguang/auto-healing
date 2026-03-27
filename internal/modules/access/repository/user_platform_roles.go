package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *UserRepository) UpdatePlatformUserWithRole(ctx context.Context, user *model.User, roleID *uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		if roleID == nil {
			return nil
		}
		if err := clearPlatformRoles(tx, user.ID); err != nil {
			return err
		}
		if err := insertPlatformRole(tx, user.ID, *roleID); err != nil {
			return err
		}
		return setPlatformAdminFlag(tx, user.ID, true)
	})
}

// AssignRoles 分配角色给用户
func (r *UserRepository) AssignRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := clearPlatformRoles(tx, userID); err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if err := insertPlatformRole(tx, userID, roleID); err != nil {
				return err
			}
		}
		return setPlatformAdminFlag(tx, userID, len(roleIDs) > 0)
	})
}

func clearPlatformRoles(tx *gorm.DB, userID uuid.UUID) error {
	return tx.Where("user_id = ?", userID).Delete(&model.UserPlatformRole{}).Error
}

func insertPlatformRole(tx *gorm.DB, userID, roleID uuid.UUID) error {
	return tx.Table("user_platform_roles").Create(map[string]any{
		"user_id": userID,
		"role_id": roleID,
	}).Error
}

func setPlatformAdminFlag(tx *gorm.DB, userID uuid.UUID, enabled bool) error {
	return tx.Model(&model.User{}).
		Where("id = ?", userID).
		Update("is_platform_admin", enabled).Error
}
