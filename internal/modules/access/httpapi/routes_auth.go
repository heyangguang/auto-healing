package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", r.deps.Auth.Login)
	auth.POST("/refresh", r.deps.Auth.RefreshToken)
	auth.GET("/invitation/:token", r.deps.Tenant.ValidateInvitation)
	auth.POST("/register", r.deps.Tenant.RegisterByInvitation)

	authProtected := auth.Group("")
	authProtected.Use(middleware.JWTAuthWithDeps(r.deps.Auth.GetJWTService(), r.deps.Middleware))
	authProtected.GET("/me",
		middleware.ImpersonationMiddlewareWithDeps(r.deps.Middleware),
		RequireAuthTenantContext(r.deps.Tenant.repo),
		r.deps.Auth.GetCurrentUser,
	)
	authProtected.GET("/profile", r.deps.Auth.GetProfile)
	authProtected.GET("/profile/login-history", r.deps.Auth.GetLoginHistory)
	authProtected.GET("/profile/activities",
		middleware.ImpersonationMiddlewareWithDeps(r.deps.Middleware),
		RequireAuthTenantContext(r.deps.Tenant.repo),
		r.deps.Auth.GetProfileActivities,
	)

	authAudited := authProtected.Group("")
	authAudited.Use(middleware.ImpersonationMiddlewareWithDeps(r.deps.Middleware))
	authAudited.Use(OptionalAuthTenantContext(r.deps.Tenant.repo))
	authAudited.Use(middleware.AuditMiddlewareWithDeps(r.deps.Middleware))
	authAudited.PUT("/profile", r.deps.Auth.UpdateProfile)
	authAudited.PUT("/password", r.deps.Auth.ChangePassword)
	authAudited.POST("/logout", r.deps.Auth.Logout)
}

func (r Registrar) RegisterCommonRoutes(common *gin.RouterGroup) {
	common.GET("/user/tenants", r.deps.Tenant.GetUserTenants)
}
