package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

type incidentHealingActions interface {
	TriggerIncidentManually(*gin.Context)
	DismissIncident(*gin.Context)
}

func registerTenantIncidentRoutes(incidents *gin.RouterGroup, plugin *PluginHandler, healing incidentHealingActions) {
	incidents.GET("/stats", middleware.RequirePermission("plugin:list"), plugin.GetIncidentStats)
	incidents.GET("/search-schema", middleware.RequirePermission("plugin:list"), plugin.GetIncidentSearchSchema)
	incidents.GET("", middleware.RequirePermission("plugin:list"), plugin.ListIncidents)
	incidents.POST("/batch-reset-scan", middleware.RequirePermission("plugin:sync"), plugin.BatchResetIncidentScan)
	incidents.GET("/:id", middleware.RequirePermission("plugin:list"), plugin.GetIncident)
	incidents.GET("/:id/writeback-logs", middleware.RequirePermission("plugin:list"), plugin.ListIncidentWritebackLogs)
	incidents.POST("/:id/reset-scan", middleware.RequirePermission("plugin:sync"), plugin.ResetIncidentScan)
	incidents.POST("/:id/close", middleware.RequirePermission("plugin:sync"), plugin.CloseIncident)
	incidents.POST("/:id/trigger", middleware.RequirePermission("healing:trigger:execute"), healing.TriggerIncidentManually)
	incidents.POST("/:id/dismiss", middleware.RequirePermission("healing:trigger:execute"), healing.DismissIncident)
}
