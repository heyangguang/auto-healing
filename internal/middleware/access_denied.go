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

type PermissionDeniedDetails struct {
	RequiredPermission  string   `json:"required_permission,omitempty"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
	Match               string   `json:"match,omitempty"`
}

type RoleRequiredDetails struct {
	RequiredRole string `json:"required_role"`
}

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
	details := PermissionDeniedDetails{Match: match}
	switch v := required.(type) {
	case string:
		details.RequiredPermission = v
	case []string:
		details.RequiredPermissions = v
	}
	if !hasPermissionDeniedDetails(details) {
		response.ErrorWithMetadata(
			c,
			http.StatusForbidden,
			response.CodeForbidden,
			"Permission denied",
			ErrorCodePermissionRequired,
			nil,
		)
		c.Abort()
		return
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

func hasPermissionDeniedDetails(details PermissionDeniedDetails) bool {
	return details.RequiredPermission != "" || len(details.RequiredPermissions) > 0 || details.Match != ""
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
		RoleRequiredDetails{RequiredRole: requiredRole},
	)
	c.Abort()
}
