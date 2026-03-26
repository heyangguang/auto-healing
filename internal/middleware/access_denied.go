package middleware

import (
	"net/http"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	ErrorCodePermissionsContextMissing = "PERMISSIONS_CONTEXT_MISSING"
	ErrorCodePermissionRequired        = "PERMISSION_REQUIRED"
	ErrorCodeRolesContextMissing       = "ROLES_CONTEXT_MISSING"
	ErrorCodeRoleRequired              = "ROLE_REQUIRED"
)

func AbortPermissionsContextMissing(c *gin.Context) {
	response.ErrorWithMetadata(
		c,
		http.StatusForbidden,
		response.CodeForbidden,
		"No permissions found",
		ErrorCodePermissionsContextMissing,
		nil,
	)
	c.Abort()
}

func AbortPermissionDenied(c *gin.Context, required any, match string) {
	details := gin.H{}
	switch v := required.(type) {
	case string:
		details["required_permission"] = v
	case []string:
		details["required_permissions"] = v
	}
	if match != "" {
		details["match"] = match
	}
	response.ErrorWithMetadata(
		c,
		http.StatusForbidden,
		response.CodeForbidden,
		"Permission denied",
		ErrorCodePermissionRequired,
		details,
	)
	c.Abort()
}

func AbortRolesContextMissing(c *gin.Context) {
	response.ErrorWithMetadata(
		c,
		http.StatusForbidden,
		response.CodeForbidden,
		"No roles found",
		ErrorCodeRolesContextMissing,
		nil,
	)
	c.Abort()
}

func AbortRoleRequired(c *gin.Context, requiredRole string) {
	response.ErrorWithMetadata(
		c,
		http.StatusForbidden,
		response.CodeForbidden,
		"Role required",
		ErrorCodeRoleRequired,
		gin.H{"required_role": requiredRole},
	)
	c.Abort()
}
