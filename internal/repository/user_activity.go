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
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	query = applyOptionalTenantFilter(query, ctx, "tenant_id")
	err := query.Order("created_at DESC").Find(&favorites).Error
	return favorites, err
}

// AddFavorite 添加收藏（使用 ON CONFLICT DO NOTHING 避免先查后写竞态）
func (r *UserActivityRepository) AddFavorite(ctx context.Context, fav *model.UserFavorite) error {
	// 设置租户 ID；平台级公共数据使用 NULL tenant_id。
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		fav.TenantID = &tenantID
	} else {
		fav.TenantID = nil
	}

	// idx_user_favorite 是 (user_id, menu_key) 的联合唯一约束
	// ON CONFLICT DO NOTHING 原子性地处理重复添加，彻底消除先 COUNT 后 CREATE 竞态
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(fav)
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
	query := r.db.WithContext(ctx).Where("user_id = ? AND menu_key = ?", userID, menuKey)
	result := applyOptionalTenantFilter(query, ctx, "tenant_id").Delete(&model.UserFavorite{})
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
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	query = applyOptionalTenantFilter(query, ctx, "tenant_id")
	err := query.Order("accessed_at DESC").Limit(10).Find(&recents).Error
	return recents, err
}

// UpsertRecent 记录最近访问（已存在则更新访问时间，超过 10 条淘汰最旧的）
func (r *UserActivityRepository) UpsertRecent(ctx context.Context, recent *model.UserRecent) error {
	// 设置租户 ID；平台级公共数据使用 NULL tenant_id。
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		recent.TenantID = &tenantID
	} else {
		recent.TenantID = nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 尝试查找已有记录
		var existing model.UserRecent
		query := tx.Where("user_id = ? AND menu_key = ?", recent.UserID, recent.MenuKey)
		query = applyOptionalTenantFilter(query, ctx, "tenant_id")
		err := query.First(&existing).Error

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
		countQuery := tx.Model(&model.UserRecent{}).Where("user_id = ?", recent.UserID)
		applyOptionalTenantFilter(countQuery, ctx, "tenant_id").Count(&count)
		if count > 10 {
			// 找到第 10 条的访问时间，删除更旧的
			var cutoff model.UserRecent
			cutoffQuery := tx.Where("user_id = ?", recent.UserID)
			applyOptionalTenantFilter(cutoffQuery, ctx, "tenant_id").
				Order("accessed_at DESC").
				Offset(10).
				Limit(1).
				First(&cutoff)

			deleteQuery := tx.Where("user_id = ? AND accessed_at <= ?", recent.UserID, cutoff.AccessedAt)
			applyOptionalTenantFilter(deleteQuery, ctx, "tenant_id").Delete(&model.UserRecent{})
		}

		return nil
	})
}

func applyOptionalTenantFilter(db *gorm.DB, ctx context.Context, column string) *gorm.DB {
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		return db.Where(column+" = ?", tenantID)
	}
	return db.Where(column + " IS NULL")
}
