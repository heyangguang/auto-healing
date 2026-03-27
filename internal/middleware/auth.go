package middleware

import (
	"errors"
	"net/http"
	"strings"

	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/logger"
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

const (
	ErrorCodeUnauthorized         = "UNAUTHORIZED"
	ErrorCodeAccountDisabled      = "ACCOUNT_DISABLED"
	ErrorCodeAccountNotFound      = "ACCOUNT_NOT_FOUND"
	ErrorCodeAccountLookup        = "ACCOUNT_LOOKUP_FAILED"
	ErrorCodeTokenBlacklistLookup = "TOKEN_BLACKLIST_LOOKUP_FAILED"
)

// JWTAuth JWT认证中间件
func JWTAuth(jwtService *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := resolveBearerToken(c)
		if !ok {
			return
		}
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			logger.Auth("TOKEN").Warn("鉴权失败: token 无效或已过期 | ip=%s path=%s err=%v", c.ClientIP(), c.Request.URL.Path, err)
			abortUnauthorized(c, "Invalid or expired token", ErrorCodeUnauthorized)
			return
		}

		// 检查 Token 是否在黑名单中
		isBlacklisted, blacklistErr := jwtService.IsBlacklisted(c.Request.Context(), claims.ID)
		if blacklistErr != nil {
			logger.Auth("TOKEN").Error("鉴权失败: token 黑名单校验失败 | user=%s ip=%s err=%v", claims.Subject, c.ClientIP(), blacklistErr)
			abortInternalError(c, "令牌撤销状态校验失败", ErrorCodeTokenBlacklistLookup)
			return
		}
		if isBlacklisted {
			logger.Auth("TOKEN").Warn("鉴权失败: token 已撤销 | user=%s ip=%s", claims.Subject, c.ClientIP())
			abortUnauthorized(c, "Token has been revoked", ErrorCodeUnauthorized)
			return
		}
		if !ensureActiveUser(c, claims.Subject) {
			return
		}
		setAuthContext(c, claims)
		c.Next()
	}
}

func resolveBearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader(AuthorizationHeader)
	if authHeader == "" {
		authHeader = queryTokenHeader(c)
	}
	if authHeader == "" {
		logger.Auth("TOKEN").Warn("鉴权失败: 缺少 Authorization 头 | ip=%s path=%s", c.ClientIP(), c.Request.URL.Path)
		abortUnauthorized(c, "Missing authorization header", ErrorCodeUnauthorized)
		return "", false
	}
	if !strings.HasPrefix(authHeader, BearerPrefix) {
		logger.Auth("TOKEN").Warn("鉴权失败: Authorization 格式非法 | ip=%s path=%s", c.ClientIP(), c.Request.URL.Path)
		abortUnauthorized(c, "Invalid authorization header format", ErrorCodeUnauthorized)
		return "", false
	}
	return strings.TrimPrefix(authHeader, BearerPrefix), true
}

func queryTokenHeader(c *gin.Context) string {
	if !allowQueryToken(c) {
		return ""
	}
	if token := c.Query("token"); token != "" {
		return BearerPrefix + token
	}
	return ""
}

func ensureActiveUser(c *gin.Context, subject string) bool {
	userRepo := accessrepo.NewUserRepository()
	uid, err := uuid.Parse(subject)
	if err != nil {
		logger.Auth("TOKEN").Warn("鉴权失败: token subject 非法 | user=%s ip=%s err=%v", subject, c.ClientIP(), err)
		abortUnauthorized(c, "Invalid token subject", ErrorCodeUnauthorized)
		return false
	}
	user, userErr := userRepo.GetByID(c.Request.Context(), uid)
	if userErr == nil && user.Status == "active" {
		return true
	}
	if userErr != nil && errors.Is(userErr, accessrepo.ErrUserNotFound) {
		logger.Auth("TOKEN").Warn("鉴权失败: 用户不存在 | user=%s ip=%s", subject, c.ClientIP())
		abortUnauthorized(c, "账户不存在或已失效，请重新登录", ErrorCodeAccountNotFound)
		return false
	}
	if userErr != nil {
		logger.Auth("TOKEN").Error("鉴权失败: 查询用户失败 | user=%s ip=%s err=%v", subject, c.ClientIP(), userErr)
		abortInternalError(c, "用户状态校验失败", ErrorCodeAccountLookup)
		return false
	}
	logger.Auth("TOKEN").Warn("鉴权失败: 用户不可用 | user=%s ip=%s err=%v", subject, c.ClientIP(), userErr)
	abortUnauthorized(c, "账户已被禁用，请联系管理员", ErrorCodeAccountDisabled)
	return false
}

func setAuthContext(c *gin.Context, claims *jwt.Claims) {
	c.Set(UserIDKey, claims.Subject)
	c.Set(UsernameKey, claims.Username)
	c.Set(RolesKey, claims.Roles)
	c.Set(PermissionsKey, claims.Permissions)
	c.Set(IsPlatformAdminKey, claims.IsPlatformAdmin)
	c.Set(TenantIDsKey, claims.TenantIDs)
	c.Set(DefaultTenantIDKey, claims.DefaultTenantID)
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
