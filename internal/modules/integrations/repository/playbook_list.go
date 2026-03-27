package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"gorm.io/gorm"
)

// ListWithOptions 列出 Playbooks（支持完整查询参数）
func (r *PlaybookRepository) ListWithOptions(ctx context.Context, opts *PlaybookListOptions) ([]model.Playbook, int64, error) {
	query := r.buildPlaybookListQuery(ctx, opts)
	total, err := countWithSession(query)
	if err != nil {
		return nil, 0, err
	}

	var playbooks []model.Playbook
	query = applyPlaybookSorting(query, opts)
	if opts.Page > 0 && opts.PageSize > 0 {
		query = query.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize)
	}
	err = query.Preload("Repository").Find(&playbooks).Error
	return playbooks, total, err
}

func (r *PlaybookRepository) buildPlaybookListQuery(ctx context.Context, opts *PlaybookListOptions) *gorm.DB {
	queryBuilder := r.tenantDB(ctx).Model(&model.Playbook{})

	if !opts.Name.IsEmpty() {
		queryBuilder = query.ApplyStringFilter(queryBuilder, "name", opts.Name)
	}
	if !opts.FilePath.IsEmpty() {
		queryBuilder = query.ApplyStringFilter(queryBuilder, "file_path", opts.FilePath)
	}
	if opts.RepositoryID != nil {
		queryBuilder = queryBuilder.Where("repository_id = ?", *opts.RepositoryID)
	}
	if opts.Status != "" {
		queryBuilder = queryBuilder.Where("status = ?", opts.Status)
	}
	if opts.ConfigMode != "" {
		queryBuilder = queryBuilder.Where("config_mode = ?", opts.ConfigMode)
	}
	queryBuilder = applyPlaybookVariableFilters(queryBuilder, opts)
	if opts.CreatedFrom != nil {
		queryBuilder = queryBuilder.Where("created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		queryBuilder = queryBuilder.Where("created_at <= ?", *opts.CreatedTo)
	}
	return queryBuilder
}

func applyPlaybookVariableFilters(queryBuilder *gorm.DB, opts *PlaybookListOptions) *gorm.DB {
	if opts.HasVariables != nil {
		if *opts.HasVariables {
			queryBuilder = queryBuilder.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) > 0")
		} else {
			queryBuilder = queryBuilder.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) = 0")
		}
	}
	if opts.MinVariables != nil {
		queryBuilder = queryBuilder.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) >= ?", *opts.MinVariables)
	}
	if opts.MaxVariables != nil {
		queryBuilder = queryBuilder.Where("jsonb_array_length(COALESCE(variables, '[]'::jsonb)) <= ?", *opts.MaxVariables)
	}
	if opts.HasRequiredVars != nil {
		if *opts.HasRequiredVars {
			queryBuilder = queryBuilder.Where("EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(variables, '[]'::jsonb)) AS v WHERE (v->>'required')::boolean = true)")
		} else {
			queryBuilder = queryBuilder.Where("NOT EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(variables, '[]'::jsonb)) AS v WHERE (v->>'required')::boolean = true)")
		}
	}
	return queryBuilder
}

func applyPlaybookSorting(queryBuilder *gorm.DB, opts *PlaybookListOptions) *gorm.DB {
	allowedSortFields := map[string]bool{
		"name": true, "status": true, "config_mode": true, "file_path": true,
		"created_at": true, "updated_at": true, "last_scanned_at": true,
	}
	if opts.SortField == "" || !allowedSortFields[opts.SortField] {
		return queryBuilder.Order("created_at DESC")
	}

	order := "ASC"
	if strings.ToLower(opts.SortOrder) == "desc" {
		order = "DESC"
	}
	return queryBuilder.Order(fmt.Sprintf("%s %s", opts.SortField, order))
}
