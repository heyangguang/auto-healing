package httpapi

import (
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", r.deps.Auth.Login)
	auth.POST("/refresh", r.deps.Auth.RefreshToken)
	auth.GET("/invitation/:token", handler.ValidateInvitation)
	auth.POST("/register", handler.RegisterByInvitation(r.deps.Auth.GetAuthService()))

	authProtected := auth.Group("")
	authProtected.Use(middleware.JWTAuth(r.deps.Auth.GetJWTService()))
	authProtected.GET("/me",
		middleware.ImpersonationMiddleware(),
		handler.RequireAuthTenantContext(),
		r.deps.Auth.GetCurrentUser,
	)
	authProtected.GET("/profile", r.deps.Auth.GetProfile)
	authProtected.GET("/profile/login-history", r.deps.Auth.GetLoginHistory)
	authProtected.GET("/profile/activities",
		middleware.ImpersonationMiddleware(),
		handler.RequireAuthTenantContext(),
		r.deps.Auth.GetProfileActivities,
	)

	authAudited := authProtected.Group("")
	authAudited.Use(middleware.ImpersonationMiddleware())
	authAudited.Use(handler.OptionalAuthTenantContext())
	authAudited.Use(middleware.AuditMiddleware())
	authAudited.PUT("/profile", r.deps.Auth.UpdateProfile)
	authAudited.PUT("/password", r.deps.Auth.ChangePassword)
	authAudited.POST("/logout", r.deps.Auth.Logout)
}

func (r Registrar) RegisterCommonRoutes(common *gin.RouterGroup) {
	common.GET("/user/tenants", r.deps.Tenant.GetUserTenants)
}
