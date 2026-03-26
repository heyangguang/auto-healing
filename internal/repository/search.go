package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/company/auto-healing/internal/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SearchRepository 全局搜索仓库
type SearchRepository struct {
	db *gorm.DB
}

// NewSearchRepository 创建全局搜索仓库
func NewSearchRepository() *SearchRepository {
	return &SearchRepository{db: database.DB}
}

// SearchResultItem 搜索结果项
type SearchResultItem struct {
	ID          uuid.UUID      `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Path        string         `json:"path"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// SearchResultCategory 搜索结果分类
type SearchResultCategory struct {
	Category      string             `json:"category"`
	CategoryLabel string             `json:"category_label"`
	Items         []SearchResultItem `json:"items"`
	Total         int64              `json:"total"`
}

type searchCategoryDef struct {
	category      string
	categoryLabel string
	searchFn      func(ctx context.Context, db *gorm.DB, keyword string, limit int) ([]SearchResultItem, int64, error)
}

func (r *SearchRepository) tenantDB(ctx context.Context) *gorm.DB {
	return TenantDB(r.db, ctx)
}

// GlobalSearch 全局搜索
func (r *SearchRepository) GlobalSearch(ctx context.Context, keyword string, limit int, allowed map[string]bool) ([]SearchResultCategory, int64, error) {
	if _, ok := TenantIDFromContextOK(ctx); !ok {
		return nil, 0, fmt.Errorf("missing tenant context")
	}

	categories := filterSearchCategories(allSearchCategories(r), allowed)
	results, total, err := r.searchCategories(ctx, "%"+keyword+"%", limit, categories)
	if err != nil {
		return nil, 0, err
	}
	return compactSearchResults(results), total, nil
}

func allSearchCategories(r *SearchRepository) []searchCategoryDef {
	return []searchCategoryDef{
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
}

func filterSearchCategories(categories []searchCategoryDef, allowed map[string]bool) []searchCategoryDef {
	if allowed == nil {
		return categories
	}
	filtered := make([]searchCategoryDef, 0, len(categories))
	for _, category := range categories {
		if allowed[category.category] {
			filtered = append(filtered, category)
		}
	}
	return filtered
}

func (r *SearchRepository) searchCategories(ctx context.Context, like string, limit int, categories []searchCategoryDef) ([]SearchResultCategory, int64, error) {
	results := make([]SearchResultCategory, len(categories))
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		firstErr   error
		totalCount int64
	)

	for i, category := range categories {
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
			if total == 0 {
				return
			}

			results[idx] = SearchResultCategory{
				Category:      def.category,
				CategoryLabel: def.categoryLabel,
				Items:         items,
				Total:         total,
			}
			totalCount += total
		}(i, category)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, 0, firstErr
	}
	return results, totalCount, nil
}

func compactSearchResults(results []SearchResultCategory) []SearchResultCategory {
	filtered := make([]SearchResultCategory, 0, len(results))
	for _, result := range results {
		if result.Total > 0 {
			filtered = append(filtered, result)
		}
	}
	return filtered
}
