package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterTenantRoutes(tenant *gin.RouterGroup) {
	sources := tenant.Group("/secrets-sources")
	sources.GET("", middleware.RequirePermission("plugin:list"), r.deps.Secrets.ListSources)
	sources.POST("", middleware.RequirePermission("plugin:create"), r.deps.Secrets.CreateSource)
	sources.GET("/stats", middleware.RequirePermission("plugin:list"), r.deps.Secrets.GetStats)
	sources.GET("/:id", middleware.RequirePermission("plugin:list"), r.deps.Secrets.GetSource)
	sources.PUT("/:id", middleware.RequirePermission("plugin:update"), r.deps.Secrets.UpdateSource)
	sources.DELETE("/:id", middleware.RequirePermission("plugin:delete"), r.deps.Secrets.DeleteSource)
	sources.POST("/:id/test", middleware.RequirePermission("plugin:test"), r.deps.Secrets.TestConnection)
	sources.POST("/:id/test-query", middleware.RequirePermission("plugin:test"), r.deps.Secrets.TestQuery)
	sources.POST("/:id/enable", middleware.RequirePermission("plugin:update"), r.deps.Secrets.Enable)
	sources.POST("/:id/disable", middleware.RequirePermission("plugin:update"), r.deps.Secrets.Disable)

	tenant.POST("/secrets/query", middleware.RequirePermission("secrets:query"), r.deps.Secrets.QuerySecret)
}
