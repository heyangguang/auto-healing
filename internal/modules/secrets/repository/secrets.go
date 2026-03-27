package repository

import (
	"context"
	"database/sql"

	"github.com/company/auto-healing/internal/database"
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
