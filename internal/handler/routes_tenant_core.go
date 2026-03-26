package handler

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// ==================== 租户级路由（Tenant） ====================
// 需要认证 + 租户上下文
func setupTenantRoutes(api *gin.RouterGroup, h *Handlers) {
	tenant := api.Group("/tenant")
	tenant.Use(middleware.JWTAuth(h.Auth.GetJWTService()))
	tenant.Use(middleware.ImpersonationMiddleware())
	tenant.Use(middleware.TenantMiddleware())
	tenant.Use(middleware.AuditMiddleware())

	registerTenantUserRoutes(tenant, h)
	registerTenantRoleRoutes(tenant, h)
	registerTenantPluginRoutes(tenant, h)
	registerTenantExecutionRoutes(tenant, h)
	registerTenantNotificationRoutes(tenant, h)
	registerTenantAuditRoutes(tenant, h)
	registerTenantOperationalRoutes(tenant, h)
	registerTenantAutomationRoutes(tenant, h)
	registerTenantExperienceRoutes(tenant, h)
	registerTenantSecurityRoutes(tenant, h)
}

func registerTenantUserRoutes(tenant *gin.RouterGroup, h *Handlers) {
	users := tenant.Group("/users")
	users.GET("", middleware.RequirePermission("user:list"), h.TenantUser.ListTenantUsers)
	users.GET("/simple", middleware.RequirePermission("user:list"), h.TenantUser.ListSimpleUsers)
	users.POST("", middleware.RequirePermission("user:create"), h.TenantUser.CreateTenantUser)
	users.GET("/:id", middleware.RequirePermission("user:list"), h.TenantUser.GetTenantUser)
	users.PUT("/:id", middleware.RequirePermission("user:update"), h.TenantUser.UpdateTenantUser)
	users.DELETE("/:id", middleware.RequirePermission("user:delete"), h.TenantUser.DeleteTenantUser)
	users.POST("/:id/reset-password", middleware.RequirePermission("user:reset_password"), h.TenantUser.ResetTenantUserPassword)
	users.PUT("/:id/roles", middleware.RequirePermission("role:assign"), h.TenantUser.AssignTenantUserRoles)
}

func registerTenantRoleRoutes(tenant *gin.RouterGroup, h *Handlers) {
	roles := tenant.Group("/roles")
	roles.GET("", middleware.RequirePermission("role:list"), h.Role.ListTenantRoles)
	roles.GET("/:id", middleware.RequirePermission("role:list"), h.Role.GetRole)
	roles.GET("/:id/users", middleware.RequirePermission("role:list"), h.Role.GetTenantRoleUsers)
	roles.POST("", middleware.RequirePermission("role:create"), h.Role.CreateRole)
	roles.PUT("/:id", middleware.RequirePermission("role:update"), h.Role.UpdateRole)
	roles.DELETE("/:id", middleware.RequirePermission("role:delete"), h.Role.DeleteRole)
	roles.PUT("/:id/permissions", middleware.RequirePermission("role:assign"), h.Role.AssignRolePermissions)

	tenant.GET("/permissions", middleware.RequirePermission("role:list"), h.Permission.ListPermissions)
	tenant.GET("/permissions/tree", middleware.RequirePermission("role:list"), h.Permission.GetPermissionTree)
}

func registerTenantPluginRoutes(tenant *gin.RouterGroup, h *Handlers) {
	plugins := tenant.Group("/plugins")
	plugins.GET("", middleware.RequirePermission("plugin:list"), h.Plugin.ListPlugins)
	plugins.GET("/stats", middleware.RequirePermission("plugin:list"), h.Plugin.GetPluginStats)
	plugins.GET("/search-schema", middleware.RequirePermission("plugin:list"), h.Plugin.GetPluginSearchSchema)
	plugins.POST("", middleware.RequirePermission("plugin:create"), h.Plugin.CreatePlugin)
	plugins.GET("/:id", middleware.RequirePermission("plugin:detail"), h.Plugin.GetPlugin)
	plugins.PUT("/:id", middleware.RequirePermission("plugin:update"), h.Plugin.UpdatePlugin)
	plugins.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.Plugin.DeletePlugin)
	plugins.POST("/:id/test", middleware.RequirePermission("plugin:test"), h.Plugin.TestPlugin)
	plugins.POST("/:id/activate", middleware.RequirePermission("plugin:update"), h.Plugin.ActivatePlugin)
	plugins.POST("/:id/deactivate", middleware.RequirePermission("plugin:update"), h.Plugin.DeactivatePlugin)
	plugins.POST("/:id/sync", middleware.RequirePermission("plugin:sync"), h.Plugin.SyncPlugin)
	plugins.GET("/:id/logs", middleware.RequirePermission("plugin:list"), h.Plugin.GetPluginSyncLogs)
}

func registerTenantExecutionRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantExecutionTaskRoutes(tenant.Group("/execution-tasks"), h)
	registerTenantExecutionRunRoutes(tenant.Group("/execution-runs"), h)
	registerTenantScheduleRoutes(tenant.Group("/execution-schedules"), h)
}

func registerTenantExecutionTaskRoutes(tasks *gin.RouterGroup, h *Handlers) {
	tasks.GET("", middleware.RequirePermission("task:list"), h.Execution.ListTasks)
	tasks.POST("", middleware.RequirePermission("task:create"), h.Execution.CreateTask)
	tasks.GET("/stats", middleware.RequirePermission("task:list"), h.Execution.GetTaskStats)
	tasks.GET("/search-schema", middleware.RequirePermission("task:list"), h.Execution.GetTaskSearchSchema)
	tasks.POST("/batch-confirm-review", middleware.RequirePermission("task:update"), h.Execution.BatchConfirmReview)
	tasks.GET("/:id", middleware.RequirePermission("task:detail"), h.Execution.GetTask)
	tasks.PUT("/:id", middleware.RequirePermission("task:update"), h.Execution.UpdateTask)
	tasks.DELETE("/:id", middleware.RequirePermission("task:delete"), h.Execution.DeleteTask)
	tasks.POST("/:id/execute", middleware.RequirePermission("playbook:execute"), h.Execution.ExecuteTask)
	tasks.POST("/:id/confirm-review", middleware.RequirePermission("task:update"), h.Execution.ConfirmReview)
	tasks.GET("/:id/runs", middleware.RequirePermission("task:detail"), h.Execution.ListRuns)
}

func registerTenantExecutionRunRoutes(runs *gin.RouterGroup, h *Handlers) {
	runs.GET("", middleware.RequirePermission("task:list"), h.Execution.ListAllRuns)
	runs.GET("/stats", middleware.RequirePermission("task:list"), h.Execution.GetRunStats)
	runs.GET("/search-schema", middleware.RequirePermission("task:list"), h.Execution.GetRunSearchSchema)
	runs.GET("/trend", middleware.RequirePermission("task:list"), h.Execution.GetRunTrend)
	runs.GET("/trigger-distribution", middleware.RequirePermission("task:list"), h.Execution.GetTriggerDistribution)
	runs.GET("/top-failed", middleware.RequirePermission("task:list"), h.Execution.GetTopFailedTasks)
	runs.GET("/top-active", middleware.RequirePermission("task:list"), h.Execution.GetTopActiveTasks)
	runs.GET("/:id", middleware.RequirePermission("task:detail"), h.Execution.GetRun)
	runs.GET("/:id/logs", middleware.RequirePermission("task:detail"), h.Execution.GetRunLogs)
	runs.GET("/:id/stream", middleware.RequirePermission("task:detail"), h.Execution.StreamLogs)
	runs.POST("/:id/cancel", middleware.RequirePermission("task:cancel"), h.Execution.CancelRun)
}

