package httpapi

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 按业务域模块注册 API 路由。
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	modules := newModules(cfg)
	api := r.Group("/api/v1")

	modules.routes.access.RegisterAuthRoutes(api)
	registerCommonRoutes(api, modules)
	registerPlatformRoutes(api, modules)
	registerTenantRoutes(api, modules)
}

func registerCommonRoutes(api *gin.RouterGroup, modules moduleSet) {
	common := api.Group("/common")
	common.Use(middleware.JWTAuth(modules.access.Auth.GetJWTService()))
	common.Use(middleware.ImpersonationMiddleware())
	common.Use(middleware.CommonTenantMiddleware())
	common.Use(middleware.AuditMiddleware())

	modules.routes.access.RegisterCommonRoutes(common)
	modules.routes.engagement.RegisterCommonRoutes(common)
	modules.routes.ops.RegisterCommonRoutes(common)
}

func registerPlatformRoutes(api *gin.RouterGroup, modules moduleSet) {
	platform := api.Group("/platform")
	platform.Use(middleware.JWTAuth(modules.access.Auth.GetJWTService()))
	platform.Use(middleware.ImpersonationMiddleware())
	platform.Use(middleware.AuditMiddleware())
	platform.Use(middleware.RequirePlatformAdmin())

	modules.routes.access.RegisterPlatformRoutes(platform)
	modules.routes.engagement.RegisterPlatformRoutes(platform)
	modules.routes.ops.RegisterPlatformRoutes(platform)
}

func registerTenantRoutes(api *gin.RouterGroup, modules moduleSet) {
	tenant := api.Group("/tenant")
	tenant.Use(middleware.JWTAuth(modules.access.Auth.GetJWTService()))
	tenant.Use(middleware.ImpersonationMiddleware())
	tenant.Use(middleware.TenantMiddleware())
	tenant.Use(middleware.AuditMiddleware())

	modules.routes.access.RegisterTenantRoutes(tenant)
	modules.routes.integrations.RegisterTenantRoutes(tenant)
	modules.routes.automation.RegisterTenantRoutes(tenant)
	modules.routes.engagement.RegisterTenantRoutes(tenant)
	modules.routes.ops.RegisterTenantRoutes(tenant)
	modules.routes.secrets.RegisterTenantRoutes(tenant)
}
