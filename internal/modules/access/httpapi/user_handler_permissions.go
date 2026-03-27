package httpapi

import (
	"strings"

	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ListPermissions 获取权限列表
func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	filter := accessrepo.PermissionFilter{
		Module: c.Query("module"),
		Name:   c.Query("name"),
		Code:   c.Query("code"),
	}
	perms, err := h.listScopedPermissions(c, filter)
	if err != nil {
		response.InternalError(c, "获取权限列表失败")
		return
	}
	response.Success(c, perms)
}

// GetPermissionTree 获取权限树
func (h *PermissionHandler) GetPermissionTree(c *gin.Context) {
	perms, err := h.listScopedPermissions(c, accessrepo.PermissionFilter{})
	if err != nil {
		response.InternalError(c, "获取权限列表失败")
		return
	}

	tree := make(map[string][]model.Permission)
	for _, permission := range perms {
		tree[permission.Module] = append(tree[permission.Module], permission)
	}
	response.Success(c, tree)
}

func (h *PermissionHandler) listScopedPermissions(c *gin.Context, filter accessrepo.PermissionFilter) ([]model.Permission, error) {
	if isTenantPermissionRequest(c) {
		return h.permRepo.ListTenantWithFilter(c.Request.Context(), filter)
	}
	return h.permRepo.ListWithFilter(c.Request.Context(), filter)
}

func isTenantPermissionRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.FullPath(), "/api/v1/tenant/")
}
