package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/ops/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 平台级设置 Repository ====================
// 通用 KV 设置存储，平台级配置（与租户无关）

// PlatformSettingsRepository 平台级设置仓库
type PlatformSettingsRepository struct {
	db *gorm.DB
}

// NewPlatformSettingsRepository 创建平台设置仓库
func NewPlatformSettingsRepository() *PlatformSettingsRepository {
	return &PlatformSettingsRepository{db: database.DB}
}

// GetAll 获取所有平台设置
func (r *PlatformSettingsRepository) GetAll(ctx context.Context) ([]model.PlatformSetting, error) {
	var settings []model.PlatformSetting
	err := r.db.WithContext(ctx).Order("module, key").Find(&settings).Error
	return settings, err
}

// GetByModule 按模块查询设置
func (r *PlatformSettingsRepository) GetByModule(ctx context.Context, module string) ([]model.PlatformSetting, error) {
	var settings []model.PlatformSetting
	err := r.db.WithContext(ctx).Where("module = ?", module).Order("key").Find(&settings).Error
	return settings, err
}

// GetByKey 获取单个设置
func (r *PlatformSettingsRepository) GetByKey(ctx context.Context, key string) (*model.PlatformSetting, error) {
	var setting model.PlatformSetting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// GetIntValue 获取 int 类型设置值（带默认值）
func (r *PlatformSettingsRepository) GetIntValue(ctx context.Context, key string, defaultVal int) int {
	setting, err := r.GetByKey(ctx, key)
	if err != nil {
		return getPlatformSettingDefault(key, defaultVal, err)
	}
	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		panic(fmt.Errorf("平台设置 %s 不是合法整数: %w", key, err))
	}
	return val
}

// GetStringValue 获取 string 类型设置值（带默认值）
func (r *PlatformSettingsRepository) GetStringValue(ctx context.Context, key string, defaultVal string) string {
	setting, err := r.GetByKey(ctx, key)
	if err != nil {
		return getPlatformSettingDefault(key, defaultVal, err)
	}
	return setting.Value
}

// GetBoolValue 获取 bool 类型设置值（带默认值）
func (r *PlatformSettingsRepository) GetBoolValue(ctx context.Context, key string, defaultVal bool) bool {
	setting, err := r.GetByKey(ctx, key)
	if err != nil {
		return getPlatformSettingDefault(key, defaultVal, err)
	}
	val, err := strconv.ParseBool(setting.Value)
	if err != nil {
		panic(fmt.Errorf("平台设置 %s 不是合法布尔值: %w", key, err))
	}
	return val
}

func getPlatformSettingDefault[T any](key string, defaultVal T, err error) T {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultVal
	}
	panic(fmt.Errorf("读取平台设置 %s 失败: %w", key, err))
}

// Update 更新设置值
func (r *PlatformSettingsRepository) Update(ctx context.Context, key string, value string, updatedBy *uuid.UUID) (*model.PlatformSetting, error) {
	var setting model.PlatformSetting
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}

	setting.Value = value
	setting.UpdatedAt = time.Now()
	setting.UpdatedBy = updatedBy

	if err := r.db.WithContext(ctx).Save(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}
