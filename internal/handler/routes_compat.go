package handler

import (
	automationhttp "github.com/company/auto-healing/internal/modules/automation/httpapi"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerTenantExecutionRunRoutes(runs *gin.RouterGroup, execution *automationhttp.ExecutionHandler) {
	runs.GET("", middleware.RequirePermission("task:list"), execution.ListAllRuns)
	runs.GET("/stats", middleware.RequirePermission("task:list"), execution.GetRunStats)
	runs.GET("/search-schema", middleware.RequirePermission("task:list"), execution.GetRunSearchSchema)
	runs.GET("/trend", middleware.RequirePermission("task:list"), execution.GetRunTrend)
	runs.GET("/trigger-distribution", middleware.RequirePermission("task:list"), execution.GetTriggerDistribution)
	runs.GET("/top-failed", middleware.RequirePermission("task:list"), execution.GetTopFailedTasks)
	runs.GET("/top-active", middleware.RequirePermission("task:list"), execution.GetTopActiveTasks)
	runs.GET("/:id", middleware.RequirePermission("task:detail"), execution.GetRun)
	runs.GET("/:id/logs", middleware.RequirePermission("task:detail"), execution.GetRunLogs)
	runs.GET("/:id/stream", middleware.RequirePermission("task:detail"), execution.StreamLogs)
	runs.POST("/:id/cancel", middleware.RequirePermission("task:cancel"), execution.CancelRun)
}

func registerTenantHealingInstanceRoutes(instances *gin.RouterGroup, healing *automationhttp.HealingHandler) {
	instances.GET("/search-schema", middleware.RequirePermission("healing:instances:view"), healing.GetInstanceSearchSchema)
	instances.GET("", middleware.RequirePermission("healing:instances:view"), healing.ListInstances)
	instances.GET("/stats", middleware.RequirePermission("healing:instances:view"), healing.GetInstanceStats)
	instances.GET("/:id", middleware.RequirePermission("healing:instances:view"), healing.GetInstance)
	instances.POST("/:id/cancel", middleware.RequirePermission("healing:flows:update"), healing.CancelInstance)
	instances.POST("/:id/retry", middleware.RequirePermission("healing:flows:update"), healing.RetryInstance)
	instances.GET("/:id/events", middleware.RequirePermission("healing:instances:view"), healing.InstanceEvents)
}

func registerTenantIncidentRoutes(incidents *gin.RouterGroup, plugin *PluginHandler, healing *automationhttp.HealingHandler) {
	incidents.GET("/stats", middleware.RequirePermission("plugin:list"), plugin.GetIncidentStats)
	incidents.GET("/search-schema", middleware.RequirePermission("plugin:list"), plugin.GetIncidentSearchSchema)
	incidents.GET("", middleware.RequirePermission("plugin:list"), plugin.ListIncidents)
	incidents.POST("/batch-reset-scan", middleware.RequirePermission("plugin:sync"), plugin.BatchResetIncidentScan)
	incidents.GET("/:id", middleware.RequirePermission("plugin:list"), plugin.GetIncident)
	incidents.POST("/:id/reset-scan", middleware.RequirePermission("plugin:sync"), plugin.ResetIncidentScan)
	incidents.POST("/:id/close", middleware.RequirePermission("plugin:sync"), plugin.CloseIncident)
	incidents.POST("/:id/trigger", middleware.RequirePermission("healing:trigger:execute"), healing.TriggerIncidentManually)
	incidents.POST("/:id/dismiss", middleware.RequirePermission("healing:trigger:execute"), healing.DismissIncident)
}

func registerTenantDashboardRoutes(dashboard *gin.RouterGroup, handler *DashboardHandler) {
	dashboard.GET("/overview", middleware.RequirePermission("dashboard:view"), handler.GetOverview)
	dashboard.GET("/config", middleware.RequirePermission("dashboard:view"), handler.GetConfig)
	dashboard.PUT("/config", middleware.RequirePermission("dashboard:config:manage"), handler.SaveConfig)
	dashboard.POST("/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), handler.CreateSystemWorkspace)
	dashboard.GET("/workspaces", middleware.RequirePermission("dashboard:view"), handler.ListSystemWorkspaces)
	dashboard.PUT("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), handler.UpdateSystemWorkspace)
	dashboard.DELETE("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), handler.DeleteSystemWorkspace)
	dashboard.GET("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:view"), handler.GetRoleWorkspaces)
	dashboard.PUT("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), handler.AssignRoleWorkspaces)
}
