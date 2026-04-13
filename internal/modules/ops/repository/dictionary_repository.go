package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/ops/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DictionaryRepository 字典值仓储
type DictionaryRepository struct {
	db *gorm.DB
}

func NewDictionaryRepositoryWithDB(db *gorm.DB) *DictionaryRepository {
	return &DictionaryRepository{db: db}
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
	return r.db.WithContext(ctx).Table((model.Dictionary{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dict_type"}, {Name: "dict_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"label", "label_en", "color", "tag_color", "badge", "icon", "bg", "extra", "sort_order", "is_system", "is_active", "updated_at"}),
	}).CreateInBatches(buildDictionaryUpsertPayloads(items), 100).Error
}

func buildDictionaryUpsertPayloads(items []model.Dictionary) []map[string]interface{} {
	payloads := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		normalized := normalizeDictionarySeed(item)
		payloads = append(payloads, map[string]interface{}{
			"id":         normalized.ID,
			"dict_type":  normalized.DictType,
			"dict_key":   normalized.DictKey,
			"label":      normalized.Label,
			"label_en":   normalized.LabelEn,
			"color":      normalized.Color,
			"tag_color":  normalized.TagColor,
			"badge":      normalized.Badge,
			"icon":       normalized.Icon,
			"bg":         normalized.Bg,
			"extra":      normalized.Extra,
			"sort_order": normalized.SortOrder,
			"is_system":  normalized.IsSystem,
			"is_active":  normalized.IsActive,
			"created_at": normalized.CreatedAt,
			"updated_at": normalized.UpdatedAt,
		})
	}
	return payloads
}

func normalizeDictionarySeed(item model.Dictionary) model.Dictionary {
	now := time.Now().UTC()
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	return item
}
