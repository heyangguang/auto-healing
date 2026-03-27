package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

// SystemHealth 系统健康状态
type SystemHealth struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	Environment   string  `json:"environment"`
	APILatencyMs  float64 `json:"api_latency_ms"`
	DBLatencyMs   float64 `json:"db_latency_ms"`
}

// HealingStats 今日自愈统计
type HealingStats struct {
	TodaySuccess int64 `json:"today_success"`
	TodayFailed  int64 `json:"today_failed"`
}

// WorkbenchIncidentStats 工单统计（工作台专用）
type WorkbenchIncidentStats struct {
	PendingCount   int64 `json:"pending_count"`
	Last7DaysTotal int64 `json:"last_7_days_total"`
}

// HostStats 主机统计
type HostStats struct {
	OnlineCount  int64 `json:"online_count"`
	OfflineCount int64 `json:"offline_count"`
}

// GetSystemHealth 获取系统健康状态
func (r *WorkbenchRepository) GetSystemHealth(ctx context.Context) (*SystemHealth, error) {
	cfg := config.GetAppConfig()
	dbLatency, status := r.measureDBHealth(ctx)
	apiLatency := 0.0
	if dbLatency > 0 {
		apiLatency = dbLatency / 2
	}
	return &SystemHealth{
		Status:        status,
		Version:       cfg.Version,
		UptimeSeconds: int64(time.Since(appStartTime).Seconds()),
		Environment:   cfg.Env,
		APILatencyMs:  apiLatency,
		DBLatencyMs:   dbLatency,
	}, nil
}

func (r *WorkbenchRepository) measureDBHealth(ctx context.Context) (float64, string) {
	startedAt := time.Now()
	sqlDB, err := r.db.DB()
	if err != nil {
		return -1, "down"
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return -1, "degraded"
	}

	dbLatency := float64(time.Since(startedAt).Microseconds()) / 1000.0
	if dbLatency > 100 {
		return dbLatency, "degraded"
	}
	return dbLatency, "healthy"
}

// GetHealingStats 获取今日自愈统计
func (r *WorkbenchRepository) GetHealingStats(ctx context.Context) (*HealingStats, error) {
	stats := &HealingStats{}
	todayStart := workbenchDayStart(time.Now())

	success, err := workbenchCountWhere(r.tenantDB(ctx), &model.FlowInstance{}, "created_at >= ? AND status = ?", todayStart, "completed")
	if err != nil {
		return nil, err
	}
	failed, err := workbenchCountWhere(r.tenantDB(ctx), &model.FlowInstance{}, "created_at >= ? AND status = ?", todayStart, "failed")
	if err != nil {
		return nil, err
	}

	stats.TodaySuccess = success
	stats.TodayFailed = failed
	return stats, nil
}

// GetIncidentStats 获取工单统计
func (r *WorkbenchRepository) GetIncidentStats(ctx context.Context) (*WorkbenchIncidentStats, error) {
	stats := &WorkbenchIncidentStats{}

	pendingCount, err := workbenchCountWhere(r.tenantDB(ctx), &model.Incident{}, "scanned = ? OR healing_status = ?", false, "pending")
	if err != nil {
		return nil, err
	}
	last7DaysTotal, err := workbenchCountWhere(r.tenantDB(ctx), &model.Incident{}, "created_at >= ?", time.Now().AddDate(0, 0, -7))
	if err != nil {
		return nil, err
	}

	stats.PendingCount = pendingCount
	stats.Last7DaysTotal = last7DaysTotal
	return stats, nil
}

// GetHostStats 获取主机统计
func (r *WorkbenchRepository) GetHostStats(ctx context.Context) (*HostStats, error) {
	stats := &HostStats{}

	onlineCount, err := workbenchCountWhere(r.tenantDB(ctx), &model.CMDBItem{}, "status = ?", "active")
	if err != nil {
		return nil, err
	}
	offlineCount, err := workbenchCountWhere(r.tenantDB(ctx), &model.CMDBItem{}, "status != ?", "active")
	if err != nil {
		return nil, err
	}

	stats.OnlineCount = onlineCount
	stats.OfflineCount = offlineCount
	return stats, nil
}

func workbenchDayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func workbenchCountWhere(db *gorm.DB, entity any, condition string, args ...any) (int64, error) {
	var count int64
	query := db.Model(entity)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	return count, query.Count(&count).Error
}
