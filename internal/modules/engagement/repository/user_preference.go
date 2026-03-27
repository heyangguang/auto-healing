package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPreferenceNotFound  = errors.New("偏好设置不存在")
	ErrPreferenceCorrupted = errors.New("偏好设置数据损坏")
)

// UserPreferenceRepository 用户偏好数据仓库
type UserPreferenceRepository struct {
	db *gorm.DB
}

// NewUserPreferenceRepository 创建用户偏好仓库
func NewUserPreferenceRepository() *UserPreferenceRepository {
	return &UserPreferenceRepository{db: database.DB}
}

func NewUserPreferenceRepositoryWithDB(db *gorm.DB) *UserPreferenceRepository {
	return &UserPreferenceRepository{db: db}
}

// GetByUserID 根据用户 ID 和租户获取偏好设置
func (r *UserPreferenceRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.UserPreference, error) {
	var pref model.UserPreference
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		query = query.Where("tenant_id = ?", tenantID)
	} else {
		query = query.Where("tenant_id IS NULL")
	}
	err := query.First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPreferenceNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err := validatePreferenceObject(pref.Preferences); err != nil {
		return nil, err
	}
	return &pref, err
}

// Upsert 创建或全量更新偏好设置（使用 ON CONFLICT DO UPDATE 保证原子性）
func (r *UserPreferenceRepository) Upsert(ctx context.Context, userID uuid.UUID, preferences json.RawMessage) (*model.UserPreference, error) {
	pref := model.UserPreference{
		ID:          uuid.New(),
		UserID:      userID,
		Preferences: preferences,
	}

	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		pref.TenantID = &tenantID
		err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "tenant_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"preferences", "updated_at"}),
		}).Create(&pref).Error
		if err != nil {
			return nil, err
		}
		return &pref, nil
	}

	query := r.db.WithContext(ctx).Model(&model.UserPreference{}).
		Where("user_id = ? AND tenant_id IS NULL", userID).
		Updates(map[string]any{"preferences": preferences, "updated_at": time.Now()})
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		if err := r.db.WithContext(ctx).Create(&pref).Error; err != nil {
			return nil, err
		}
	}
	return &pref, nil
}

// MergeUpdate 部分更新偏好设置（合并已有偏好）
func (r *UserPreferenceRepository) MergeUpdate(ctx context.Context, userID uuid.UUID, patch json.RawMessage) (*model.UserPreference, error) {
	var pref model.UserPreference
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if tenantID, ok := TenantIDFromContextOK(ctx); ok {
		query = query.Where("tenant_id = ?", tenantID)
	} else {
		query = query.Where("tenant_id IS NULL")
	}
	err := query.First(&pref).Error

	var existing map[string]interface{}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		existing = make(map[string]interface{})
	} else if err != nil {
		return nil, err
	} else {
		existing, err = validatePreferenceObject(pref.Preferences)
		if err != nil {
			return nil, err
		}
	}

	// 合并 patch 到 existing
	var patchMap map[string]interface{}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, errors.New("无效的偏好设置格式")
	}
	if patchMap == nil {
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

func validatePreferenceObject(raw json.RawMessage) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPreferenceCorrupted, err)
	}
	if data == nil {
		return nil, fmt.Errorf("%w: stored preferences is null", ErrPreferenceCorrupted)
	}
	return data, nil
}
