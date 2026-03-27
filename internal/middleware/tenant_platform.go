package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ErrorCodePlatformPermissionReloadFailed = "PLATFORM_PERMISSION_RELOAD_FAILED"

func RequirePlatformAdminWithDeps(deps RuntimeDeps) gin.HandlerFunc {
	permissionRepo := deps.requirePermissionRepo()
	return func(c *gin.Context) {
		if !IsPlatformAdmin(c) {
			abortForbidden(c, "此操作需要平台管理员权限", ErrorCodePlatformAdminRequired)
			return
		}
		if err := reloadPlatformPermissionsWithRepo(c, permissionRepo); err != nil {
			abortInternalError(c, "刷新平台权限失败", ErrorCodePlatformPermissionReloadFailed)
			return
		}
		c.Next()
	}
}

func reloadPlatformPermissionsWithRepo(c *gin.Context, permRepo permissionCodeRepository) error {
	userID, err := uuid.Parse(GetUserID(c))
	if err != nil {
		return fmt.Errorf("解析当前用户失败: %w", err)
	}
	perms, permErr := permRepo.GetPlatformPermissionCodes(c.Request.Context(), userID)
	if permErr != nil {
		return permErr
	}
	c.Set(PermissionsKey, perms)
	return nil
}
