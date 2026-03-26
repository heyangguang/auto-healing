package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

// AuditStats 审计统计概览
type AuditStats struct {
	TotalCount    int64        `json:"total_count"`
	SuccessCount  int64        `json:"success_count"`
	FailedCount   int64        `json:"failed_count"`
	HighRiskCount int64        `json:"high_risk_count"`
	ActionStats   []ActionStat `json:"action_stats"`
	TodayCount    int64        `json:"today_count"`
	WeekCount     int64        `json:"week_count"`
}

// ActionStat 按操作分组统计
type ActionStat struct {
	Action string `json:"action" gorm:"column:action"`
	Count  int64  `json:"count" gorm:"column:count"`
}

// UserRanking 用户操作排行
type UserRanking struct {
	UserID   string `json:"user_id" gorm:"column:user_id"`
	Username string `json:"username" gorm:"column:username"`
	Count    int64  `json:"count" gorm:"column:count"`
}

// ActionGroupItem 操作分组明细
type ActionGroupItem struct {
	Action       string `json:"action" gorm:"column:action"`
	ResourceType string `json:"resource_type" gorm:"column:resource_type"`
	Username     string `json:"username" gorm:"column:username"`
	Count        int64  `json:"count" gorm:"column:count"`
}

// ResourceTypeGroupItem 资源类型分组
type ResourceTypeGroupItem struct {
	ResourceType string `json:"resource_type" gorm:"column:resource_type"`
	Count        int64  `json:"count" gorm:"column:count"`
}

// TrendItem 趋势数据
type TrendItem struct {
	Date  string `json:"date" gorm:"column:date"`
	Count int64  `json:"count" gorm:"column:count"`
}

// HighRiskLog 高危操作记录（带风险原因）
type HighRiskLog struct {
	model.AuditLog
	RiskReason string `json:"risk_reason" gorm:"-"`
}

// GetStats 获取审计统计概览
func (r *AuditLogRepository) GetStats(ctx context.Context) (*AuditStats, error) {
	stats := &AuditStats{}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	totalCount, err := auditCount(newDB().Model(&model.AuditLog{}))
	if err != nil {
		return nil, err
	}
	successCount, err := auditCount(newDB().Model(&model.AuditLog{}).Where("status = ?", "success"))
	if err != nil {
		return nil, err
	}
	failedCount, err := auditCount(newDB().Model(&model.AuditLog{}).Where("status = ?", "failed"))
	if err != nil {
		return nil, err
	}
	highRiskCount, err := auditCount(newDB().Model(&model.AuditLog{}).Where(buildHighRiskCondition()))
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

// GetUserRanking 获取用户操作排行榜
func (r *AuditLogRepository) GetUserRanking(ctx context.Context, limit int, days int) ([]UserRanking, error) {
	var rankings []UserRanking
	err := r.auditDaysFilter(r.tenantDB(ctx).Model(&model.AuditLog{}).Select("user_id, username, count(*) as count"), days).
		Where("user_id IS NOT NULL").
		Group("user_id, username").
		Order("count DESC").
		Limit(limit).
		Scan(&rankings).Error
	return rankings, err
}

// GetActionGrouping 按操作类型 + 用户分组统计
func (r *AuditLogRepository) GetActionGrouping(ctx context.Context, action string, days int) ([]ActionGroupItem, error) {
	var items []ActionGroupItem
	query := r.auditDaysFilter(r.tenantDB(ctx).Model(&model.AuditLog{}).Select("action, resource_type, username, count(*) as count"), days)
	if action != "" {
		query = query.Where("action = ?", action)
	}
	err := query.Group("action, resource_type, username").Order("count DESC").Scan(&items).Error
	return items, err
}

// GetResourceTypeStats 按资源类型统计
func (r *AuditLogRepository) GetResourceTypeStats(ctx context.Context, days int) ([]ResourceTypeGroupItem, error) {
	var items []ResourceTypeGroupItem
	err := r.auditDaysFilter(r.tenantDB(ctx).Model(&model.AuditLog{}).Select("resource_type, count(*) as count"), days).
		Group("resource_type").
		Order("count DESC").
		Scan(&items).Error
	return items, err
}

// GetTrend 获取操作趋势（按天分组）
func (r *AuditLogRepository) GetTrend(ctx context.Context, days int) ([]TrendItem, error) {
	var items []TrendItem
	since := time.Now().AddDate(0, 0, -days)
	err := r.tenantDB(ctx).Model(&model.AuditLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, count(*) as count").
		Where("created_at >= ?", since).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

// GetHighRiskLogs 获取高危操作日志
func (r *AuditLogRepository) GetHighRiskLogs(ctx context.Context, page, pageSize int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	queryBuilder := r.tenantDB(ctx).Model(&model.AuditLog{}).Where(buildHighRiskCondition())
	total, err := countWithSession(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

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
	err := r.tenantDB(ctx).Model(&model.AuditLog{}).
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

	todayCount, err := auditCount(r.tenantDB(ctx).Model(&model.AuditLog{}).Where("created_at >= ?", todayStart))
	if err != nil {
		return 0, 0, err
	}
	weekCount, err := auditCount(r.tenantDB(ctx).Model(&model.AuditLog{}).Where("created_at >= ?", weekStart))
	if err != nil {
		return 0, 0, err
	}
	return todayCount, weekCount, nil
}

func (r *AuditLogRepository) auditDaysFilter(db *gorm.DB, days int) *gorm.DB {
	if days <= 0 {
		return db
	}
	return db.Where("created_at >= ?", time.Now().AddDate(0, 0, -days))
}

func auditCount(db *gorm.DB) (int64, error) {
	var count int64
	return count, db.Count(&count).Error
}
