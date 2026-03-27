package audit

import (
	"context"
	"time"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"gorm.io/gorm"
)

func (r *PlatformAuditLogRepository) GetStats(ctx context.Context) (*PlatformAuditStats, error) {
	totalCount, loginCount, operCount, successCount, failedCount, err := r.platformAuditSummaryCounts(ctx)
	if err != nil {
		return nil, err
	}
	todayCount, weekCount, err := r.platformAuditPeriodCounts(ctx)
	if err != nil {
		return nil, err
	}
	actionStats, err := r.platformAuditActionStats(ctx)
	if err != nil {
		return nil, err
	}
	return &PlatformAuditStats{
		TotalCount:   totalCount,
		LoginCount:   loginCount,
		OperCount:    operCount,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		TodayCount:   todayCount,
		WeekCount:    weekCount,
		ActionStats:  actionStats,
	}, nil
}

func (r *PlatformAuditLogRepository) GetTrend(ctx context.Context, days int) ([]TrendItem, error) {
	var items []TrendItem
	since := time.Now().AddDate(0, 0, -days)
	err := r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, count(*) as count").
		Where("created_at >= ?", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

func (r *PlatformAuditLogRepository) GetHighRiskLogs(ctx context.Context, page, pageSize int) ([]platformmodel.PlatformAuditLog, int64, error) {
	queryBuilder := r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).Where(buildHighRiskCondition())
	total, err := countWithClone(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	var logs []platformmodel.PlatformAuditLog
	offset := (page - 1) * pageSize
	err = queryBuilder.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&logs).Error
	return logs, total, err
}

func (r *PlatformAuditLogRepository) GetResourceTypeStats(ctx context.Context, days int) ([]ResourceTypeGroupItem, error) {
	var items []ResourceTypeGroupItem
	err := applyDaysFilter(r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).Select("resource_type, count(*) as count"), days).
		Group("resource_type").
		Order("count DESC").
		Scan(&items).Error
	return items, err
}

func (r *PlatformAuditLogRepository) GetActionGrouping(ctx context.Context, action string, days int) ([]ActionGroupItem, error) {
	var items []ActionGroupItem
	query := applyDaysFilter(r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).Select("action, resource_type, username, count(*) as count"), days)
	if action != "" {
		query = query.Where("action = ?", action)
	}
	err := query.Group("action, resource_type, username").Order("count DESC").Scan(&items).Error
	return items, err
}

func (r *PlatformAuditLogRepository) platformAuditSummaryCounts(ctx context.Context) (int64, int64, int64, int64, int64, error) {
	newDB := func() *gorm.DB { return r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}) }
	totalCount, err := auditCount(newDB())
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	loginCount, err := auditCount(newDB().Where("category = ?", "login"))
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	operCount, err := auditCount(newDB().Where("category = ?", "operation"))
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	successCount, err := auditCount(newDB().Where("status = ?", "success"))
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	failedCount, err := auditCount(newDB().Where("status = ?", "failed"))
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return totalCount, loginCount, operCount, successCount, failedCount, nil
}

func (r *PlatformAuditLogRepository) platformAuditPeriodCounts(ctx context.Context) (int64, int64, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))

	todayCount, err := auditCount(r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).Where("created_at >= ?", todayStart))
	if err != nil {
		return 0, 0, err
	}
	weekCount, err := auditCount(r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).Where("created_at >= ?", weekStart))
	if err != nil {
		return 0, 0, err
	}
	return todayCount, weekCount, nil
}

func (r *PlatformAuditLogRepository) platformAuditActionStats(ctx context.Context) ([]ActionStat, error) {
	var actionStats []ActionStat
	err := r.db.WithContext(ctx).Model(&platformmodel.PlatformAuditLog{}).
		Select("action, count(*) as count").
		Group("action").
		Order("count DESC").
		Scan(&actionStats).Error
	return actionStats, err
}
