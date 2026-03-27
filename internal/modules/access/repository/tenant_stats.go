package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrTenantStatsTableNotAllowed = errors.New("tenant stats table is not allowed")

var tenantStatsAllowedTables = map[string]bool{
	"audit_logs":             true,
	"flow_instances":         true,
	"execution_tasks":        true,
	"execution_runs":         true,
	"execution_schedules":    true,
	"cmdb_items":             true,
	"git_repositories":       true,
	"playbooks":              true,
	"secrets_sources":        true,
	"plugins":                true,
	"incidents":              true,
	"healing_flows":          true,
	"healing_rules":          true,
	"notification_channels":  true,
	"notification_templates": true,
}

// CountTenantMembers 统计某租户的成员数
func (r *TenantRepository) CountTenantMembers(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UserTenantRole{}).
		Where("tenant_id = ?", tenantID).
		Distinct("user_id").
		Count(&count).Error
	return count, err
}

// CountTenantTable 统计某租户在指定表中的记录数（通用方法）
func (r *TenantRepository) CountTenantTable(ctx context.Context, tenantID uuid.UUID, tableName string) (int64, error) {
	tableName, ok := resolveTenantStatsTable(tableName)
	if !ok {
		return 0, ErrTenantStatsTableNotAllowed
	}
	var count int64
	err := r.db.WithContext(ctx).Table(tableName).Where("tenant_id = ?", tenantID).Count(&count).Error
	return count, err
}

func resolveTenantStatsTable(tableName string) (string, bool) {
	_, ok := tenantStatsAllowedTables[tableName]
	return tableName, ok
}

// GetTenantLastActivity 获取租户最近一条审计日志的时间
func (r *TenantRepository) GetTenantLastActivity(ctx context.Context, tenantID uuid.UUID) (*string, error) {
	var result sql.NullString
	err := r.db.WithContext(ctx).
		Table("audit_logs").
		Select("to_char(MAX(created_at), 'YYYY-MM-DD HH24:MI:SS')").
		Where("tenant_id = ?", tenantID).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	if !result.Valid {
		return nil, nil
	}
	return &result.String, nil
}

// CountTenantTableWhere 统计某租户在指定表中满足额外条件的记录数
func (r *TenantRepository) CountTenantTableWhere(ctx context.Context, tenantID uuid.UUID, tableName string, extraWhere string) (int64, error) {
	tableName, ok := resolveTenantStatsTable(tableName)
	if !ok {
		return 0, ErrTenantStatsTableNotAllowed
	}
	var count int64
	query := r.db.WithContext(ctx).Table(tableName).Where("tenant_id = ?", tenantID)
	query = applyTenantStatsExtraFilter(query, extraWhere)
	err := query.Count(&count).Error
	return count, err
}

type trendRow struct {
	Date  string `gorm:"column:date"`
	Count int64  `gorm:"column:cnt"`
}

// GetTrendByDay 按天统计某张表最近 N 天的记录数（跨所有租户）
func (r *TenantRepository) GetTrendByDay(ctx context.Context, tableName string, days int) ([]string, []int64, error) {
	return r.getTrendByDay(ctx, tableName, days, "")
}

// GetTrendByDayWhere 带条件按天统计
func (r *TenantRepository) GetTrendByDayWhere(ctx context.Context, tableName string, days int, extraWhere string) ([]string, []int64, error) {
	return r.getTrendByDay(ctx, tableName, days, extraWhere)
}

func (r *TenantRepository) getTrendByDay(ctx context.Context, tableName string, days int, extraWhere string) ([]string, []int64, error) {
	tableName, ok := resolveTenantStatsTable(tableName)
	if !ok {
		return nil, nil, ErrTenantStatsTableNotAllowed
	}
	var rows []trendRow
	since := time.Now().AddDate(0, 0, -days)
	query := r.db.WithContext(ctx).
		Table(tableName).
		Select("TO_CHAR(DATE(created_at), 'MM/DD') AS date, COUNT(*) AS cnt").
		Where("created_at >= ?", since)
	query = applyTenantStatsExtraFilter(query, extraWhere)
	if err := query.Group("DATE(created_at)").Order("DATE(created_at) ASC").Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	return mergeTrendRows(rows, days), fillTrendCounts(rows, days), nil
}

func applyTenantStatsExtraFilter(query *gorm.DB, extraWhere string) *gorm.DB {
	switch extraWhere {
	case "":
		return query
	case "status = 'completed'":
		return query.Where("status = ?", "completed")
	case "matched_rule_id IS NOT NULL":
		return query.Where("matched_rule_id IS NOT NULL")
	case "action IN ('login','logout','impersonation_enter','impersonation_exit','impersonation_terminate','approve')":
		return query.Where("action IN ?", []string{"login", "logout", "impersonation_enter", "impersonation_exit", "impersonation_terminate", "approve"})
	default:
		return query
	}
}

func mergeTrendRows(rows []trendRow, days int) []string {
	dates := make([]string, days)
	now := time.Now()
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -(days - 1 - i))
		dates[i] = fmt.Sprintf("%02d/%02d", d.Month(), d.Day())
	}
	return dates
}

func fillTrendCounts(rows []trendRow, days int) []int64 {
	dateMap := make(map[string]int64, len(rows))
	for _, row := range rows {
		dateMap[row.Date] = row.Count
	}
	counts := make([]int64, days)
	now := time.Now()
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -(days - 1 - i))
		counts[i] = dateMap[fmt.Sprintf("%02d/%02d", d.Month(), d.Day())]
	}
	return counts
}

func fillEmptyTrend(days int) []string {
	return mergeTrendRows(nil, days)
}

func fillZeroCounts(days int) []int64 {
	return make([]int64, days)
}
