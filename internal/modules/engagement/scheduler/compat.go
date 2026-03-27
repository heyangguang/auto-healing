package scheduler

import "github.com/company/auto-healing/internal/database"

// NewNotificationRetryScheduler 保留给兼容调用方；生产主路径应使用 NewNotificationRetrySchedulerWithDB/WithDeps。
func NewNotificationRetryScheduler() *NotificationRetryScheduler {
	return NewNotificationRetrySchedulerWithDB(database.DB)
}
