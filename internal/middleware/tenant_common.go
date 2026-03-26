package middleware

import (
	"fmt"

	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CommonTenantMiddleware 公共路由的租户上下文解析。
func CommonTenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
	tenantID, ok := resolveCommonRouteTenant(c)
	if !ok {
		return
	}
		if tenantID == uuid.Nil {
			c.Next()
			return
		}
		if !ensureActiveTenant(c, tenantID) {
			return
		}

		injectTenantContext(c, tenantID)
		if err := reloadTenantPermissions(c, tenantID); err != nil {
			abortInternalError(c, "刷新租户权限失败", ErrorCodeTenantPermissionReloadFailed)
			return
		}
		c.Next()
	}
}

const ErrorCodeTenantPermissionReloadFailed = "TENANT_PERMISSION_RELOAD_FAILED"

func resolveCommonRouteTenant(c *gin.Context) (uuid.UUID, bool) {
	if IsImpersonating(c) {
		return resolveImpersonationTenant(c)
	}
	if IsPlatformAdmin(c) {
		return uuid.Nil, true
	}

	tenantIDs, defaultTenantID := middlewareTenantClaims(c)
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
		return tenantID, true
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		abortBadRequest(c, "无效的 X-Tenant-ID 格式", ErrorCodeTenantIDInvalid)
		return uuid.Nil, false
	}
	if !ensureTenantMembership(c, tenantIDs, tenantIDStr) {
		return uuid.Nil, false
	}
	return tenantID, true
}

func middlewareTenantClaims(c *gin.Context) ([]string, string) {
	var tenantIDs []string
	if rawTenantIDs, exists := c.Get(TenantIDsKey); exists && rawTenantIDs != nil {
		tenantIDs, _ = rawTenantIDs.([]string)
	}

	defaultTenantID := ""
	if raw, exists := c.Get(DefaultTenantIDKey); exists && raw != nil {
		defaultTenantID, _ = raw.(string)
	}
	return tenantIDs, defaultTenantID
}

func reloadTenantPermissions(c *gin.Context, tenantID uuid.UUID) error {
	if IsImpersonating(c) {
		return nil
	}
	userID, err := uuid.Parse(GetUserID(c))
	if err != nil {
		return fmt.Errorf("解析当前用户失败: %w", err)
	}
	permRepo := repository.NewPermissionRepository()
	dbPerms, permErr := permRepo.GetTenantPermissionCodes(c.Request.Context(), userID, tenantID)
	if permErr != nil {
		return permErr
	}
	c.Set(PermissionsKey, dbPerms)
	return nil
}
