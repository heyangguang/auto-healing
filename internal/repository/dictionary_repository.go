package repository

import (
	"context"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DictionaryRepository 字典值仓储
type DictionaryRepository struct {
	db *gorm.DB
}

// NewDictionaryRepository 创建仓储
func NewDictionaryRepository() *DictionaryRepository {
	return &DictionaryRepository{db: database.DB}
}

// DictTypeInfo 字典类型信息
type DictTypeInfo struct {
	DictType string `json:"dict_type"`
	Count    int64  `json:"count"`
}

// ListByTypes 按类型查询字典（支持多类型筛选）
func (r *DictionaryRepository) ListByTypes(ctx context.Context, types []string, activeOnly bool) ([]model.Dictionary, error) {
	query := r.db.WithContext(ctx).Model(&model.Dictionary{})

	if len(types) > 0 {
		query = query.Where("dict_type IN ?", types)
	}
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var items []model.Dictionary
	err := query.Order("dict_type, sort_order, dict_key").Find(&items).Error
	return items, err
}

// ListTypes 查询所有字典类型及数量
func (r *DictionaryRepository) ListTypes(ctx context.Context) ([]DictTypeInfo, error) {
	var results []DictTypeInfo
	err := r.db.WithContext(ctx).Model(&model.Dictionary{}).
		Select("dict_type, COUNT(*) as count").
		Where("is_active = ?", true).
		Group("dict_type").
		Order("dict_type").
		Find(&results).Error
	return results, err
}

// Create 创建字典项
func (r *DictionaryRepository) Create(ctx context.Context, item *model.Dictionary) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// Update 更新字典项
func (r *DictionaryRepository) Update(ctx context.Context, item *model.Dictionary) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// Delete 删除字典项
func (r *DictionaryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Dictionary{}, "id = ?", id).Error
}

// GetByID 根据ID查询
func (r *DictionaryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Dictionary, error) {
	var item model.Dictionary
	err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpsertBatch 批量 Upsert（用于 Seed）
func (r *DictionaryRepository) UpsertBatch(ctx context.Context, items []model.Dictionary) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dict_type"}, {Name: "dict_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"label", "label_en", "color", "tag_color", "badge", "icon", "bg", "extra", "sort_order", "is_system", "updated_at"}),
	}).CreateInBatches(items, 100).Error
}
