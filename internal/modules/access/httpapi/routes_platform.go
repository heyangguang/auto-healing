package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterPlatformRoutes(platform *gin.RouterGroup) {
	users := platform.Group("/users")
	users.GET("", middleware.RequirePermission("platform:users:list"), r.deps.User.ListUsers)
	users.POST("", middleware.RequirePermission("platform:users:create"), r.deps.User.CreateUser)
	users.GET("/simple", middleware.RequirePermission("platform:users:list"), r.deps.User.ListSimpleUsers)
	users.GET("/:id", middleware.RequirePermission("platform:users:list"), r.deps.User.GetUser)
	users.PUT("/:id", middleware.RequirePermission("platform:users:update"), r.deps.User.UpdateUser)
	users.DELETE("/:id", middleware.RequirePermission("platform:users:delete"), r.deps.User.DeleteUser)
	users.POST("/:id/reset-password", middleware.RequirePermission("platform:users:reset_password"), r.deps.User.ResetPassword)
	users.PUT("/:id/roles", middleware.RequirePermission("platform:roles:manage"), r.deps.User.AssignUserRoles)

	roles := platform.Group("/roles")
	roles.GET("", middleware.RequirePermission("platform:roles:list"), r.deps.Role.ListRoles)
	roles.POST("", middleware.RequirePermission("platform:roles:manage"), r.deps.Role.CreateRole)
	roles.GET("/:id", middleware.RequirePermission("platform:roles:list"), r.deps.Role.GetRole)
	roles.PUT("/:id", middleware.RequirePermission("platform:roles:manage"), r.deps.Role.UpdateRole)
	roles.DELETE("/:id", middleware.RequirePermission("platform:roles:manage"), r.deps.Role.DeleteRole)
	roles.PUT("/:id/permissions", middleware.RequirePermission("platform:roles:manage"), r.deps.Role.AssignRolePermissions)
	roles.GET("/:id/users", middleware.RequirePermission("platform:roles:list"), r.deps.Role.GetRoleUsers)

	platform.GET("/tenant-roles", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Role.ListSystemTenantRoles)
	platform.GET("/permissions", middleware.RequirePermission("platform:permissions:list"), r.deps.Permission.ListPermissions)
	platform.GET("/permissions/tree", middleware.RequirePermission("platform:permissions:list"), r.deps.Permission.GetPermissionTree)

	platform.GET("/tenants", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Tenant.ListTenants)
	platform.POST("/tenants", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.CreateTenant)
	platform.GET("/tenants/stats", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Tenant.GetTenantStats)
	platform.GET("/tenants/trends", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Tenant.GetTenantTrends)
	platform.GET("/tenants/:id", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Tenant.GetTenant)
	platform.PUT("/tenants/:id", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.UpdateTenant)
	platform.DELETE("/tenants/:id", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.DeleteTenant)
	platform.GET("/tenants/:id/members", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Tenant.ListMembers)
	platform.POST("/tenants/:id/members", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.AddMember)
	platform.DELETE("/tenants/:id/members/:userId", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.RemoveMember)
	platform.PUT("/tenants/:id/members/:userId/role", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.UpdateMemberRole)
	platform.POST("/tenants/:id/invitations", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.InviteToTenant)
	platform.GET("/tenants/:id/invitations", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), r.deps.Tenant.ListInvitations)
	platform.DELETE("/tenants/:id/invitations/:invId", middleware.RequirePermission("platform:tenants:manage"), r.deps.Tenant.CancelInvitation)

	impersonation := platform.Group("/impersonation")
	impersonation.Use(middleware.RequirePermission("platform:tenants:manage"))
	impersonation.POST("/requests", r.deps.Impersonation.CreateRequest)
	impersonation.GET("/requests", r.deps.Impersonation.ListMyRequests)
	impersonation.GET("/requests/:id", r.deps.Impersonation.GetRequest)
	impersonation.POST("/requests/:id/enter", r.deps.Impersonation.EnterTenant)
	impersonation.POST("/requests/:id/exit", r.deps.Impersonation.ExitTenant)
	impersonation.POST("/requests/:id/terminate", r.deps.Impersonation.TerminateSession)
	impersonation.POST("/requests/:id/cancel", r.deps.Impersonation.CancelRequest)
}
