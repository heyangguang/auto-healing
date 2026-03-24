package middleware

import (
	"net/http"
	"strings"

	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	AuthorizationHeader = "Authorization"
	BearerPrefix        = "Bearer "
	UserIDKey           = "user_id"
	UsernameKey         = "username"
	RolesKey            = "roles"
	PermissionsKey      = "permissions"
	TenantIDsKey        = "tenant_ids"        // 用户所属的租户列表
	DefaultTenantIDKey  = "default_tenant_id" // 用户的默认租户
)

// JWTAuth JWT认证中间件
func JWTAuth(jwtService *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthorizationHeader)

		// EventSource 无法自定义 Authorization header，仅在 SSE 端点允许 query token。
		if authHeader == "" && allowQueryToken(c) {
			if token := c.Query("token"); token != "" {
				authHeader = BearerPrefix + token
			}
		}

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Missing authorization header",
				},
			})
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid authorization header format",
				},
			})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, BearerPrefix)
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid or expired token",
				},
			})
			return
		}

		// 检查 Token 是否在黑名单中
		if jwtService.IsBlacklisted(claims.ID) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Token has been revoked",
				},
			})
			return
		}

		// 🔒 实时验证用户状态（禁用用户立即失效，无需等待 JWT 过期）
		userRepo := repository.NewUserRepository()
		uid, _ := uuid.Parse(claims.Subject)
		user, userErr := userRepo.GetByID(c.Request.Context(), uid)
		if userErr != nil || user.Status != "active" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "ACCOUNT_DISABLED",
					"message": "账户已被禁用，请联系管理员",
				},
			})
			return
		}

		// 将用户信息存入上下文
		c.Set(UserIDKey, claims.Subject)
		c.Set(UsernameKey, claims.Username)
		c.Set(RolesKey, claims.Roles)
		c.Set(PermissionsKey, claims.Permissions)
		c.Set(IsPlatformAdminKey, claims.IsPlatformAdmin)
		c.Set(TenantIDsKey, claims.TenantIDs)
		c.Set(DefaultTenantIDKey, claims.DefaultTenantID)

		c.Next()
	}
}

func allowQueryToken(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	path := c.FullPath()
	if path == "" {
		path = c.Request.URL.Path
	}
	return strings.HasSuffix(path, "/events") || strings.HasSuffix(path, "/stream")
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) string {
	if id, exists := c.Get(UserIDKey); exists {
		return id.(string)
	}
	return ""
}

// GetUsername 从上下文获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(UsernameKey); exists {
		return username.(string)
	}
	return ""
}

// GetRoles 从上下文获取角色
func GetRoles(c *gin.Context) []string {
	if roles, exists := c.Get(RolesKey); exists {
		return roles.([]string)
	}
	return nil
}

// GetPermissions 从上下文获取权限
func GetPermissions(c *gin.Context) []string {
	if permissions, exists := c.Get(PermissionsKey); exists {
		return permissions.([]string)
	}
	return nil
}
