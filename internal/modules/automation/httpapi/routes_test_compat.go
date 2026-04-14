package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerTenantExecutionRunRoutes(runs *gin.RouterGroup, execution *ExecutionHandler) {
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

func registerTenantHealingInstanceRoutes(instances *gin.RouterGroup, healing *HealingHandler) {
	instances.GET("/search-schema", middleware.RequirePermission("healing:instances:view"), healing.GetInstanceSearchSchema)
	instances.GET("", middleware.RequirePermission("healing:instances:view"), healing.ListInstances)
	instances.GET("/stats", middleware.RequirePermission("healing:instances:view"), healing.GetInstanceStats)
	instances.GET("/:id", middleware.RequirePermission("healing:instances:view"), healing.GetInstance)
	instances.GET("/:id/recovery-logs", middleware.RequirePermission("healing:instances:view"), healing.ListInstanceRecoveryAttempts)
	instances.POST("/:id/cancel", middleware.RequirePermission("healing:flows:update"), healing.CancelInstance)
	instances.POST("/:id/recover", middleware.RequirePermission("healing:flows:update"), healing.RecoverInstance)
	instances.POST("/:id/retry", middleware.RequirePermission("healing:flows:update"), healing.RetryInstance)
	instances.GET("/:id/events", middleware.RequirePermission("healing:instances:view"), healing.InstanceEvents)
}
