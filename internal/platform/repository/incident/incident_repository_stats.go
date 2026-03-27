package incident

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/gorm"
)

// IncidentStats 工单统计数据
type IncidentStats struct {
	Total      int64 `json:"total"`
	Scanned    int64 `json:"scanned"`
	Unscanned  int64 `json:"unscanned"`
	Matched    int64 `json:"matched"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Healed     int64 `json:"healed"`
	Failed     int64 `json:"failed"`
	Skipped    int64 `json:"skipped"`
	Dismissed  int64 `json:"dismissed"`
}

// GetStats 获取工单统计数据
func (r *IncidentRepository) GetStats(ctx context.Context) (*IncidentStats, error) {
	stats := &IncidentStats{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	if err := newDB().Model(&model.Incident{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.Incident{}).Where("scanned = ?", true).Count(&stats.Scanned).Error; err != nil {
		return nil, err
	}
	stats.Unscanned = stats.Total - stats.Scanned
	if err := newDB().Model(&model.Incident{}).Where("matched_rule_id IS NOT NULL").Count(&stats.Matched).Error; err != nil {
		return nil, err
	}
	if err := countIncidentsByHealingStatus(newDB, "pending", &stats.Pending); err != nil {
		return nil, err
	}
	if err := countIncidentsByHealingStatus(newDB, "processing", &stats.Processing); err != nil {
		return nil, err
	}
	if err := countIncidentsByHealingStatus(newDB, "healed", &stats.Healed); err != nil {
		return nil, err
	}
	if err := countIncidentsByHealingStatus(newDB, "failed", &stats.Failed); err != nil {
		return nil, err
	}
	if err := countIncidentsByHealingStatus(newDB, "skipped", &stats.Skipped); err != nil {
		return nil, err
	}
	if err := countIncidentsByHealingStatus(newDB, "dismissed", &stats.Dismissed); err != nil {
		return nil, err
	}
	return stats, nil
}

func countIncidentsByHealingStatus(newDB func() *gorm.DB, status string, count *int64) error {
	return newDB().Model(&model.Incident{}).Where("healing_status = ?", status).Count(count).Error
}
