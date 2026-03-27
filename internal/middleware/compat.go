package middleware

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// AuditMiddleware 保留给兼容调用方；生产主路径应使用 AuditMiddlewareWithDeps。
func AuditMiddleware() gin.HandlerFunc {
	return AuditMiddlewareWithDeps(NewRuntimeDepsWithDB(database.DB))
}

// JWTAuth 保留给兼容调用方；生产主路径应使用 JWTAuthWithDeps。
func JWTAuth(jwtService *jwt.Service) gin.HandlerFunc {
	return JWTAuthWithDeps(jwtService, NewRuntimeDepsWithDB(database.DB))
}

// ImpersonationMiddleware 保留给兼容调用方；生产主路径应使用 ImpersonationMiddlewareWithDeps。
func ImpersonationMiddleware() gin.HandlerFunc {
	return ImpersonationMiddlewareWithDeps(NewRuntimeDepsWithDB(database.DB))
}

// CommonTenantMiddleware 保留给兼容调用方；生产主路径应使用 CommonTenantMiddlewareWithDeps。
func CommonTenantMiddleware() gin.HandlerFunc {
	return CommonTenantMiddlewareWithDeps(NewRuntimeDepsWithDB(database.DB))
}

// RequirePlatformAdmin 保留给兼容调用方；生产主路径应使用 RequirePlatformAdminWithDeps。
func RequirePlatformAdmin() gin.HandlerFunc {
	return RequirePlatformAdminWithDeps(NewRuntimeDepsWithDB(database.DB))
}

// TenantMiddleware 保留给兼容调用方；生产主路径应使用 TenantMiddlewareWithDeps。
func TenantMiddleware() gin.HandlerFunc {
	return TenantMiddlewareWithDeps(NewRuntimeDepsWithDB(database.DB))
}
