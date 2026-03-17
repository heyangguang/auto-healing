package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPreferenceNotFound = errors.New("偏好设置不存在")
)

// UserPreferenceRepository 用户偏好数据仓库
type UserPreferenceRepository struct {
	db *gorm.DB
}

// NewUserPreferenceRepository 创建用户偏好仓库
func NewUserPreferenceRepository() *UserPreferenceRepository {
	return &UserPreferenceRepository{db: database.DB}
}

// GetByUserID 根据用户 ID 和租户获取偏好设置
func (r *UserPreferenceRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.UserPreference, error) {
	var pref model.UserPreference
	tenantID := TenantIDFromContext(ctx)
	err := r.db.WithContext(ctx).Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPreferenceNotFound
	}
	return &pref, err
}

// Upsert 创建或全量更新偏好设置（使用 ON CONFLICT DO UPDATE 保证原子性）
func (r *UserPreferenceRepository) Upsert(ctx context.Context, userID uuid.UUID, preferences json.RawMessage) (*model.UserPreference, error) {
	tenantID := TenantIDFromContext(ctx)
	pref := model.UserPreference{
		UserID:      userID,
		TenantID:    &tenantID,
		Preferences: preferences,
	}

	// 唯一约束现在是 (user_id, tenant_id) 联合索引
	// 使用 ON CONFLICT(user_id, tenant_id) DO UPDATE SET preferences = EXCLUDED.preferences
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "tenant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"preferences", "updated_at"}),
	}).Create(&pref).Error

	if err != nil {
		return nil, err
	}

	return &pref, nil
}

// MergeUpdate 部分更新偏好设置（合并已有偏好）
func (r *UserPreferenceRepository) MergeUpdate(ctx context.Context, userID uuid.UUID, patch json.RawMessage) (*model.UserPreference, error) {
	var pref model.UserPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&pref).Error

	var existing map[string]interface{}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = make(map[string]interface{})
	} else if err != nil {
		return nil, err
	} else {
		if err := json.Unmarshal(pref.Preferences, &existing); err != nil {
			existing = make(map[string]interface{})
		}
	}

	// 合并 patch 到 existing
	var patchMap map[string]interface{}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, errors.New("无效的偏好设置格式")
	}
	for k, v := range patchMap {
		existing[k] = v
	}

	merged, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}

	return r.Upsert(ctx, userID, merged)
}
