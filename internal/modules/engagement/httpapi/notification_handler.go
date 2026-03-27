package httpapi

import (
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
)

// NotificationHandler 通知处理器
type NotificationHandler struct {
	svc       *notification.Service
	notifRepo *engagementrepo.NotificationRepository
}

type NotificationHandlerDeps struct {
	Service          *notification.Service
	NotificationRepo *engagementrepo.NotificationRepository
}

func NewNotificationHandlerWithDeps(deps NotificationHandlerDeps) *NotificationHandler {
	return &NotificationHandler{
		svc:       deps.Service,
		notifRepo: deps.NotificationRepo,
	}
}

// PreviewTemplateRequest 预览模板请求
type PreviewTemplateRequest struct {
	Variables map[string]interface{} `json:"variables"`
}
