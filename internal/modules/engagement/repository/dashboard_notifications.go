package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationSection struct {
	ChannelsTotal  int64          `json:"channels_total"`
	TemplatesTotal int64          `json:"templates_total"`
	LogsTotal      int64          `json:"logs_total"`
	DeliveryRate   float64        `json:"delivery_rate"`
	ByChannelType  []StatusCount  `json:"by_channel_type"`
	ByLogStatus    []StatusCount  `json:"by_log_status"`
	Trend7d        []TrendPoint   `json:"trend_7d"`
	RecentLogs     []NotifLogItem `json:"recent_logs"`
	FailedLogs     []NotifLogItem `json:"failed_logs"`
}

type NotifLogItem struct {
	ID        uuid.UUID `json:"id"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetNotificationSection(ctx context.Context) (*NotificationSection, error) {
	section := &NotificationSection{}
	db := r.tenantDB(ctx)

	if err := countModel(db, &model.NotificationChannel{}, &section.ChannelsTotal); err != nil {
		return nil, err
	}
	if err := countModel(db, &model.NotificationTemplate{}, &section.TemplatesTotal); err != nil {
		return nil, err
	}
	if err := countModel(db, &model.NotificationLog{}, &section.LogsTotal); err != nil {
		return nil, err
	}
	rate, err := calculateDeliveryRate(db, section.LogsTotal)
	if err != nil {
		return nil, err
	}
	section.DeliveryRate = rate
	if err := scanStatusCounts(db, &model.NotificationChannel{}, "type", &section.ByChannelType); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(db, &model.NotificationLog{}, "status", &section.ByLogStatus); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(db, &model.NotificationLog{}, "created_at", time.Now().AddDate(0, 0, -7), &section.Trend7d); err != nil {
		return nil, err
	}
	recentLogs, err := listNotificationLogs(db.Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentLogs = recentLogs
	failedLogs, err := listNotificationLogs(db.Where("status = ?", "failed").Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.FailedLogs = failedLogs
	return section, nil
}

func calculateDeliveryRate(db *gorm.DB, total int64) (float64, error) {
	if total == 0 {
		return 0, nil
	}
	var sentCount int64
	if err := countModel(db.Where("status IN ?", []string{"sent", "delivered"}), &model.NotificationLog{}, &sentCount); err != nil {
		return 0, err
	}
	return float64(sentCount) / float64(total) * 100, nil
}

func listNotificationLogs(query *gorm.DB) ([]NotifLogItem, error) {
	var logs []model.NotificationLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	items := make([]NotifLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, NotifLogItem{
			ID:        log.ID,
			Subject:   log.Subject,
			Status:    log.Status,
			CreatedAt: log.CreatedAt,
		})
	}
	return items, nil
}
