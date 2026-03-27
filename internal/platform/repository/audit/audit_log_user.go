package audit

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func (r *AuditLogRepository) GetUserLoginHistory(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, limit int) ([]model.AuditLog, error) {
	if limit <= 0 {
		limit = 10
	}
	return r.auditLogsByUserCategory(ctx, userID, tenantID, "login", limit)
}

func (r *AuditLogRepository) GetUserActivities(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, limit int) ([]model.AuditLog, error) {
	if limit <= 0 {
		limit = 15
	}
	return r.auditLogsByUserCategory(ctx, userID, tenantID, "operation", limit)
}

func (r *AuditLogRepository) auditLogsByUserCategory(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, category string, limit int) ([]model.AuditLog, error) {
	var logs []model.AuditLog
	query := r.db.WithContext(ctx).Where("user_id = ? AND category = ?", userID, category)
	if tenantID != uuid.Nil {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}
