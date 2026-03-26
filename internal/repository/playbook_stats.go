package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

// GetStats 获取 Playbook 统计信息
func (r *PlaybookRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	var total int64
	if err := newDB().Model(&model.Playbook{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var byStatus []statusCount
	newDB().Model(&model.Playbook{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&byStatus)
	stats["by_status"] = byStatus

	type configModeCount struct {
		ConfigMode string `json:"config_mode"`
		Count      int64  `json:"count"`
	}
	var byConfigMode []configModeCount
	newDB().Model(&model.Playbook{}).
		Select("config_mode, count(*) as count").
		Group("config_mode").
		Scan(&byConfigMode)
	stats["by_config_mode"] = byConfigMode

	return stats, nil
}
