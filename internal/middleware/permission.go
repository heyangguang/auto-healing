package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequirePermission 权限检查中间件
func RequirePermission(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := GetPermissions(c)
		if permissions == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "No permissions found",
				},
			})
			return
		}

		if hasPermission(permissions, requiredPermission) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":     "FORBIDDEN",
				"message":  "Permission denied",
				"required": requiredPermission,
			},
		})
	}
}

// RequireAnyPermission 要求任一权限
func RequireAnyPermission(requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := GetPermissions(c)
		if permissions == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "No permissions found",
				},
			})
			return
		}

		for _, required := range requiredPermissions {
			if hasPermission(permissions, required) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":     "FORBIDDEN",
				"message":  "Permission denied",
				"required": requiredPermissions,
			},
		})
	}
}

// RequireAllPermissions 要求所有权限
func RequireAllPermissions(requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions := GetPermissions(c)
		if permissions == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "No permissions found",
				},
			})
			return
		}

		for _, required := range requiredPermissions {
			if !hasPermission(permissions, required) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"code":     "FORBIDDEN",
						"message":  "Permission denied",
						"required": requiredPermissions,
					},
				})
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "No roles found",
				},
			})
			return
		}

		for _, role := range roles {
			if role == requiredRole {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":     "FORBIDDEN",
				"message":  "Role required",
				"required": requiredRole,
			},
		})
	}
}
