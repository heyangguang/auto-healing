package httpapi

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 按业务域模块注册 API 路由。
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	handlers := handler.NewHandlers(cfg)
	modules := newModules(handlers)
	api := r.Group("/api/v1")

	modules.access.RegisterAuthRoutes(api)
	registerCommonRoutes(api, modules, handlers)
	registerPlatformRoutes(api, modules, handlers)
	registerTenantRoutes(api, modules, handlers)
}

func registerCommonRoutes(api *gin.RouterGroup, modules moduleRegistrars, handlers *handler.Handlers) {
	common := api.Group("/common")
	common.Use(middleware.JWTAuth(handlers.Auth.GetJWTService()))
	common.Use(middleware.ImpersonationMiddleware())
	common.Use(middleware.CommonTenantMiddleware())
	common.Use(middleware.AuditMiddleware())

	modules.access.RegisterCommonRoutes(common)
	modules.engagement.RegisterCommonRoutes(common)
	modules.ops.RegisterCommonRoutes(common)
}

func registerPlatformRoutes(api *gin.RouterGroup, modules moduleRegistrars, handlers *handler.Handlers) {
	platform := api.Group("/platform")
	platform.Use(middleware.JWTAuth(handlers.Auth.GetJWTService()))
	platform.Use(middleware.ImpersonationMiddleware())
	platform.Use(middleware.AuditMiddleware())
	platform.Use(middleware.RequirePlatformAdmin())

	modules.access.RegisterPlatformRoutes(platform)
	modules.engagement.RegisterPlatformRoutes(platform)
	modules.ops.RegisterPlatformRoutes(platform)
}

func registerTenantRoutes(api *gin.RouterGroup, modules moduleRegistrars, handlers *handler.Handlers) {
	tenant := api.Group("/tenant")
	tenant.Use(middleware.JWTAuth(handlers.Auth.GetJWTService()))
	tenant.Use(middleware.ImpersonationMiddleware())
	tenant.Use(middleware.TenantMiddleware())
	tenant.Use(middleware.AuditMiddleware())

	modules.access.RegisterTenantRoutes(tenant)
	modules.integrations.RegisterTenantRoutes(tenant)
	modules.automation.RegisterTenantRoutes(tenant)
	modules.engagement.RegisterTenantRoutes(tenant)
	modules.ops.RegisterTenantRoutes(tenant)
	modules.secrets.RegisterTenantRoutes(tenant)
}
