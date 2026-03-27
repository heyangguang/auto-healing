package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxRecentItems = 10

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
	err := query.Order("accessed_at DESC, id DESC").Limit(maxRecentItems).Find(&recents).Error
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
		existing, err := findExistingRecent(tx, ctx, recent.UserID, recent.MenuKey)
		if err == nil {
			return updateExistingRecent(tx, existing, recent)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return upsertRecentRecord(tx, ctx, recent)
	})
}

func findExistingRecent(tx *gorm.DB, ctx context.Context, userID uuid.UUID, menuKey string) (*model.UserRecent, error) {
	var existing model.UserRecent
	query := tx.Where("user_id = ? AND menu_key = ?", userID, menuKey)
	err := applyOptionalTenantFilter(query, ctx, "tenant_id").First(&existing).Error
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

func updateExistingRecent(tx *gorm.DB, existing, recent *model.UserRecent) error {
	accessedAt := time.Now()
	if err := tx.Model(existing).Updates(map[string]interface{}{
		"accessed_at": accessedAt,
		"name":        recent.Name,
		"path":        recent.Path,
	}).Error; err != nil {
		return err
	}
	recent.ID = existing.ID
	recent.AccessedAt = accessedAt
	recent.TenantID = existing.TenantID
	return nil
}

func upsertRecentRecord(tx *gorm.DB, ctx context.Context, recent *model.UserRecent) error {
	if recent.ID == uuid.Nil {
		recent.ID = uuid.New()
	}
	recent.AccessedAt = time.Now()
	if err := tx.Create(recent).Error; err != nil {
		if !isRecentDuplicateError(err) {
			return err
		}
		existing, findErr := findExistingRecent(tx, ctx, recent.UserID, recent.MenuKey)
		if findErr != nil {
			return findErr
		}
		return updateExistingRecent(tx, existing, recent)
	}
	return trimOverflowRecents(tx, ctx, recent.UserID)
}

func isRecentDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "UNIQUE constraint failed") || strings.Contains(message, "duplicate key value")
}

func trimOverflowRecents(tx *gorm.DB, ctx context.Context, userID uuid.UUID) error {
	count, err := countUserRecents(tx, ctx, userID)
	if err != nil || count <= maxRecentItems {
		return err
	}

	staleIDs, err := staleRecentIDs(tx, ctx, userID)
	if err != nil || len(staleIDs) == 0 {
		return err
	}

	return applyOptionalTenantFilter(tx.Where("id IN ?", staleIDs), ctx, "tenant_id").
		Delete(&model.UserRecent{}).Error
}

func countUserRecents(tx *gorm.DB, ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query := tx.Model(&model.UserRecent{}).Where("user_id = ?", userID)
	err := applyOptionalTenantFilter(query, ctx, "tenant_id").Count(&count).Error
	return count, err
}

func staleRecentIDs(tx *gorm.DB, ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var staleIDs []uuid.UUID
	query := tx.Model(&model.UserRecent{}).Where("user_id = ?", userID)
	err := applyOptionalTenantFilter(query, ctx, "tenant_id").
		Order("accessed_at DESC, id DESC").
		Offset(maxRecentItems).
		Pluck("id", &staleIDs).Error
	return staleIDs, err
}

func applyOptionalTenantFilter(db *gorm.DB, ctx context.Context, column string) *gorm.DB {
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		return db.Where(column+" = ?", tenantID)
	}
	return db.Where(column + " IS NULL")
}
