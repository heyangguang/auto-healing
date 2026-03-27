package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func requireRepositoryValidatePermission(c *gin.Context) bool {
	if middleware.HasPermission(middleware.GetPermissions(c), "repository:validate") {
		return true
	}
	middleware.AbortPermissionDenied(c, "repository:validate", "all")
	return false
}
