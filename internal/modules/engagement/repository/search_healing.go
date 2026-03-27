package repository

import (
	"context"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"gorm.io/gorm"
)

func (r *SearchRepository) searchRules(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &projection.HealingRule{}, "name ILIKE ? OR description ILIKE ?", like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []projection.HealingRule
	err = db.Model(&projection.HealingRule{}).
		Select("id, name, description, is_active").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/healing/rules",
			Extra:       map[string]any{"is_active": item.IsActive},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchFlows(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &projection.HealingFlow{}, "name ILIKE ? OR description ILIKE ?", like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []projection.HealingFlow
	err = db.Model(&projection.HealingFlow{}).
		Select("id, name, description, is_active").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/healing/flows",
			Extra:       map[string]any{"is_active": item.IsActive},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchInstances(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &projection.FlowInstance{}, "flow_name ILIKE ? OR error_message ILIKE ?", like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []projection.FlowInstance
	err = db.Model(&projection.FlowInstance{}).
		Select("id, flow_name, status, created_at").
		Where("flow_name ILIKE ? OR error_message ILIKE ?", like, like).
		Order("created_at DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.FlowName,
			Description: item.Status,
			Path:        "/healing/instances",
			Extra: map[string]any{
				"status":     item.Status,
				"created_at": item.CreatedAt,
			},
		})
	}
	return results, total, nil
}
