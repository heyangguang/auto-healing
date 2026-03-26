package middleware

import (
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequirePlatformAdmin 要求平台用户权限
func RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsPlatformAdmin(c) {
			abortForbidden(c, "此操作需要平台管理员权限", ErrorCodePlatformAdminRequired)
			return
		}
		reloadPlatformPermissions(c)
		c.Next()
	}
}

func reloadPlatformPermissions(c *gin.Context) {
	userID, err := uuid.Parse(GetUserID(c))
	if err != nil {
		return
	}
	permRepo := repository.NewPermissionRepository()
	if perms, permErr := permRepo.GetPlatformPermissionCodes(c.Request.Context(), userID); permErr == nil {
		c.Set(PermissionsKey, perms)
	}
}
