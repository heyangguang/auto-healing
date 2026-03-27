package audit

import (
	"context"
	"time"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"gorm.io/gorm"
)

func (r *AuditLogRepository) GetStats(ctx context.Context) (*AuditStats, error) {
	stats := &AuditStats{}
	newDB := func() *gorm.DB { return tenantDB(r.db, ctx) }

	totalCount, err := auditCount(newDB().Model(&platformmodel.AuditLog{}))
	if err != nil {
		return nil, err
	}
	successCount, err := auditCount(newDB().Model(&platformmodel.AuditLog{}).Where("status = ?", "success"))
	if err != nil {
		return nil, err
	}
	failedCount, err := auditCount(newDB().Model(&platformmodel.AuditLog{}).Where("status = ?", "failed"))
	if err != nil {
		return nil, err
	}
	highRiskCount, err := auditCount(newDB().Model(&platformmodel.AuditLog{}).Where(buildHighRiskCondition()))
	if err != nil {
		return nil, err
	}
	actionStats, err := r.auditActionStats(ctx)
	if err != nil {
		return nil, err
	}
	todayCount, weekCount, err := r.auditPeriodCounts(ctx)
	if err != nil {
		return nil, err
	}

	stats.TotalCount = totalCount
	stats.SuccessCount = successCount
	stats.FailedCount = failedCount
	stats.HighRiskCount = highRiskCount
	stats.ActionStats = actionStats
	stats.TodayCount = todayCount
	stats.WeekCount = weekCount
	return stats, nil
}

func (r *AuditLogRepository) GetUserRanking(ctx context.Context, limit int, days int) ([]UserRanking, error) {
	var rankings []UserRanking
	err := applyDaysFilter(tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).Select("user_id, username, count(*) as count"), days).
		Where("user_id IS NOT NULL").
		Group("user_id, username").
		Order("count DESC").
		Limit(limit).
		Scan(&rankings).Error
	return rankings, err
}

func (r *AuditLogRepository) GetActionGrouping(ctx context.Context, action string, days int) ([]ActionGroupItem, error) {
	var items []ActionGroupItem
	query := applyDaysFilter(tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).Select("action, resource_type, username, count(*) as count"), days)
	if action != "" {
		query = query.Where("action = ?", action)
	}
	err := query.Group("action, resource_type, username").Order("count DESC").Scan(&items).Error
	return items, err
}

func (r *AuditLogRepository) GetResourceTypeStats(ctx context.Context, days int) ([]ResourceTypeGroupItem, error) {
	var items []ResourceTypeGroupItem
	err := applyDaysFilter(tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).Select("resource_type, count(*) as count"), days).
		Group("resource_type").
		Order("count DESC").
		Scan(&items).Error
	return items, err
}

func (r *AuditLogRepository) GetTrend(ctx context.Context, days int) ([]TrendItem, error) {
	var items []TrendItem
	since := time.Now().AddDate(0, 0, -days)
	err := tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, count(*) as count").
		Where("created_at >= ?", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

func (r *AuditLogRepository) GetHighRiskLogs(ctx context.Context, page, pageSize int) ([]platformmodel.AuditLog, int64, error) {
	queryBuilder := tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).Where(buildHighRiskCondition())
	total, err := countWithClone(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	var logs []platformmodel.AuditLog
	offset := (page - 1) * pageSize
	err = queryBuilder.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Preload("User").
		Find(&logs).Error
	return logs, total, err
}

func (r *AuditLogRepository) auditActionStats(ctx context.Context) ([]ActionStat, error) {
	var stats []ActionStat
	err := tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).
		Select("action, count(*) as count").
		Group("action").
		Order("count DESC").
		Scan(&stats).Error
	return stats, err
}

func (r *AuditLogRepository) auditPeriodCounts(ctx context.Context) (int64, int64, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))

	todayCount, err := auditCount(tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).Where("created_at >= ?", todayStart))
	if err != nil {
		return 0, 0, err
	}
	weekCount, err := auditCount(tenantDB(r.db, ctx).Model(&platformmodel.AuditLog{}).Where("created_at >= ?", weekStart))
	if err != nil {
		return 0, 0, err
	}
	return todayCount, weekCount, nil
}
