package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterTenantRoutes(tenant *gin.RouterGroup) {
	users := tenant.Group("/users")
	users.GET("", middleware.RequirePermission("user:list"), r.deps.TenantUser.ListTenantUsers)
	users.GET("/simple", middleware.RequirePermission("user:list"), r.deps.TenantUser.ListSimpleUsers)
	users.POST("", middleware.RequirePermission("user:create"), r.deps.TenantUser.CreateTenantUser)
	users.GET("/:id", middleware.RequirePermission("user:list"), r.deps.TenantUser.GetTenantUser)
	users.PUT("/:id", middleware.RequirePermission("user:update"), r.deps.TenantUser.UpdateTenantUser)
	users.DELETE("/:id", middleware.RequirePermission("user:delete"), r.deps.TenantUser.DeleteTenantUser)
	users.POST("/:id/reset-password", middleware.RequirePermission("user:reset_password"), r.deps.TenantUser.ResetTenantUserPassword)
	users.PUT("/:id/roles", middleware.RequirePermission("role:assign"), r.deps.TenantUser.AssignTenantUserRoles)

	roles := tenant.Group("/roles")
	roles.GET("", middleware.RequirePermission("role:list"), r.deps.Role.ListTenantRoles)
	roles.GET("/:id", middleware.RequirePermission("role:list"), r.deps.Role.GetRole)
	roles.GET("/:id/users", middleware.RequirePermission("role:list"), r.deps.Role.GetTenantRoleUsers)
	roles.POST("", middleware.RequirePermission("role:create"), r.deps.Role.CreateRole)
	roles.PUT("/:id", middleware.RequirePermission("role:update"), r.deps.Role.UpdateRole)
	roles.DELETE("/:id", middleware.RequirePermission("role:delete"), r.deps.Role.DeleteRole)
	roles.PUT("/:id/permissions", middleware.RequirePermission("role:assign"), r.deps.Role.AssignRolePermissions)

	tenant.GET("/permissions", middleware.RequirePermission("role:list"), r.deps.Permission.ListPermissions)
	tenant.GET("/permissions/tree", middleware.RequirePermission("role:list"), r.deps.Permission.GetPermissionTree)

	impersonation := tenant.Group("/impersonation")
	impersonation.GET("/pending", middleware.RequirePermission("tenant:impersonation:view"), r.deps.Impersonation.ListPending)
	impersonation.GET("/history", middleware.RequirePermission("tenant:impersonation:view"), r.deps.Impersonation.ListHistory)
	impersonation.POST("/:id/approve", middleware.RequirePermission("tenant:impersonation:approve"), r.deps.Impersonation.Approve)
	impersonation.POST("/:id/reject", middleware.RequirePermission("tenant:impersonation:approve"), r.deps.Impersonation.Reject)

	settings := tenant.Group("/settings")
	settings.GET("/impersonation-approvers", middleware.RequirePermission("tenant:impersonation:approve"), r.deps.Impersonation.GetApprovers)
	settings.PUT("/impersonation-approvers", middleware.RequirePermission("tenant:impersonation:approve"), r.deps.Impersonation.SetApprovers)
}
