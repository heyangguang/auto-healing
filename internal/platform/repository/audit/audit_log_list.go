package audit

import (
	"context"

	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"gorm.io/gorm"
)

func (r *AuditLogRepository) List(ctx context.Context, opts *AuditLogListOptions) ([]platformmodel.AuditLog, int64, error) {
	if opts != nil && opts.Category == "login" {
		return tenantVisibleLoginList(r.db, ctx, opts)
	}
	queryBuilder := applyAuditLogFilters(tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}), opts)
	total, err := countWithClone(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	var logs []platformmodel.AuditLog
	offset := (opts.Page - 1) * opts.PageSize
	err = queryBuilder.Order(auditLogOrderClause(opts)).
		Offset(offset).
		Limit(opts.PageSize).
		Preload("User").
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func applyAuditLogFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	db = applyAuditCategoryFilters(db, opts)
	db = applyAuditActorFilters(db, opts)
	db = applyAuditTimeFilters(db, opts)
	db = applyAuditRiskFilter(db, opts.RiskLevel)
	if !opts.Search.IsEmpty() {
		db = query.ApplyMultiStringFilter(db, []string{"username", "resource_name", "request_path"}, opts.Search)
	}
	return db
}

func applyAuditCategoryFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
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

func applyAuditActorFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
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

func applyAuditTimeFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	if opts.CreatedAfter != nil {
		db = db.Where("created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		db = db.Where("created_at <= ?", *opts.CreatedBefore)
	}
	return db
}

func applyAuditRiskFilter(db *gorm.DB, riskLevel string) *gorm.DB {
	switch riskLevel {
	case "high":
		return db.Where(buildHighRiskCondition())
	case "normal":
		return db.Where("NOT (" + buildHighRiskCondition() + ")")
	default:
		return db
	}
}

func auditLogOrderClause(opts *AuditLogListOptions) string {
	return orderClause(opts.SortBy, opts.SortOrder, map[string]bool{
		"created_at":    true,
		"action":        true,
		"resource_type": true,
		"username":      true,
		"status":        true,
	})
}
