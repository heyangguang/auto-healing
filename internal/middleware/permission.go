package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// RequirePermission 权限检查中间件
func RequirePermission(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := GetPermissions(c)
		if permissions == nil {
			AbortPermissionsContextMissing(c)
			return
		}

		if hasPermission(permissions, requiredPermission) {
			c.Next()
			return
		}

		AbortPermissionDenied(c, requiredPermission, "all")
	}
}

// RequireAnyPermission 要求任一权限
func RequireAnyPermission(requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := GetPermissions(c)
		if permissions == nil {
			AbortPermissionsContextMissing(c)
			return
		}

		for _, required := range requiredPermissions {
			if hasPermission(permissions, required) {
				c.Next()
				return
			}
		}

		AbortPermissionDenied(c, requiredPermissions, "any")
	}
}

// RequireAllPermissions 要求所有权限
func RequireAllPermissions(requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := GetPermissions(c)
		if permissions == nil {
			AbortPermissionsContextMissing(c)
			return
		}

		for _, required := range requiredPermissions {
			if !hasPermission(permissions, required) {
				AbortPermissionDenied(c, requiredPermissions, "all")
				return
			}
		}

		c.Next()
	}
}

// HasPermission 检查是否有权限
func HasPermission(userPermissions []string, required string) bool {
	return hasPermission(userPermissions, required)
}

// hasPermission 检查是否有权限
func hasPermission(userPermissions []string, required string) bool {
	for _, p := range userPermissions {
		// 超级管理员通配符
		if p == "*" {
			return true
		}

		// 精确匹配
		if p == required {
			return true
		}

		// 模块级通配符 (e.g., "plugin:*" 匹配 "plugin:create")
		if strings.HasSuffix(p, ":*") {
			module := strings.TrimSuffix(p, ":*")
			if strings.HasPrefix(required, module+":") {
				return true
			}
		}
	}
	return false
}

// RequireRole 角色检查中间件
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles := GetRoles(c)
		if roles == nil {
			AbortRolesContextMissing(c)
			return
		}

		for _, role := range roles {
			if role == requiredRole {
				c.Next()
				return
			}
		}

		AbortRoleRequired(c, requiredRole)
	}
}
