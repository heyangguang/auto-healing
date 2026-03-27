package httpapi

import (
	"github.com/company/auto-healing/internal/database"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/notification"
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

// NewNotificationHandler 创建通知处理器
func NewNotificationHandler() *NotificationHandler {
	db := database.DB
	return NewNotificationHandlerWithDeps(NotificationHandlerDeps{
		Service:          notification.NewConfiguredService(db),
		NotificationRepo: engagementrepo.NewNotificationRepository(db),
	})
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
