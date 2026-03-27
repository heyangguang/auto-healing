package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterTenantRoutes(tenant *gin.RouterGroup) {
	dashboard := tenant.Group("/dashboard")
	dashboard.GET("/overview", middleware.RequirePermission("dashboard:view"), r.deps.Dashboard.GetOverview)
	dashboard.GET("/config", middleware.RequirePermission("dashboard:view"), r.deps.Dashboard.GetConfig)
	dashboard.PUT("/config", middleware.RequirePermission("dashboard:config:manage"), r.deps.Dashboard.SaveConfig)
	dashboard.POST("/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), r.deps.Dashboard.CreateSystemWorkspace)
	dashboard.GET("/workspaces", middleware.RequirePermission("dashboard:view"), r.deps.Dashboard.ListSystemWorkspaces)
	dashboard.PUT("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), r.deps.Dashboard.UpdateSystemWorkspace)
	dashboard.DELETE("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), r.deps.Dashboard.DeleteSystemWorkspace)
	dashboard.GET("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:view"), r.deps.Dashboard.GetRoleWorkspaces)
	dashboard.PUT("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), r.deps.Dashboard.AssignRoleWorkspaces)

	channels := tenant.Group("/channels")
	channels.GET("", middleware.RequirePermission("channel:list"), r.deps.Notification.ListChannels)
	channels.POST("", middleware.RequirePermission("channel:create"), r.deps.Notification.CreateChannel)
	channels.GET("/:id", middleware.RequirePermission("channel:list"), r.deps.Notification.GetChannel)
	channels.PUT("/:id", middleware.RequirePermission("channel:update"), r.deps.Notification.UpdateChannel)
	channels.DELETE("/:id", middleware.RequirePermission("channel:delete"), r.deps.Notification.DeleteChannel)
	channels.POST("/:id/test", middleware.RequirePermission("channel:update"), r.deps.Notification.TestChannel)

	templates := tenant.Group("/templates")
	templates.GET("", middleware.RequirePermission("template:list"), r.deps.Notification.ListTemplates)
	templates.POST("", middleware.RequirePermission("template:create"), r.deps.Notification.CreateTemplate)
	templates.GET("/:id", middleware.RequirePermission("template:list"), r.deps.Notification.GetTemplate)
	templates.PUT("/:id", middleware.RequirePermission("template:update"), r.deps.Notification.UpdateTemplate)
	templates.DELETE("/:id", middleware.RequirePermission("template:delete"), r.deps.Notification.DeleteTemplate)
	templates.POST("/:id/preview", middleware.RequirePermission("template:list"), r.deps.Notification.PreviewTemplate)

	notifications := tenant.Group("/notifications")
	notifications.POST("/send", middleware.RequirePermission("notification:send"), r.deps.Notification.SendNotification)
	notifications.GET("", middleware.RequirePermission("notification:list"), r.deps.Notification.ListNotifications)
	notifications.GET("/stats", middleware.RequirePermission("notification:list"), r.deps.Notification.GetStats)
	notifications.GET("/:id", middleware.RequirePermission("notification:list"), r.deps.Notification.GetNotification)
	tenant.GET("/template-variables", middleware.RequirePermission("template:list"), r.deps.Notification.GetAvailableVariables)

	siteMessages := tenant.Group("/site-messages")
	siteMessages.GET("/unread-count", r.deps.SiteMessage.GetUnreadCount)
	siteMessages.GET("/events", r.deps.SiteMessage.Events)
	siteMessages.PUT("/read", r.deps.SiteMessage.MarkRead)
	siteMessages.PUT("/read-all", r.deps.SiteMessage.MarkAllRead)
	siteMessages.GET("", middleware.RequirePermission("site-message:list"), r.deps.SiteMessage.ListMessages)
}
