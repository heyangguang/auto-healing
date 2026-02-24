package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SearchRepository 全局搜索仓库
type SearchRepository struct {
	db *gorm.DB
}

// NewSearchRepository 创建全局搜索仓库
func NewSearchRepository() *SearchRepository {
	return &SearchRepository{
		db: database.DB,
	}
}

// SearchResultItem 搜索结果项
type SearchResultItem struct {
	ID          uuid.UUID              `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Path        string                 `json:"path"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// SearchResultCategory 搜索结果分类
type SearchResultCategory struct {
	Category      string             `json:"category"`
	CategoryLabel string             `json:"category_label"`
	Items         []SearchResultItem `json:"items"`
	Total         int64              `json:"total"`
}

// searchCategoryDef 搜索分类定义
type searchCategoryDef struct {
	category      string
	categoryLabel string
	searchFn      func(ctx context.Context, db *gorm.DB, keyword string, limit int) ([]SearchResultItem, int64, error)
}

// GlobalSearch 全局搜索
func (r *SearchRepository) GlobalSearch(ctx context.Context, keyword string, limit int) ([]SearchResultCategory, int64, error) {
	// 每次查询使用新的 TenantDB 实例，避免并发 GORM session 共享导致的竞态和 WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	like := "%" + keyword + "%"

	// 定义所有搜索分类
	categories := []searchCategoryDef{
		{"hosts", "主机资产", r.searchHosts},
		{"incidents", "工单", r.searchIncidents},
		{"rules", "自愈规则", r.searchRules},
		{"flows", "自愈流程", r.searchFlows},
		{"instances", "自愈实例", r.searchInstances},
		{"playbooks", "剧本", r.searchPlaybooks},
		{"templates", "任务模板", r.searchTemplates},
		{"schedules", "定时任务", r.searchSchedules},
		{"execution_runs", "执行记录", r.searchExecutionRuns},
		{"git_repos", "Git 仓库", r.searchGitRepos},
		{"secrets", "密钥", r.searchSecrets},
		{"plugins", "插件", r.searchPlugins},
		{"notification_templates", "通知模板", r.searchNotificationTemplates},
		{"notification_channels", "通知渠道", r.searchNotificationChannels},
	}

	// 并发搜索所有分类
	results := make([]SearchResultCategory, len(categories))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	var totalCount int64

	for i, cat := range categories {
		wg.Add(1)
		go func(idx int, def searchCategoryDef) {
			defer wg.Done()
			items, total, err := def.searchFn(ctx, newDB(), like, limit)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("search %s failed: %w", def.category, err)
				}
				return
			}

			if total > 0 {
				results[idx] = SearchResultCategory{
					Category:      def.category,
					CategoryLabel: def.categoryLabel,
					Items:         items,
					Total:         total,
				}
				totalCount += total
			}
		}(i, cat)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, 0, firstErr
	}

	// 过滤掉空分类
	filtered := make([]SearchResultCategory, 0)
	for _, r := range results {
		if r.Total > 0 {
			filtered = append(filtered, r)
		}
	}

	return filtered, totalCount, nil
}

// ==================== 各分类搜索实现 ====================

