package handler

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/repository"
)

// NotificationHandler 通知处理器
type NotificationHandler struct {
	svc       *notification.Service
	notifRepo *repository.NotificationRepository
}

// NewNotificationHandler 创建通知处理器
func NewNotificationHandler() *NotificationHandler {
	db := database.DB
	return &NotificationHandler{
		svc:       notification.NewConfiguredService(db),
		notifRepo: repository.NewNotificationRepository(db),
	}
}

// PreviewTemplateRequest 预览模板请求
type PreviewTemplateRequest struct {
	Variables map[string]interface{} `json:"variables"`
}
