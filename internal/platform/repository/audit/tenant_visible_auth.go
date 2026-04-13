package audit

import (
	"context"

	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *PlatformAuditLogRepository) ListTenantVisibleAuthLogs(ctx context.Context, opts *AuditLogListOptions) ([]platformmodel.PlatformAuditLog, int64, error) {
	query, err := tenantVisibleAuthBaseQuery(r.db, ctx, opts.Category)
	if err != nil {
		return nil, 0, err
	}
	query = applyTenantVisibleAuthFilters(query, opts)
	total, err := countWithClone(query)
	if err != nil {
		return nil, 0, err
	}

	var logs []platformmodel.PlatformAuditLog
	err = query.
		Order(auditLogOrderClause(opts)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *PlatformAuditLogRepository) GetTenantVisibleAuthLogByID(ctx context.Context, id uuid.UUID) (*platformmodel.PlatformAuditLog, error) {
	query, err := tenantVisibleAuthBaseQuery(r.db, ctx, authCategoryStored)
	if err != nil {
		return nil, err
	}

	var log platformmodel.PlatformAuditLog
	err = query.Where("platform_audit_logs.id = ?", id).Take(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *PlatformAuditLogRepository) GetTenantVisibleAuthStats(ctx context.Context, category string) (*AuditStats, error) {
	query, err := tenantVisibleAuthBaseQuery(r.db, ctx, category)
	if err != nil {
		return nil, err
	}
	totalCount, err := auditCount(query.Session(&gorm.Session{}))
	if err != nil {
		return nil, err
	}
	successCount, err := auditCount(query.Session(&gorm.Session{}).Where("platform_audit_logs.status = ?", "success"))
	if err != nil {
		return nil, err
	}
	failedCount, err := auditCount(query.Session(&gorm.Session{}).Where("platform_audit_logs.status = ?", "failed"))
	if err != nil {
		return nil, err
	}
	highRiskCount, err := auditCount(query.Session(&gorm.Session{}).Where(buildHighRiskCondition()))
	if err != nil {
		return nil, err
	}
	return &AuditStats{
		TotalCount:    totalCount,
		SuccessCount:  successCount,
		FailedCount:   failedCount,
		HighRiskCount: highRiskCount,
	}, nil
}

func (r *PlatformAuditLogRepository) GetTenantVisibleAuthTrend(ctx context.Context, category string, days int) ([]TrendItem, error) {
	query, err := tenantVisibleAuthBaseQuery(r.db, ctx, category)
	if err != nil {
		return nil, err
	}
	var items []TrendItem
	err = applyDaysFilter(query.Select("TO_CHAR(platform_audit_logs.created_at, 'YYYY-MM-DD') as date, count(*) as count"), days).
		Group("TO_CHAR(platform_audit_logs.created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

func tenantVisibleAuthBaseQuery(db *gorm.DB, ctx context.Context, category string) (*gorm.DB, error) {
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	tenantMembers := db.Table("user_tenant_roles").
		Select("DISTINCT user_id").
		Where("tenant_id = ?", tenantID)
	query := db.WithContext(ctx).
		Table("platform_audit_logs").
		Joins("INNER JOIN (?) AS tenant_members ON tenant_members.user_id = platform_audit_logs.user_id", tenantMembers)
	return applyPlatformAuditCategoryScope(query, category), nil
}

func applyTenantVisibleAuthFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
	if opts.Action != "" {
		db = db.Where("platform_audit_logs.action = ?", opts.Action)
	}
	if opts.ResourceType != "" {
		db = db.Where("platform_audit_logs.resource_type = ?", opts.ResourceType)
	}
	if len(opts.ExcludeActions) > 0 {
		db = db.Where("platform_audit_logs.action NOT IN ?", opts.ExcludeActions)
	}
	if len(opts.ExcludeResourceTypes) > 0 {
		db = db.Where("platform_audit_logs.resource_type NOT IN ?", opts.ExcludeResourceTypes)
	}
	if !opts.Username.IsEmpty() {
		db = query.ApplyStringFilter(db, "platform_audit_logs.username", opts.Username)
	}
	if opts.UserID != nil {
		db = db.Where("platform_audit_logs.user_id = ?", *opts.UserID)
	}
	if opts.Status != "" {
		db = db.Where("platform_audit_logs.status = ?", opts.Status)
	}
	if !opts.RequestPath.IsEmpty() {
		db = query.ApplyStringFilter(db, "platform_audit_logs.request_path", opts.RequestPath)
	}
	if opts.CreatedAfter != nil {
		db = db.Where("platform_audit_logs.created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		db = db.Where("platform_audit_logs.created_at <= ?", *opts.CreatedBefore)
	}
	if !opts.Search.IsEmpty() {
		db = query.ApplyMultiStringFilter(db, []string{
			"platform_audit_logs.username",
			"platform_audit_logs.principal_username",
			"platform_audit_logs.subject_tenant_name",
			"platform_audit_logs.request_path",
		}, opts.Search)
	}
	return db
}
