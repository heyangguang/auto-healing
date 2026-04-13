package audit

import (
	"context"

	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const tenantVisibleLoginSelectClause = `
	NULL AS tenant_id,
	platform_audit_logs.id AS id,
	platform_audit_logs.user_id AS user_id,
	platform_audit_logs.username AS username,
	platform_audit_logs.ip_address AS ip_address,
	platform_audit_logs.user_agent AS user_agent,
	platform_audit_logs.category AS category,
	platform_audit_logs.action AS action,
	platform_audit_logs.resource_type AS resource_type,
	platform_audit_logs.resource_id AS resource_id,
	platform_audit_logs.resource_name AS resource_name,
	platform_audit_logs.request_method AS request_method,
	platform_audit_logs.request_path AS request_path,
	platform_audit_logs.request_body AS request_body,
	platform_audit_logs.response_status AS response_status,
	platform_audit_logs.changes AS changes,
	platform_audit_logs.status AS status,
	platform_audit_logs.error_message AS error_message,
	platform_audit_logs.created_at AS created_at
`

func tenantVisibleLoginBaseQuery(db *gorm.DB, ctx context.Context) (*gorm.DB, error) {
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	tenantMembers := db.Table("user_tenant_roles").
		Select("DISTINCT user_id").
		Where("tenant_id = ?", tenantID)
	query := db.WithContext(ctx).
		Table("platform_audit_logs").
		Joins("INNER JOIN (?) AS tenant_members ON tenant_members.user_id = platform_audit_logs.user_id", tenantMembers).
		Where("platform_audit_logs.category = ?", "login")
	return query, nil
}

func tenantVisibleLoginByID(db *gorm.DB, ctx context.Context, id uuid.UUID) (*platformmodel.AuditLog, error) {
	query, err := tenantVisibleLoginBaseQuery(db, ctx)
	if err != nil {
		return nil, err
	}

	var log platformmodel.AuditLog
	err = query.
		Select(tenantVisibleLoginSelectClause).
		Where("platform_audit_logs.id = ?", id).
		Take(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func tenantVisibleLoginList(db *gorm.DB, ctx context.Context, opts *AuditLogListOptions) ([]platformmodel.AuditLog, int64, error) {
	query, err := tenantVisibleLoginBaseQuery(db, ctx)
	if err != nil {
		return nil, 0, err
	}

	query = applyTenantVisibleLoginFilters(query, opts)
	total, err := countWithClone(query)
	if err != nil {
		return nil, 0, err
	}

	var logs []platformmodel.AuditLog
	err = query.
		Select(tenantVisibleLoginSelectClause).
		Order(auditLogOrderClause(opts)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func applyTenantVisibleLoginFilters(db *gorm.DB, opts *AuditLogListOptions) *gorm.DB {
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
			"platform_audit_logs.resource_name",
			"platform_audit_logs.request_path",
		}, opts.Search)
	}
	return db
}
