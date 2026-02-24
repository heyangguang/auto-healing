package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// ImpersonationKey gin.Context 中标记当前请求是否为 Impersonation
	ImpersonationKey          = "is_impersonating"
	ImpersonationRequestIDKey = "impersonation_request_id"
	ImpersonationTenantIDKey  = "impersonation_tenant_id"
)

// 缓存 impersonation_accessor 角色的权限列表（进程级缓存）
var (
	impersonationPermsOnce sync.Once
	impersonationPerms     []string
)

// loadImpersonationPermissions 从数据库加载 impersonation_accessor 角色的权限
func loadImpersonationPermissions() []string {
	impersonationPermsOnce.Do(func() {
		roleRepo := repository.NewRoleRepository()
		role, err := roleRepo.GetByName(context.Background(), "impersonation_accessor")
		if err != nil || role == nil {
			// 回退：使用空权限（安全第一，拒绝所有操作）
			impersonationPerms = []string{}
			return
		}
		codes := make([]string, len(role.Permissions))
		for i, p := range role.Permissions {
			codes[i] = p.Code
		}
		impersonationPerms = codes
	})
	return impersonationPerms
}

// ImpersonationMiddleware 验证 Impersonation 会话
// 当检测到请求携带 X-Impersonation=true 时：
// 1. 从 X-Impersonation-Request-ID 获取申请单 ID
// 2. 验证申请单 status=active 且未过期
// 3. 验证 requester_id 与当前用户匹配
// 4. 在 gin.Context 中设置 impersonation 标记
// 5. 用 impersonation_accessor 角色权限覆盖 JWT 中的 * 通配符
func ImpersonationMiddleware() gin.HandlerFunc {
	repo := repository.NewImpersonationRepository()

	return func(c *gin.Context) {
		// 默认不是 Impersonation
		c.Set(ImpersonationKey, false)

		impersonating := c.GetHeader("X-Impersonation")
		if impersonating != "true" {
			c.Next()
			return
		}

		// 只有平台管理员才能 Impersonate
		if !IsPlatformAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "只有平台管理员才能使用 Impersonation",
			})
			return
		}

		// 获取申请单 ID
		requestIDStr := c.GetHeader("X-Impersonation-Request-ID")
		if requestIDStr == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"code":    40000,
				"message": "Impersonation 缺少 X-Impersonation-Request-ID",
			})
			return
		}

		requestID, err := uuid.Parse(requestIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"code":    40000,
				"message": "无效的 X-Impersonation-Request-ID",
			})
			return
		}

		// 验证申请单
		req, err := repo.GetByID(c.Request.Context(), requestID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "Impersonation 申请不存在",
			})
			return
		}

		// 验证申请人是当前用户
		userID, _ := uuid.Parse(GetUserID(c))
		if req.RequesterID != userID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "Impersonation 申请与当前用户不匹配",
			})
			return
		}

		// 验证会话是否有效
		if !req.IsSessionValid() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "Impersonation 会话已过期或未激活",
			})
			return
		}

		// 验证请求的 X-Tenant-ID 与申请的目标租户一致
		tenantIDStr := c.GetHeader("X-Tenant-ID")
		if tenantIDStr != "" && tenantIDStr != req.TenantID.String() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "Impersonation 请求的租户与申请不匹配",
			})
			return
		}

		// 设置 Impersonation 标记到上下文
		c.Set(ImpersonationKey, true)
		c.Set(ImpersonationRequestIDKey, requestID.String())
		c.Set(ImpersonationTenantIDKey, req.TenantID.String())

		// 🔒 关键：用 impersonation_accessor 角色的权限覆盖 JWT 中的 * 通配符
		// 这样提权用户就无法审批自己的请求（因为没有 tenant:impersonation:approve 权限）
		perms := loadImpersonationPermissions()
		c.Set(PermissionsKey, perms)

		c.Next()
	}
}

// IsImpersonating 检查当前请求是否为 Impersonation
func IsImpersonating(c *gin.Context) bool {
	if v, exists := c.Get(ImpersonationKey); exists {
		return v.(bool)
	}
	return false
}
