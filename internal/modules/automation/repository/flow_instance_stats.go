package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

// GetStats 获取流程实例统计信息
func (r *FlowInstanceRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	var total int64
	if err := newDB().Model(&model.FlowInstance{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []statusCount
	newDB().Model(&model.FlowInstance{}).Select("status, count(*) as count").Group("status").Scan(&statusCounts)
	stats["by_status"] = statusCounts
	return stats, nil
}
