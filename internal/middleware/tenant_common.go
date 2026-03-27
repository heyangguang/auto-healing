package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CommonTenantMiddleware 公共路由的租户上下文解析。
func CommonTenantMiddleware() gin.HandlerFunc {
	return CommonTenantMiddlewareWithDeps(NewRuntimeDeps())
}

func CommonTenantMiddlewareWithDeps(deps RuntimeDeps) gin.HandlerFunc {
	deps = deps.withDefaults()
	return func(c *gin.Context) {
		tenantID, ok := resolveCommonRouteTenantWithRepo(c, deps.TenantRepo)
		if !ok {
			return
		}
		if tenantID == uuid.Nil {
			if err := reloadCommonRoutePermissionsWithRepo(c, deps.PermissionRepo); err != nil {
				abortInternalError(c, "刷新平台权限失败", ErrorCodePlatformPermissionReloadFailed)
				return
			}
			c.Next()
			return
		}
		if !ensureActiveTenantWithRepo(c, deps.TenantRepo, tenantID) {
			return
		}

		injectTenantContext(c, tenantID)
		if err := reloadTenantPermissions(c, deps.PermissionRepo, tenantID); err != nil {
			abortInternalError(c, "刷新租户权限失败", ErrorCodeTenantPermissionReloadFailed)
			return
		}
		c.Next()
	}
}

const ErrorCodeTenantPermissionReloadFailed = "TENANT_PERMISSION_RELOAD_FAILED"

func reloadCommonRoutePermissions(c *gin.Context) error {
	return reloadCommonRoutePermissionsWithRepo(c, NewRuntimeDeps().PermissionRepo)
}

func reloadCommonRoutePermissionsWithRepo(c *gin.Context, permRepo permissionCodeRepository) error {
	if !IsPlatformAdmin(c) || IsImpersonating(c) {
		return nil
	}
	return reloadPlatformPermissionsWithRepo(c, permRepo)
}

func resolveCommonRouteTenant(c *gin.Context) (uuid.UUID, bool) {
	return resolveCommonRouteTenantWithRepo(c, NewRuntimeDeps().TenantRepo)
}

func resolveCommonRouteTenantWithRepo(c *gin.Context, tenantRepo userTenantLister) (uuid.UUID, bool) {
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
			if !ensureTenantMembership(c, tenantRepo, tenantIDs, defaultTenantID) {
				return uuid.Nil, false
			}
			return tenantID, true
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		abortBadRequest(c, "无效的 X-Tenant-ID 格式", ErrorCodeTenantIDInvalid)
		return uuid.Nil, false
	}
	if !ensureTenantMembership(c, tenantRepo, tenantIDs, tenantIDStr) {
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

func reloadTenantPermissions(c *gin.Context, permRepo permissionCodeRepository, tenantID uuid.UUID) error {
	if IsImpersonating(c) {
		return nil
	}
	userID, err := uuid.Parse(GetUserID(c))
	if err != nil {
		return fmt.Errorf("解析当前用户失败: %w", err)
	}
	dbPerms, permErr := permRepo.GetTenantPermissionCodes(c.Request.Context(), userID, tenantID)
	if permErr != nil {
		return permErr
	}
	c.Set(PermissionsKey, dbPerms)
	return nil
}
