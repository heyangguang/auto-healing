package handler

import (
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

// ListPermissions 获取权限列表
func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	perms, err := h.permRepo.ListWithFilter(c.Request.Context(), repository.PermissionFilter{
		Module: c.Query("module"),
		Name:   c.Query("name"),
		Code:   c.Query("code"),
	})
	if err != nil {
		response.InternalError(c, "获取权限列表失败")
		return
	}
	response.Success(c, perms)
}

// GetPermissionTree 获取权限树
func (h *PermissionHandler) GetPermissionTree(c *gin.Context) {
	perms, err := h.permRepo.List(c.Request.Context())
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
