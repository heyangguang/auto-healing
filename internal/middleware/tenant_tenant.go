package middleware

import (
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ErrorCodeTenantMembershipLookupFailed = "TENANT_MEMBERSHIP_LOOKUP_FAILED"

// TenantMiddleware 租户上下文中间件
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDs, defaultTenantID := middlewareTenantClaims(c)

		tenantID, ok := resolveTenantRouteTenant(c, tenantIDs, defaultTenantID)
		if !ok {
			return
		}
		if !ensureActiveTenant(c, tenantID) {
			return
		}

		injectTenantContext(c, tenantID)
		if !IsPlatformAdmin(c) && !IsImpersonating(c) {
			if err := reloadTenantPermissions(c, tenantID); err != nil {
				abortInternalError(c, "刷新租户权限失败", ErrorCodeTenantPermissionReloadFailed)
				return
			}
		}
		c.Next()
	}
}

func resolveTenantRouteTenant(c *gin.Context, tenantIDs []string, defaultTenantID string) (uuid.UUID, bool) {
	if IsPlatformAdmin(c) {
		return resolveImpersonationTenant(c)
	}
	return resolveRegularTenant(c, tenantIDs, defaultTenantID)
}

func resolveImpersonationTenant(c *gin.Context) (uuid.UUID, bool) {
	if !IsImpersonating(c) {
		abortForbidden(c, "此接口为租户级资源，平台管理员需通过临时提权（Impersonation）后才能访问", ErrorCodeImpersonationRequired)
		return uuid.Nil, false
	}

	impTenantIDStr, _ := c.Get(ImpersonationTenantIDKey)
	if impTenantIDStr == nil || impTenantIDStr.(string) == "" {
		abortForbidden(c, "Impersonation 会话缺少租户信息", ErrorCodeImpersonationTenantMissing)
		return uuid.Nil, false
	}
	tenantID, err := uuid.Parse(impTenantIDStr.(string))
	if err != nil {
		abortBadRequest(c, "Impersonation 租户 ID 格式无效", ErrorCodeImpersonationTenantInvalid)
		return uuid.Nil, false
	}
	return tenantID, true
}

func resolveRegularTenant(c *gin.Context, tenantIDs []string, defaultTenantID string) (uuid.UUID, bool) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		if defaultTenantID == "" {
			abortForbidden(c, "用户未分配任何租户，请联系管理员", ErrorCodeTenantUnassigned)
			return uuid.Nil, false
		}
		tenantID, err := uuid.Parse(defaultTenantID)
		if err != nil {
			abortForbidden(c, "默认租户无效，请重新登录", ErrorCodeDefaultTenantInvalid)
			return uuid.Nil, false
		}
		if !ensureTenantMembership(c, tenantIDs, defaultTenantID) {
			return uuid.Nil, false
		}
		return tenantID, true
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		abortBadRequest(c, "无效的 X-Tenant-ID 格式", ErrorCodeTenantIDInvalid)
		return uuid.Nil, false
	}
	if ensureTenantMembership(c, tenantIDs, tenantIDStr) {
		return tenantID, true
	}
	return uuid.Nil, false
}

func ensureTenantMembership(c *gin.Context, tenantIDs []string, tenantIDStr string) bool {
	userID, err := uuid.Parse(GetUserID(c))
	if err != nil {
		abortForbidden(c, "无权访问该租户", ErrorCodeTenantAccessDenied)
		return false
	}

	tenantRepo := accessrepo.NewTenantRepository()
	dbTenants, dbErr := tenantRepo.GetUserTenants(c.Request.Context(), userID, "")
	if dbErr != nil {
		abortInternalError(c, "租户成员关系校验失败", ErrorCodeTenantMembershipLookupFailed)
		return false
	}
	if !containsTenantByID(dbTenants, tenantIDStr) {
		abortForbidden(c, "无权访问该租户", ErrorCodeTenantAccessDenied)
		return false
	}
	if !contains(tenantIDs, tenantIDStr) {
		c.Header("X-Refresh-Token", "true")
	}
	return true
}
