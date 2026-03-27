package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxFailedLoginAttempts = 5

// UpdateLoginInfo 更新登录信息（同时解锁账户、重置失败计数）
func (r *UserRepository) UpdateLoginInfo(ctx context.Context, userID uuid.UUID, ip string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"last_login_at":      gorm.Expr("NOW()"),
			"last_login_ip":      ip,
			"failed_login_count": 0,
			"status":             "active",
			"locked_until":       nil,
		}).Error
}

// IncrementFailedLogin 增加登录失败次数，达到阈值时自动锁定账户（永久锁定，需管理员解锁）
func (r *UserRepository) IncrementFailedLogin(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := incrementFailedLoginCount(tx, userID); err != nil {
			return err
		}
		count, err := failedLoginCount(tx, userID)
		if err != nil || count < maxFailedLoginAttempts {
			return err
		}
		return lockUserAccount(tx, userID)
	})
}

func incrementFailedLoginCount(tx *gorm.DB, userID uuid.UUID) error {
	return tx.Model(&model.User{}).
		Where("id = ?", userID).
		Update("failed_login_count", gorm.Expr("failed_login_count + 1")).Error
}

func failedLoginCount(tx *gorm.DB, userID uuid.UUID) (int, error) {
	var count int
	err := tx.Model(&model.User{}).
		Where("id = ?", userID).
		Select("failed_login_count").
		Scan(&count).Error
	return count, err
}

func lockUserAccount(tx *gorm.DB, userID uuid.UUID) error {
	return tx.Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"status":       "locked",
			"locked_until": nil,
		}).Error
}

// UpdatePassword 更新密码
func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"password_hash":       passwordHash,
			"password_changed_at": gorm.Expr("NOW()"),
		}).Error
}
