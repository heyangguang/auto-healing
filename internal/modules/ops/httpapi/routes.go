package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterCommonRoutes(common *gin.RouterGroup) {
	dictionaries := common.Group("/dictionaries")
	dictionaries.GET("", r.deps.Dictionary.ListDictionaries)
	dictionaries.GET("/types", r.deps.Dictionary.ListTypes)
}

func (r Registrar) RegisterPlatformRoutes(platform *gin.RouterGroup) {
	platform.GET("/settings", middleware.RequirePermission("platform:settings:manage"), r.deps.PlatformSettings.ListSettings)
	platform.PUT("/settings/:key", middleware.RequirePermission("platform:settings:manage"), r.deps.PlatformSettings.UpdateSetting)

	auditLogs := platform.Group("/audit-logs")
	auditLogs.GET("", middleware.RequirePermission("platform:audit:list"), r.deps.PlatformAudit.ListPlatformAuditLogs)
	auditLogs.GET("/stats", middleware.RequirePermission("platform:audit:list"), r.deps.PlatformAudit.GetPlatformAuditStats)
	auditLogs.GET("/trend", middleware.RequirePermission("platform:audit:list"), r.deps.PlatformAudit.GetPlatformAuditTrend)
	auditLogs.GET("/user-ranking", middleware.RequirePermission("platform:audit:list"), r.deps.PlatformAudit.GetPlatformUserRanking)
	auditLogs.GET("/high-risk", middleware.RequirePermission("platform:audit:list"), r.deps.PlatformAudit.GetPlatformHighRiskLogs)
	auditLogs.GET("/:id", middleware.RequirePermission("platform:audit:list"), r.deps.PlatformAudit.GetPlatformAuditLog)

	dictionaries := platform.Group("/dictionaries")
	dictionaries.POST("", middleware.RequirePermission("platform:settings:manage"), r.deps.Dictionary.CreateDictionary)
	dictionaries.PUT("/:id", middleware.RequirePermission("platform:settings:manage"), r.deps.Dictionary.UpdateDictionary)
	dictionaries.DELETE("/:id", middleware.RequirePermission("platform:settings:manage"), r.deps.Dictionary.DeleteDictionary)
}

func (r Registrar) RegisterTenantRoutes(tenant *gin.RouterGroup) {
	auditLogs := tenant.Group("/audit-logs")
	auditLogs.GET("", middleware.RequirePermission("audit:list"), r.deps.Audit.ListAuditLogs)
	auditLogs.GET("/stats", middleware.RequirePermission("audit:list"), r.deps.Audit.GetAuditStats)
	auditLogs.GET("/user-ranking", middleware.RequirePermission("audit:list"), r.deps.Audit.GetUserRanking)
	auditLogs.GET("/action-grouping", middleware.RequirePermission("audit:list"), r.deps.Audit.GetActionGrouping)
	auditLogs.GET("/resource-stats", middleware.RequirePermission("audit:list"), r.deps.Audit.GetResourceTypeStats)
	auditLogs.GET("/trend", middleware.RequirePermission("audit:list"), r.deps.Audit.GetTrend)
	auditLogs.GET("/high-risk", middleware.RequirePermission("audit:list"), r.deps.Audit.GetHighRiskLogs)
	auditLogs.GET("/export", middleware.RequirePermission("audit:export"), r.deps.Audit.ExportAuditLogs)
	auditLogs.GET("/:id", middleware.RequirePermission("audit:list"), r.deps.Audit.GetAuditLog)

	blacklist := tenant.Group("/command-blacklist")
	blacklist.GET("", middleware.RequirePermission("security:blacklist:view"), r.deps.CommandBlacklist.List)
	blacklist.GET("/search-schema", middleware.RequirePermission("security:blacklist:view"), r.deps.CommandBlacklist.GetSearchSchema)
	blacklist.POST("", middleware.RequirePermission("security:blacklist:create"), r.deps.CommandBlacklist.Create)
	blacklist.POST("/batch-toggle", middleware.RequirePermission("security:blacklist:update"), r.deps.CommandBlacklist.BatchToggle)
	blacklist.POST("/simulate", middleware.RequirePermission("security:blacklist:view"), r.deps.CommandBlacklist.Simulate)
	blacklist.GET("/:id", middleware.RequirePermission("security:blacklist:view"), r.deps.CommandBlacklist.Get)
	blacklist.PUT("/:id", middleware.RequirePermission("security:blacklist:update"), r.deps.CommandBlacklist.Update)
	blacklist.DELETE("/:id", middleware.RequirePermission("security:blacklist:delete"), r.deps.CommandBlacklist.Delete)
	blacklist.POST("/:id/toggle", middleware.RequirePermission("security:blacklist:update"), r.deps.CommandBlacklist.ToggleActive)

	exemptions := tenant.Group("/blacklist-exemptions")
	exemptions.GET("", middleware.RequirePermission("security:exemption:view"), r.deps.BlacklistExemption.List)
	exemptions.GET("/search-schema", middleware.RequirePermission("security:exemption:view"), r.deps.BlacklistExemption.GetSearchSchema)
	exemptions.GET("/pending", middleware.RequirePermission("security:exemption:approve"), r.deps.BlacklistExemption.GetPending)
	exemptions.POST("", middleware.RequirePermission("security:exemption:create"), r.deps.BlacklistExemption.Create)
	exemptions.GET("/:id", middleware.RequirePermission("security:exemption:view"), r.deps.BlacklistExemption.Get)
	exemptions.POST("/:id/approve", middleware.RequirePermission("security:exemption:approve"), r.deps.BlacklistExemption.Approve)
	exemptions.POST("/:id/reject", middleware.RequirePermission("security:exemption:approve"), r.deps.BlacklistExemption.Reject)
}
