package handler

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

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

func registerTenantHealingInstanceRoutes(instances *gin.RouterGroup, h *Handlers) {
	instances.GET("/search-schema", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstanceSearchSchema)
	instances.GET("", middleware.RequirePermission("healing:instances:view"), h.Healing.ListInstances)
	instances.GET("/stats", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstanceStats)
	instances.GET("/:id", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstance)
	instances.POST("/:id/cancel", middleware.RequirePermission("healing:flows:update"), h.Healing.CancelInstance)
	instances.POST("/:id/retry", middleware.RequirePermission("healing:flows:update"), h.Healing.RetryInstance)
	instances.GET("/:id/events", middleware.RequirePermission("healing:instances:view"), h.Healing.InstanceEvents)
}

func registerTenantIncidentRoutes(incidents *gin.RouterGroup, h *Handlers) {
	incidents.GET("/stats", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncidentStats)
	incidents.GET("/search-schema", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncidentSearchSchema)
	incidents.GET("", middleware.RequirePermission("plugin:list"), h.Plugin.ListIncidents)
	incidents.POST("/batch-reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.BatchResetIncidentScan)
	incidents.GET("/:id", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncident)
	incidents.POST("/:id/reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.ResetIncidentScan)
	incidents.POST("/:id/close", middleware.RequirePermission("plugin:sync"), h.Plugin.CloseIncident)
	incidents.POST("/:id/trigger", middleware.RequirePermission("healing:trigger:execute"), h.Healing.TriggerIncidentManually)
	incidents.POST("/:id/dismiss", middleware.RequirePermission("healing:trigger:execute"), h.Healing.DismissIncident)
}

func registerTenantDashboardRoutes(dashboard *gin.RouterGroup, h *Handlers) {
	dashboard.GET("/overview", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetOverview)
	dashboard.GET("/config", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetConfig)
	dashboard.PUT("/config", middleware.RequirePermission("dashboard:config:manage"), h.Dashboard.SaveConfig)
	dashboard.POST("/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.CreateSystemWorkspace)
	dashboard.GET("/workspaces", middleware.RequirePermission("dashboard:view"), h.Dashboard.ListSystemWorkspaces)
	dashboard.PUT("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.UpdateSystemWorkspace)
	dashboard.DELETE("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.DeleteSystemWorkspace)
	dashboard.GET("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetRoleWorkspaces)
	dashboard.PUT("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.AssignRoleWorkspaces)
}
