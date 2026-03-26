package repository

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"gorm.io/gorm"
)

// List 分页查询审计日志
func (r *AuditLogRepository) List(ctx context.Context, opts *AuditLogListOptions) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog

	queryBuilder := r.applyAuditLogFilters(r.tenantDB(ctx).Model(&model.AuditLog{}), opts)
	total, err := countWithSession(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	err = queryBuilder.
		Order(auditLogOrderClause(opts)).
		Offset(offset).
		Limit(opts.PageSize).
		Preload("User").
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *AuditLogRepository) applyAuditLogFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	db = r.applyAuditCategoryFilters(db, opts)
	db = r.applyAuditActorFilters(db, opts)
	db = r.applyAuditTimeFilters(db, opts)
	db = r.applyAuditRiskFilter(db, opts.RiskLevel)

	if !opts.Search.IsEmpty() {
		db = query.ApplyMultiStringFilter(db, []string{"username", "resource_name", "request_path"}, opts.Search)
	}
	return db
}

func (r *AuditLogRepository) applyAuditCategoryFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	if opts.Category != "" {
		db = db.Where("category = ?", opts.Category)
	}
	if opts.Action != "" {
		db = db.Where("action = ?", opts.Action)
	}
	if opts.ResourceType != "" {
		db = db.Where("resource_type = ?", opts.ResourceType)
	}
	if len(opts.ExcludeActions) > 0 {
		db = db.Where("action NOT IN ?", opts.ExcludeActions)
	}
	if len(opts.ExcludeResourceTypes) > 0 {
		db = db.Where("resource_type NOT IN ?", opts.ExcludeResourceTypes)
	}
	if opts.Status != "" {
		db = db.Where("status = ?", opts.Status)
	}
	return db
}

func (r *AuditLogRepository) applyAuditActorFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	if !opts.Username.IsEmpty() {
		db = query.ApplyStringFilter(db, "username", opts.Username)
	}
	if opts.UserID != nil {
		db = db.Where("user_id = ?", *opts.UserID)
	}
	if !opts.RequestPath.IsEmpty() {
		db = query.ApplyStringFilter(db, "request_path", opts.RequestPath)
	}
	return db
}

func (r *AuditLogRepository) applyAuditTimeFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	if opts.CreatedAfter != nil {
		db = db.Where("created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		db = db.Where("created_at <= ?", *opts.CreatedBefore)
	}
	return db
}

func (r *AuditLogRepository) applyAuditRiskFilter(db *gorm.DB, riskLevel string) *gorm.DB {
	switch riskLevel {
	case "high":
		return db.Where(buildHighRiskCondition())
	case "normal":
		return db.Where(fmt.Sprintf("NOT (%s)", buildHighRiskCondition()))
	default:
		return db
	}
}

func auditLogOrderClause(opts *AuditLogListOptions) string {
	sortBy := "created_at"
	allowedSortFields := map[string]bool{
		"created_at":    true,
		"action":        true,
		"resource_type": true,
		"username":      true,
		"status":        true,
	}
	if allowedSortFields[opts.SortBy] {
		sortBy = opts.SortBy
	}

	sortOrder := "DESC"
	if opts.SortOrder == "asc" || opts.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	return fmt.Sprintf("%s %s", sortBy, sortOrder)
}
