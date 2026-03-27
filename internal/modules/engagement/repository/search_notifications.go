package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

func (r *SearchRepository) searchPlugins(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.Plugin{}, "name ILIKE ? OR description ILIKE ?", like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Plugin
	err = db.Model(&model.Plugin{}).
		Select("id, name, description, type, status").
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
			Path:        "/resources/plugins",
			Extra: map[string]any{
				"type":       item.Type,
				"is_enabled": item.Status == "active",
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchNotificationTemplates(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.NotificationTemplate{}, "name ILIKE ? OR description ILIKE ?", like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.NotificationTemplate
	err = db.Model(&model.NotificationTemplate{}).
		Select("id, name, description, event_type").
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
			Path:        "/notification/templates",
			Extra:       map[string]any{"type": item.EventType},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchNotificationChannels(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.NotificationChannel{}, "name ILIKE ?", like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.NotificationChannel
	err = db.Model(&model.NotificationChannel{}).
		Select("id, name, type, is_active").
		Where("name ILIKE ?", like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Type,
			Path:        "/notification/channels",
			Extra: map[string]any{
				"type":       item.Type,
				"is_enabled": item.IsActive,
			},
		})
	}
	return results, total, nil
}