func registerTenantScheduleRoutes(schedules *gin.RouterGroup, h *Handlers) {
	schedules.GET("", middleware.RequirePermission("task:list"), h.Schedule.List)
	schedules.POST("", middleware.RequirePermission("task:create"), h.Schedule.Create)
	schedules.GET("/stats", middleware.RequirePermission("task:list"), h.Schedule.GetStats)
	schedules.GET("/timeline", middleware.RequirePermission("task:list"), h.Schedule.GetTimeline)
	schedules.GET("/:id", middleware.RequirePermission("task:detail"), h.Schedule.Get)
	schedules.PUT("/:id", middleware.RequirePermission("task:update"), h.Schedule.Update)
	schedules.DELETE("/:id", middleware.RequirePermission("task:delete"), h.Schedule.Delete)
	schedules.POST("/:id/enable", middleware.RequirePermission("task:update"), h.Schedule.Enable)
	schedules.POST("/:id/disable", middleware.RequirePermission("task:update"), h.Schedule.Disable)
}

func registerTenantNotificationRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantChannelRoutes(tenant.Group("/channels"), h)
	registerTenantTemplateRoutes(tenant.Group("/templates"), h)
	registerTenantSendNotificationRoutes(tenant.Group("/notifications"), h)
	tenant.GET("/template-variables", middleware.RequirePermission("template:list"), h.Notification.GetAvailableVariables)
}

func registerTenantChannelRoutes(channels *gin.RouterGroup, h *Handlers) {
	channels.GET("", middleware.RequirePermission("channel:list"), h.Notification.ListChannels)
	channels.POST("", middleware.RequirePermission("channel:create"), h.Notification.CreateChannel)
	channels.GET("/:id", middleware.RequirePermission("channel:list"), h.Notification.GetChannel)
	channels.PUT("/:id", middleware.RequirePermission("channel:update"), h.Notification.UpdateChannel)
	channels.DELETE("/:id", middleware.RequirePermission("channel:delete"), h.Notification.DeleteChannel)
	channels.POST("/:id/test", middleware.RequirePermission("channel:update"), h.Notification.TestChannel)
}

func registerTenantTemplateRoutes(templates *gin.RouterGroup, h *Handlers) {
	templates.GET("", middleware.RequirePermission("template:list"), h.Notification.ListTemplates)
	templates.POST("", middleware.RequirePermission("template:create"), h.Notification.CreateTemplate)
	templates.GET("/:id", middleware.RequirePermission("template:list"), h.Notification.GetTemplate)
	templates.PUT("/:id", middleware.RequirePermission("template:update"), h.Notification.UpdateTemplate)
	templates.DELETE("/:id", middleware.RequirePermission("template:delete"), h.Notification.DeleteTemplate)
	templates.POST("/:id/preview", middleware.RequirePermission("template:list"), h.Notification.PreviewTemplate)
}

func registerTenantSendNotificationRoutes(notifications *gin.RouterGroup, h *Handlers) {
	notifications.POST("/send", middleware.RequirePermission("notification:send"), h.Notification.SendNotification)
	notifications.GET("", middleware.RequirePermission("notification:list"), h.Notification.ListNotifications)
	notifications.GET("/stats", middleware.RequirePermission("notification:list"), h.Notification.GetStats)
	notifications.GET("/:id", middleware.RequirePermission("notification:list"), h.Notification.GetNotification)
}

func registerTenantAuditRoutes(tenant *gin.RouterGroup, h *Handlers) {
	auditLogs := tenant.Group("/audit-logs")
	auditLogs.GET("", middleware.RequirePermission("audit:list"), h.Audit.ListAuditLogs)
	auditLogs.GET("/stats", middleware.RequirePermission("audit:list"), h.Audit.GetAuditStats)
	auditLogs.GET("/user-ranking", middleware.RequirePermission("audit:list"), h.Audit.GetUserRanking)
	auditLogs.GET("/action-grouping", middleware.RequirePermission("audit:list"), h.Audit.GetActionGrouping)
	auditLogs.GET("/resource-stats", middleware.RequirePermission("audit:list"), h.Audit.GetResourceTypeStats)
	auditLogs.GET("/trend", middleware.RequirePermission("audit:list"), h.Audit.GetTrend)
	auditLogs.GET("/high-risk", middleware.RequirePermission("audit:list"), h.Audit.GetHighRiskLogs)
	auditLogs.GET("/export", middleware.RequirePermission("audit:export"), h.Audit.ExportAuditLogs)
	auditLogs.GET("/:id", middleware.RequirePermission("audit:list"), h.Audit.GetAuditLog)
}
