package handler

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// ==================== 平台级路由（Platform） ====================
// 需要认证 + 平台管理员权限，不需要租户上下文
func setupPlatformRoutes(api *gin.RouterGroup, h *Handlers) {
	platform := api.Group("/platform")
	platform.Use(middleware.JWTAuth(h.Auth.GetJWTService()))
	platform.Use(middleware.ImpersonationMiddleware())
	platform.Use(middleware.AuditMiddleware())
	platform.Use(middleware.RequirePlatformAdmin())

	registerPlatformUserRoutes(platform, h)
	registerPlatformRoleRoutes(platform, h)
	registerPlatformTenantRoutes(platform, h)
	registerPlatformSiteMessageRoutes(platform, h)
	registerPlatformAuditRoutes(platform, h)
	registerPlatformImpersonationRoutes(platform, h)
	registerPlatformDictionaryRoutes(platform, h)
}

func registerPlatformUserRoutes(platform *gin.RouterGroup, h *Handlers) {
	users := platform.Group("/users")
	users.GET("", middleware.RequirePermission("platform:users:list"), h.User.ListUsers)
	users.POST("", middleware.RequirePermission("platform:users:create"), h.User.CreateUser)
	users.GET("/simple", middleware.RequirePermission("platform:users:list"), h.User.ListSimpleUsers)
	users.GET("/:id", middleware.RequirePermission("platform:users:list"), h.User.GetUser)
	users.PUT("/:id", middleware.RequirePermission("platform:users:update"), h.User.UpdateUser)
	users.DELETE("/:id", middleware.RequirePermission("platform:users:delete"), h.User.DeleteUser)
	users.POST("/:id/reset-password", middleware.RequirePermission("platform:users:reset_password"), h.User.ResetPassword)
	users.PUT("/:id/roles", middleware.RequirePermission("platform:roles:manage"), h.User.AssignUserRoles)
}

func registerPlatformRoleRoutes(platform *gin.RouterGroup, h *Handlers) {
	roles := platform.Group("/roles")
	roles.GET("", middleware.RequirePermission("platform:roles:list"), h.Role.ListRoles)
	roles.POST("", middleware.RequirePermission("platform:roles:manage"), h.Role.CreateRole)
	roles.GET("/:id", middleware.RequirePermission("platform:roles:list"), h.Role.GetRole)
	roles.PUT("/:id", middleware.RequirePermission("platform:roles:manage"), h.Role.UpdateRole)
	roles.DELETE("/:id", middleware.RequirePermission("platform:roles:manage"), h.Role.DeleteRole)
	roles.PUT("/:id/permissions", middleware.RequirePermission("platform:roles:manage"), h.Role.AssignRolePermissions)
	roles.GET("/:id/users", middleware.RequirePermission("platform:roles:list"), h.Role.GetRoleUsers)

	platform.GET("/tenant-roles", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Role.ListSystemTenantRoles)
	platform.GET("/permissions", middleware.RequirePermission("platform:permissions:list"), h.Permission.ListPermissions)
	platform.GET("/permissions/tree", middleware.RequirePermission("platform:permissions:list"), h.Permission.GetPermissionTree)
}

func registerPlatformTenantRoutes(platform *gin.RouterGroup, h *Handlers) {
	platform.GET("/settings", middleware.RequirePermission("platform:settings:manage"), h.PlatformSettings.ListSettings)
	platform.PUT("/settings/:key", middleware.RequirePermission("platform:settings:manage"), h.PlatformSettings.UpdateSetting)

	platform.GET("/tenants", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.ListTenants)
	platform.POST("/tenants", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.CreateTenant)
	platform.GET("/tenants/stats", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.GetTenantStats)
	platform.GET("/tenants/trends", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.GetTenantTrends)
	platform.GET("/tenants/:id", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.GetTenant)
	platform.PUT("/tenants/:id", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.UpdateTenant)
	platform.DELETE("/tenants/:id", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.DeleteTenant)
	platform.GET("/tenants/:id/members", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.ListMembers)
	platform.POST("/tenants/:id/members", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.AddMember)
	platform.DELETE("/tenants/:id/members/:userId", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.RemoveMember)
	platform.PUT("/tenants/:id/members/:userId/role", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.UpdateMemberRole)
	platform.POST("/tenants/:id/invitations", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.InviteToTenant)
	platform.GET("/tenants/:id/invitations", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.ListInvitations)
	platform.DELETE("/tenants/:id/invitations/:invId", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.CancelInvitation)
}

func registerPlatformSiteMessageRoutes(platform *gin.RouterGroup, h *Handlers) {
	siteMessages := platform.Group("/site-messages")
	siteMessages.POST("", middleware.RequirePermission("platform:messages:send"), h.SiteMessage.CreateMessage)
	siteMessages.GET("/settings", middleware.RequirePermission("site-message:settings:view"), h.SiteMessage.GetSettings)
	siteMessages.PUT("/settings", middleware.RequirePermission("site-message:settings:manage"), h.SiteMessage.UpdateSettings)
}

func registerPlatformAuditRoutes(platform *gin.RouterGroup, h *Handlers) {
	auditLogs := platform.Group("/audit-logs")
	auditLogs.GET("", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.ListPlatformAuditLogs)
	auditLogs.GET("/stats", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformAuditStats)
	auditLogs.GET("/trend", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformAuditTrend)
	auditLogs.GET("/user-ranking", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformUserRanking)
	auditLogs.GET("/high-risk", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformHighRiskLogs)
	auditLogs.GET("/:id", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformAuditLog)
}

func registerPlatformImpersonationRoutes(platform *gin.RouterGroup, h *Handlers) {
	impersonation := platform.Group("/impersonation")
	impersonation.Use(middleware.RequirePermission("platform:tenants:manage"))
	impersonation.POST("/requests", h.Impersonation.CreateRequest)
	impersonation.GET("/requests", h.Impersonation.ListMyRequests)
	impersonation.GET("/requests/:id", h.Impersonation.GetRequest)
	impersonation.POST("/requests/:id/enter", h.Impersonation.EnterTenant)
	impersonation.POST("/requests/:id/exit", h.Impersonation.ExitTenant)
	impersonation.POST("/requests/:id/terminate", h.Impersonation.TerminateSession)
	impersonation.POST("/requests/:id/cancel", h.Impersonation.CancelRequest)
}

func registerPlatformDictionaryRoutes(platform *gin.RouterGroup, h *Handlers) {
	dictionaries := platform.Group("/dictionaries")
	dictionaries.POST("", middleware.RequirePermission("platform:settings:manage"), h.Dictionary.CreateDictionary)
	dictionaries.PUT("/:id", middleware.RequirePermission("platform:settings:manage"), h.Dictionary.UpdateDictionary)
	dictionaries.DELETE("/:id", middleware.RequirePermission("platform:settings:manage"), h.Dictionary.DeleteDictionary)
}
