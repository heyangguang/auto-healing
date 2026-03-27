package audit

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func (r *PlatformAuditLogRepository) GetUserRanking(ctx context.Context, limit int, days int) ([]UserRanking, error) {
	var rankings []UserRanking
	query := r.db.WithContext(ctx).Model(&model.PlatformAuditLog{}).
		Select("user_id, username, count(*) as count")
	query = applyDaysFilter(query, days)
	err := query.
		Where("user_id IS NOT NULL").
		Group("user_id, username").
		Order("count DESC").
		Limit(limit).
		Scan(&rankings).Error
	return rankings, err
}

func (r *PlatformAuditLogRepository) GetUserLoginHistory(ctx context.Context, userID uuid.UUID, limit int) ([]model.PlatformAuditLog, error) {
	if limit <= 0 {
		limit = 10
	}
	var logs []model.PlatformAuditLog
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND category = ?", userID, "login").
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *PlatformAuditLogRepository) GetUserActivities(ctx context.Context, userID uuid.UUID, limit int) ([]model.PlatformAuditLog, error) {
	if limit <= 0 {
		limit = 15
	}
	var logs []model.PlatformAuditLog
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND category = ?", userID, "operation").
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
