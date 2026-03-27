package httpapi

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRoutes 按业务域模块注册 API 路由。
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	SetupRoutesWithDB(r, cfg, nil)
}

func SetupRoutesWithDB(r *gin.Engine, cfg *config.Config, db *gorm.DB) {
	modules := newModulesWithDB(cfg, db)
	api := r.Group("/api/v1")

	modules.routes.access.RegisterAuthRoutes(api)
	registerCommonRoutes(api, modules)
	registerPlatformRoutes(api, modules)
	registerTenantRoutes(api, modules)
}

func registerCommonRoutes(api *gin.RouterGroup, modules moduleSet) {
	middlewareDeps := modules.routes.access.Dependencies().Middleware
	common := api.Group("/common")
	common.Use(middleware.JWTAuthWithDeps(modules.access.Auth.GetJWTService(), middlewareDeps))
	common.Use(middleware.ImpersonationMiddlewareWithDeps(middlewareDeps))
	common.Use(middleware.CommonTenantMiddlewareWithDeps(middlewareDeps))
	common.Use(middleware.AuditMiddlewareWithDeps(middlewareDeps))

	modules.routes.access.RegisterCommonRoutes(common)
	modules.routes.engagement.RegisterCommonRoutes(common)
	modules.routes.ops.RegisterCommonRoutes(common)
}

func registerPlatformRoutes(api *gin.RouterGroup, modules moduleSet) {
	middlewareDeps := modules.routes.access.Dependencies().Middleware
	platform := api.Group("/platform")
	platform.Use(middleware.JWTAuthWithDeps(modules.access.Auth.GetJWTService(), middlewareDeps))
	platform.Use(middleware.ImpersonationMiddlewareWithDeps(middlewareDeps))
	platform.Use(middleware.AuditMiddlewareWithDeps(middlewareDeps))
	platform.Use(middleware.RequirePlatformAdminWithDeps(middlewareDeps))

	modules.routes.access.RegisterPlatformRoutes(platform)
	modules.routes.engagement.RegisterPlatformRoutes(platform)
	modules.routes.ops.RegisterPlatformRoutes(platform)
}

func registerTenantRoutes(api *gin.RouterGroup, modules moduleSet) {
	middlewareDeps := modules.routes.access.Dependencies().Middleware
	tenant := api.Group("/tenant")
	tenant.Use(middleware.JWTAuthWithDeps(modules.access.Auth.GetJWTService(), middlewareDeps))
	tenant.Use(middleware.ImpersonationMiddlewareWithDeps(middlewareDeps))
	tenant.Use(middleware.TenantMiddlewareWithDeps(middlewareDeps))
	tenant.Use(middleware.AuditMiddlewareWithDeps(middlewareDeps))

	modules.routes.access.RegisterTenantRoutes(tenant)
	modules.routes.integrations.RegisterTenantRoutes(tenant)
	modules.routes.automation.RegisterTenantRoutes(tenant)
	modules.routes.engagement.RegisterTenantRoutes(tenant)
	modules.routes.ops.RegisterTenantRoutes(tenant)
	modules.routes.secrets.RegisterTenantRoutes(tenant)
}
