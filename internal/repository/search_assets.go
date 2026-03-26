package repository

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

func (r *SearchRepository) searchHosts(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.CMDBItem{}, "hostname ILIKE ? OR ip_address ILIKE ? OR name ILIKE ?", like, like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.CMDBItem
	err = db.Model(&model.CMDBItem{}).
		Select("id, hostname, ip_address, name, status").
		Where("hostname ILIKE ? OR ip_address ILIKE ? OR name ILIKE ?", like, like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		title := item.Hostname
		if title == "" {
			title = item.Name
		}
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       title,
			Description: item.IPAddress,
			Path:        "/resources/cmdb",
			Extra:       map[string]any{"status": item.Status},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchIncidents(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.Incident{}, "title ILIKE ? OR external_id ILIKE ? OR description ILIKE ?", like, like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Incident
	err = db.Model(&model.Incident{}).
		Select("id, title, description, external_id, severity, status, healing_status").
		Where("title ILIKE ? OR external_id ILIKE ? OR description ILIKE ?", like, like, like).
		Order("created_at DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       incidentTitle(item),
			Description: incidentDescription(item),
			Path:        "/resources/incidents",
			Extra: map[string]any{
				"severity":       item.Severity,
				"status":         item.Status,
				"healing_status": item.HealingStatus,
			},
		})
	}
	return results, total, nil
}

func incidentTitle(item model.Incident) string {
	if item.Title != "" {
		return item.Title
	}
	if item.Description != "" {
		return item.Description
	}
	severity := map[string]string{"critical": "紧急", "high": "高", "medium": "中", "low": "低"}[item.Severity]
	if severity == "" {
		severity = item.Severity
	}
	externalID := item.ExternalID
	if len(externalID) > 8 {
		externalID = externalID[:8]
	}
	return fmt.Sprintf("[%s] 工单 #%s", severity, externalID)
}

func incidentDescription(item model.Incident) string {
	if item.Description != "" {
		return item.Description
	}
	if len(item.ExternalID) > 12 {
		return item.ExternalID[:12] + "..."
	}
	return item.ExternalID
}

func (r *SearchRepository) searchSecrets(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.SecretsSource{}, "name ILIKE ?", like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.SecretsSource
	err = db.Model(&model.SecretsSource{}).
		Select("id, name, type").
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
			Path:        "/resources/secrets",
			Extra:       map[string]any{"type": item.Type},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchGitRepos(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	total, err := searchCount(db, &model.GitRepository{}, "name ILIKE ? OR url ILIKE ?", like, like)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.GitRepository
	err = db.Model(&model.GitRepository{}).
		Select("id, name, url, status, default_branch").
		Where("name ILIKE ? OR url ILIKE ?", like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.URL,
			Path:        "/execution/git-repos",
			Extra: map[string]any{
				"status":         item.Status,
				"default_branch": item.DefaultBranch,
			},
		})
	}
	return results, total, nil
}
