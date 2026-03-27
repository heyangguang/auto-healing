package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"gorm.io/gorm"
)

// GetStats 获取通知统计信息
func (r *NotificationRepository) GetStats(ctx context.Context) (map[string]any, error) {
	stats := make(map[string]any)
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }

	channelsTotal, err := notificationCount(newDB().Model(&model.NotificationChannel{}))
	if err != nil {
		return nil, err
	}
	channelTypeCounts, err := notificationTypeCounts(newDB().Model(&model.NotificationChannel{}))
	if err != nil {
		return nil, err
	}
	templatesTotal, err := notificationCount(newDB().Model(&model.NotificationTemplate{}))
	if err != nil {
		return nil, err
	}
	templatesActive, err := notificationCount(newDB().Model(&model.NotificationTemplate{}).Where("is_active = ?", true))
	if err != nil {
		return nil, err
	}
	logsTotal, err := notificationCount(newDB().Model(&model.NotificationLog{}))
	if err != nil {
		return nil, err
	}
	logStatusCounts, err := notificationStatusCounts(newDB().Model(&model.NotificationLog{}))
	if err != nil {
		return nil, err
	}

	stats["channels_total"] = channelsTotal
	stats["channels_by_type"] = channelTypeCounts
	stats["templates_total"] = templatesTotal
	stats["templates_active"] = templatesActive
	stats["logs_total"] = logsTotal
	stats["logs_by_status"] = logStatusCounts
	return stats, nil
}

func notificationCount(db *gorm.DB) (int64, error) {
	var count int64
	return count, db.Count(&count).Error
}

func notificationTypeCounts(db *gorm.DB) ([]map[string]any, error) {
	var rows []struct {
		Type  string `json:"type" gorm:"column:type"`
		Count int64  `json:"count" gorm:"column:count"`
	}
	err := db.Select("type, count(*) as count").Group("type").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{"type": row.Type, "count": row.Count})
	}
	return result, nil
}

func notificationStatusCounts(db *gorm.DB) ([]map[string]any, error) {
	var rows []struct {
		Status string `json:"status" gorm:"column:status"`
		Count  int64  `json:"count" gorm:"column:count"`
	}
	err := db.Select("status, count(*) as count").Group("status").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{"status": row.Status, "count": row.Count})
	}
	return result, nil
}
