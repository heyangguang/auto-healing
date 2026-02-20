package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserActivityRepository 用户活动数据仓库（收藏 + 最近访问）
type UserActivityRepository struct {
	db *gorm.DB
}

// NewUserActivityRepository 创建用户活动仓库
func NewUserActivityRepository() *UserActivityRepository {
	return &UserActivityRepository{db: database.DB}
}

// ==================== 收藏 ====================

// ListFavorites 获取用户收藏列表（按创建时间倒序）
func (r *UserActivityRepository) ListFavorites(ctx context.Context, userID uuid.UUID) ([]model.UserFavorite, error) {
	var favorites []model.UserFavorite
	err := TenantDB(r.db, ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&favorites).Error
	return favorites, err
}

// AddFavorite 添加收藏（使用 ON CONFLICT DO NOTHING 避免先查后写竞态）
func (r *UserActivityRepository) AddFavorite(ctx context.Context, fav *model.UserFavorite) error {
	// 设置租户 ID
	tenantID := TenantIDFromContext(ctx)
	fav.TenantID = &tenantID

	// idx_user_favorite 是 (user_id, menu_key, tenant_id) 的联合唯一约束
	// ON CONFLICT DO NOTHING 原子性地处理重复添加，彻底消除先 COUNT 后 CREATE 竞态
	result := TenantDB(r.db, ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(fav)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("该菜单项已收藏")
	}
	return nil
}

// RemoveFavorite 取消收藏
func (r *UserActivityRepository) RemoveFavorite(ctx context.Context, userID uuid.UUID, menuKey string) error {
	result := TenantDB(r.db, ctx).
		Where("user_id = ? AND menu_key = ?", userID, menuKey).
		Delete(&model.UserFavorite{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("收藏记录不存在")
	}
	return nil
}

// ==================== 最近访问 ====================

// ListRecents 获取最近访问列表（按访问时间倒序，最多 10 条）
func (r *UserActivityRepository) ListRecents(ctx context.Context, userID uuid.UUID) ([]model.UserRecent, error) {
	var recents []model.UserRecent
	err := TenantDB(r.db, ctx).
		Where("user_id = ?", userID).
		Order("accessed_at DESC").
		Limit(10).
		Find(&recents).Error
	return recents, err
}

// UpsertRecent 记录最近访问（已存在则更新访问时间，超过 10 条淘汰最旧的）
func (r *UserActivityRepository) UpsertRecent(ctx context.Context, recent *model.UserRecent) error {
	// 设置租户 ID
	tenantID := TenantIDFromContext(ctx)
	recent.TenantID = &tenantID

	return TenantDB(r.db, ctx).Transaction(func(tx *gorm.DB) error {
		// 尝试查找已有记录
		var existing model.UserRecent
		err := tx.Where("user_id = ? AND menu_key = ?", recent.UserID, recent.MenuKey).First(&existing).Error

		if err == nil {
			// 已存在 → 更新访问时间和名称/路径
			return tx.Model(&existing).Updates(map[string]interface{}{
				"accessed_at": time.Now(),
				"name":        recent.Name,
				"path":        recent.Path,
			}).Error
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 不存在 → 插入新记录
		recent.AccessedAt = time.Now()
		if err := tx.Create(recent).Error; err != nil {
			return err
		}

		// 淘汰超过 10 条的旧记录
		var count int64
		tx.Model(&model.UserRecent{}).Where("user_id = ?", recent.UserID).Count(&count)
		if count > 10 {
			// 找到第 10 条的访问时间，删除更旧的
			var cutoff model.UserRecent
			tx.Where("user_id = ?", recent.UserID).
				Order("accessed_at DESC").
				Offset(10).
				Limit(1).
				First(&cutoff)

			tx.Where("user_id = ? AND accessed_at <= ?", recent.UserID, cutoff.AccessedAt).
				Delete(&model.UserRecent{})
		}

		return nil
	})
}
