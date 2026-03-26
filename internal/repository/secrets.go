package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecretsSourceRepository 密钥源仓储
type SecretsSourceRepository struct {
	db *gorm.DB
}

// NewSecretsSourceRepository 创建密钥源仓储
func NewSecretsSourceRepository() *SecretsSourceRepository {
	return &SecretsSourceRepository{
		db: database.DB,
	}
}

// Create 创建密钥源
func (r *SecretsSourceRepository) Create(ctx context.Context, source *model.SecretsSource) error {
	if err := FillTenantID(ctx, &source.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(source).Error
}

// GetByID 根据ID获取密钥源
func (r *SecretsSourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.SecretsSource, error) {
	var source model.SecretsSource
	err := TenantDB(r.db, ctx).Where("id = ?", id).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// GetByName 根据名称获取密钥源
func (r *SecretsSourceRepository) GetByName(ctx context.Context, name string) (*model.SecretsSource, error) {
	var source model.SecretsSource
	err := TenantDB(r.db, ctx).Where("name = ?", name).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// Update 更新密钥源
func (r *SecretsSourceRepository) Update(ctx context.Context, source *model.SecretsSource) error {
	return UpdateTenantScopedModel(r.db, ctx, source.ID, source)
}

// Delete 删除密钥源
func (r *SecretsSourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Delete(&model.SecretsSource{}, id).Error
}

// List 获取密钥源列表
func (r *SecretsSourceRepository) List(ctx context.Context, sourceType, status string, isDefault *bool) ([]model.SecretsSource, error) {
	var sources []model.SecretsSource
	query := TenantDB(r.db, ctx)

	if sourceType != "" {
		query = query.Where("type = ?", sourceType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if isDefault != nil {
		query = query.Where("is_default = ?", *isDefault)
	}

	err := query.Order("priority ASC, created_at ASC").Find(&sources).Error
	return sources, err
}

// GetDefault 获取默认密钥源
func (r *SecretsSourceRepository) GetDefault(ctx context.Context) (*model.SecretsSource, error) {
	var source model.SecretsSource
	err := TenantDB(r.db, ctx).Where("is_default = ? AND status = ?", true, "active").First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有默认的，返回优先级最高的活跃源
		err = TenantDB(r.db, ctx).Where("status = ?", "active").Order("priority ASC").First(&source).Error
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// SetDefault 设置默认密钥源
func (r *SecretsSourceRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Transaction(func(tx *gorm.DB) error {
		// 先取消所有默认
		if err := tx.Model(&model.SecretsSource{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		// 设置新默认
		return tx.Model(&model.SecretsSource{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

// UpdateTestResult 更新测试结果
func (r *SecretsSourceRepository) UpdateTestResult(ctx context.Context, id uuid.UUID, success bool) error {
	return TenantDB(r.db, ctx).Model(&model.SecretsSource{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_test_at":     gorm.Expr("NOW()"),
			"last_test_result": success,
		}).Error
}

// UpdateStatus 更新状态
func (r *SecretsSourceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return TenantDB(r.db, ctx).Model(&model.SecretsSource{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateTestTime 更新测试时间
func (r *SecretsSourceRepository) UpdateTestTime(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&model.SecretsSource{}).
		Where("id = ?", id).
		Update("last_test_at", gorm.Expr("NOW()")).Error
}

// CountTasksUsingSource 统计引用指定密钥源的任务模板数量
func (r *SecretsSourceRepository) CountTasksUsingSource(ctx context.Context, sourceID string) (int64, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.ExecutionTask{}).
		Where("secrets_source_ids @> ?", `["`+sourceID+`"]`).
		Count(&count).Error
	return count, err
}

// CountSchedulesUsingSource 统计引用指定密钥源的调度任务数量
func (r *SecretsSourceRepository) CountSchedulesUsingSource(ctx context.Context, sourceID string) (int64, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.ExecutionSchedule{}).
		Where("secrets_source_ids @> ?", `["`+sourceID+`"]`).
		Count(&count).Error
	return count, err
}

// ==================== 统计 ====================

// GetStats 获取密钥源统计信息
func (r *SecretsSourceRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	if err := newDB().Model(&model.SecretsSource{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	newDB().Model(&model.SecretsSource{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	// 按类型统计
	type TypeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var typeCounts []TypeCount
	newDB().Model(&model.SecretsSource{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&typeCounts)
	stats["by_type"] = typeCounts

	return stats, nil
}
