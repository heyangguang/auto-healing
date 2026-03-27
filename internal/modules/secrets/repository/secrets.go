package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/company/auto-healing/internal/database"
	rootmodel "github.com/company/auto-healing/internal/model"
	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func NewSecretsSourceRepositoryWithDB(db *gorm.DB) *SecretsSourceRepository {
	return &SecretsSourceRepository{db: db}
}

func (r *SecretsSourceRepository) withDB(db *gorm.DB) *SecretsSourceRepository {
	return &SecretsSourceRepository{db: db}
}

func (r *SecretsSourceRepository) Transaction(ctx context.Context, fn func(repo *SecretsSourceRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(r.withDB(tx))
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

// Create 创建密钥源
func (r *SecretsSourceRepository) Create(ctx context.Context, source *secretsmodel.SecretsSource) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	source.TenantID = &tenantID
	return r.db.WithContext(ctx).Create(source).Error
}

// GetByID 根据ID获取密钥源
func (r *SecretsSourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*secretsmodel.SecretsSource, error) {
	return r.getByID(ctx, id, false)
}

func (r *SecretsSourceRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*secretsmodel.SecretsSource, error) {
	return r.getByID(ctx, id, true)
}

func (r *SecretsSourceRepository) getByID(ctx context.Context, id uuid.UUID, forUpdate bool) (*secretsmodel.SecretsSource, error) {
	var source secretsmodel.SecretsSource
	query := TenantDB(r.db, ctx)
	if forUpdate && r.db.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Where("id = ?", id).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// GetByName 根据名称获取密钥源
func (r *SecretsSourceRepository) GetByName(ctx context.Context, name string) (*secretsmodel.SecretsSource, error) {
	var source secretsmodel.SecretsSource
	err := TenantDB(r.db, ctx).Where("name = ?", name).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// Update 更新密钥源
func (r *SecretsSourceRepository) Update(ctx context.Context, source *secretsmodel.SecretsSource) error {
	result := TenantDB(r.db, ctx).
		Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", source.ID).
		Updates(map[string]interface{}{
			"config":     source.Config,
			"is_default": source.IsDefault,
			"priority":   source.Priority,
			"status":     source.Status,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 删除密钥源
func (r *SecretsSourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := TenantDB(r.db, ctx).Delete(&secretsmodel.SecretsSource{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// List 获取密钥源列表
func (r *SecretsSourceRepository) List(ctx context.Context, sourceType, status string, isDefault *bool) ([]secretsmodel.SecretsSource, error) {
	var sources []secretsmodel.SecretsSource
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
func (r *SecretsSourceRepository) GetDefault(ctx context.Context) (*secretsmodel.SecretsSource, error) {
	var source secretsmodel.SecretsSource
	err := TenantDB(r.db, ctx).Where("is_default = ? AND status = ?", true, "active").First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有默认的，返回优先级最高的活跃源
		err = TenantDB(r.db, ctx).Where("status = ?", "active").Order("priority ASC, created_at ASC").First(&source).Error
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// SetDefault 设置默认密钥源
func (r *SecretsSourceRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.setDefaultWithDB(ctx, tx, id)
	})
}

func (r *SecretsSourceRepository) setDefaultWithDB(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		Update("is_default", false).Error; err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("is_default", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SecretsSourceRepository) EnsureActiveDefault(ctx context.Context) error {
	return r.ensureActiveDefaultWithDB(ctx, r.db)
}

func (r *SecretsSourceRepository) ensureActiveDefaultWithDB(ctx context.Context, db *gorm.DB) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}

	var count int64
	if err := db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
		Where("tenant_id = ? AND is_default = ? AND status = ?", tenantID, true, "active").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var candidate secretsmodel.SecretsSource
	err = db.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, "active").
		Order("priority ASC, created_at ASC").
		First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).
			Update("is_default", false).Error
	}
	if err != nil {
		return err
	}
	return r.setDefaultWithDB(ctx, db, candidate.ID)
}

// UpdateTestResult 更新测试结果
func (r *SecretsSourceRepository) UpdateTestResult(ctx context.Context, id uuid.UUID, success bool) error {
	result := TenantDB(r.db, ctx).Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_test_at":     gorm.Expr("NOW()"),
			"last_test_result": success,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus 更新状态
func (r *SecretsSourceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	result := TenantDB(r.db, ctx).Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateTestTime 更新测试时间
func (r *SecretsSourceRepository) UpdateTestTime(ctx context.Context, id uuid.UUID) error {
	result := TenantDB(r.db, ctx).Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", id).
		Update("last_test_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountTasksUsingSource 统计引用指定密钥源的任务模板数量
func (r *SecretsSourceRepository) CountTasksUsingSource(ctx context.Context, sourceID string) (int64, error) {
	var count int64
	err := countSecretsSourceUsage(TenantDB(r.db, ctx).Model(&rootmodel.ExecutionTask{}), r.db.Dialector.Name(), sourceID, &count)
	return count, err
}

// CountSchedulesUsingSource 统计引用指定密钥源的调度任务数量
func (r *SecretsSourceRepository) CountSchedulesUsingSource(ctx context.Context, sourceID string) (int64, error) {
	var count int64
	err := countSecretsSourceUsage(TenantDB(r.db, ctx).Model(&rootmodel.ExecutionSchedule{}), r.db.Dialector.Name(), sourceID, &count)
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
	if err := newDB().Model(&secretsmodel.SecretsSource{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	newDB().Model(&secretsmodel.SecretsSource{}).
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
	newDB().Model(&secretsmodel.SecretsSource{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&typeCounts)
	stats["by_type"] = typeCounts

	return stats, nil
}

func marshalSecretsSourceReference(sourceID string) ([]byte, error) {
	return json.Marshal([]string{sourceID})
}

func countSecretsSourceUsage(query *gorm.DB, dialectName, sourceID string, count *int64) error {
	switch dialectName {
	case "sqlite":
		return query.
			Where("EXISTS (SELECT 1 FROM json_each(secrets_source_ids) WHERE json_each.value = ?)", sourceID).
			Count(count).Error
	default:
		payload, err := marshalSecretsSourceReference(sourceID)
		if err != nil {
			return err
		}
		return query.Where("secrets_source_ids @> ?", payload).Count(count).Error
	}
}
