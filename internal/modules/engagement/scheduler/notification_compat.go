package scheduler

import (
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"gorm.io/gorm"
)

// NewNotificationRetrySchedulerWithDB 保留兼容入口；主实现统一走 WithDeps。
func NewNotificationRetrySchedulerWithDB(db *gorm.DB) *NotificationRetryScheduler {
	notifSvc := notification.NewConfiguredService(db)
	return NewNotificationRetrySchedulerWithDeps(NotificationRetrySchedulerDeps{
		NotificationService: notifSvc,
		Interval:            notificationRetryInterval(),
	})
}