func (r *SearchRepository) searchHosts(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.CMDBItem{}).
		Where("hostname ILIKE ? OR ip_address ILIKE ? OR name ILIKE ?", like, like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.CMDBItem
	err := db.Model(&model.CMDBItem{}).
		Select("id, hostname, ip_address, name, status").
		Where("hostname ILIKE ? OR ip_address ILIKE ? OR name ILIKE ?", like, like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra:       map[string]interface{}{"status": item.Status},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchRules(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.HealingRule{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.HealingRule
	err := db.Model(&model.HealingRule{}).
		Select("id, name, description, is_active").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra:       map[string]interface{}{"is_active": item.IsActive},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchFlows(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.HealingFlow{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.HealingFlow
	err := db.Model(&model.HealingFlow{}).
		Select("id, name, description, is_active").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra:       map[string]interface{}{"is_active": item.IsActive},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchInstances(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.FlowInstance{}).
		Where("flow_name ILIKE ? OR error_message ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.FlowInstance
	err := db.Model(&model.FlowInstance{}).
		Select("id, flow_name, status, created_at").
		Where("flow_name ILIKE ? OR error_message ILIKE ?", like, like).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error
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
			Extra: map[string]interface{}{
				"status":     item.Status,
				"created_at": item.CreatedAt,
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchPlaybooks(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.Playbook{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Playbook
	err := db.Model(&model.Playbook{}).
		Select("id, name, description, status").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/execution/playbooks",
			Extra:       map[string]interface{}{"status": item.Status},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchTemplates(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.ExecutionTask{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.ExecutionTask
	err := db.Model(&model.ExecutionTask{}).
		Select("id, name, description").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/execution/templates",
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchSchedules(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.ExecutionSchedule{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.ExecutionSchedule
	err := db.Model(&model.ExecutionSchedule{}).
		Select("id, name, enabled, schedule_expr, description").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		extra := map[string]interface{}{
			"is_enabled": item.Enabled,
		}
		if item.ScheduleExpr != nil {
			extra["cron_expression"] = *item.ScheduleExpr
		}
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/execution/schedules",
			Extra:       extra,
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchSecrets(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.SecretsSource{}).
		Where("name ILIKE ?", like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.SecretsSource
	err := db.Model(&model.SecretsSource{}).
		Select("id, name, type").
		Where("name ILIKE ?", like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra:       map[string]interface{}{"type": item.Type},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchPlugins(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.Plugin{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Plugin
	err := db.Model(&model.Plugin{}).
		Select("id, name, description, type, status").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra: map[string]interface{}{
				"type":       item.Type,
				"is_enabled": item.Status == "active",
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchNotificationTemplates(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.NotificationTemplate{}).
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.NotificationTemplate
	err := db.Model(&model.NotificationTemplate{}).
		Select("id, name, description, event_type").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra:       map[string]interface{}{"type": item.EventType},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchNotificationChannels(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.NotificationChannel{}).
		Where("name ILIKE ?", like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.NotificationChannel
	err := db.Model(&model.NotificationChannel{}).
		Select("id, name, type, is_active").
		Where("name ILIKE ?", like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra: map[string]interface{}{
				"type":       item.Type,
				"is_enabled": item.IsActive,
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchIncidents(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.Incident{}).
		Where("title ILIKE ? OR external_id ILIKE ? OR description ILIKE ?", like, like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Incident
	err := db.Model(&model.Incident{}).
		Select("id, title, description, external_id, severity, status, healing_status").
		Where("title ILIKE ? OR external_id ILIKE ? OR description ILIKE ?", like, like, like).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		title := item.Title
		if title == "" {
			title = item.Description
		}
		if title == "" {
			title = item.ExternalID
		}
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       title,
			Description: item.ExternalID,
			Path:        "/resources/incidents",
			Extra: map[string]interface{}{
				"severity":       item.Severity,
				"status":         item.Status,
				"healing_status": item.HealingStatus,
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchExecutionRuns(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.ExecutionRun{}).
		Where("triggered_by ILIKE ? OR status ILIKE ? OR id::text ILIKE ?", like, like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.ExecutionRun
	err := db.Model(&model.ExecutionRun{}).
		Select("id, task_id, status, triggered_by, created_at").
		Where("triggered_by ILIKE ? OR status ILIKE ? OR id::text ILIKE ?", like, like, like).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		title := item.TriggeredBy
		if title == "" {
			title = item.ID.String()[:8]
		}
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       title,
			Description: item.Status,
			Path:        fmt.Sprintf("/execution/runs/%s", item.ID.String()),
			Extra: map[string]interface{}{
				"status":     item.Status,
				"created_at": item.CreatedAt,
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchGitRepos(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.GitRepository{}).
		Where("name ILIKE ? OR url ILIKE ?", like, like).
		Count(&total)

	if total == 0 {
		return nil, 0, nil
	}

	var items []model.GitRepository
	err := db.Model(&model.GitRepository{}).
		Select("id, name, url, status, default_branch").
		Where("name ILIKE ? OR url ILIKE ?", like, like).
		Order("name").
		Limit(limit).
		Find(&items).Error
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
			Extra: map[string]interface{}{
				"status":         item.Status,
				"default_branch": item.DefaultBranch,
			},
		})
	}
	return results, total, nil
}
