package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"gorm.io/gorm"
)

// GetStats 获取定时任务调度统计信息
func (r *ScheduleRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	var total int64
	if err := newDB().Model(&model.ExecutionSchedule{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var byStatus []statusCount
	if err := newDB().Model(&model.ExecutionSchedule{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&byStatus).Error; err != nil {
		return nil, err
	}
	stats["by_status"] = byStatus

	type scheduleTypeCount struct {
		ScheduleType string `json:"schedule_type"`
		Count        int64  `json:"count"`
	}
	var byType []scheduleTypeCount
	if err := newDB().Model(&model.ExecutionSchedule{}).
		Select("schedule_type, count(*) as count").
		Group("schedule_type").
		Scan(&byType).Error; err != nil {
		return nil, err
	}
	stats["by_schedule_type"] = byType

	var enabledCount int64
	if err := newDB().Model(&model.ExecutionSchedule{}).Where("enabled = ?", true).Count(&enabledCount).Error; err != nil {
		return nil, err
	}
	stats["enabled_count"] = enabledCount
	stats["disabled_count"] = total - enabledCount
	return stats, nil
}
