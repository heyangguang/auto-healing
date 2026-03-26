package handler

import (
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListTenantUsers 租户级用户列表
func (h *TenantUserHandler) ListTenantUsers(c *gin.Context) {
	tenantID, ok := requireTenantID(c, "TENANT_USER")
	if !ok {
		return
	}
	members, err := h.tenantRepo.ListMembers(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "获取租户用户列表失败")
		return
	}

	items := filterTenantUserItems(members, parseTenantUserRoleFilter(c))
	page, pageSize := tenantUserPagination(c)
	response.List(c, paginateTenantUserItems(items, page, pageSize), int64(len(items)), page, pageSize)
}

func filterTenantUserItems(members []model.UserTenantRole, filterRoleID uuid.UUID) []TenantUserItem {
	items := make([]TenantUserItem, 0, len(members))
	for _, member := range members {
		if filterRoleID != uuid.Nil && member.RoleID != filterRoleID {
			continue
		}
		items = append(items, tenantUserItemFromMember(member))
	}
	return items
}

// ListSimpleUsers 获取租户下简要用户列表
func (h *TenantUserHandler) ListSimpleUsers(c *gin.Context) {
	tenantID, ok := requireTenantID(c, "TENANT_USER")
	if !ok {
		return
	}
	users, err := h.tenantRepo.ListSimpleMembers(c.Request.Context(), tenantID, c.Query("name"), c.DefaultQuery("status", "active"))
	if err != nil {
		response.InternalError(c, "获取简要用户列表失败")
		return
	}
	if users == nil {
		users = make([]repository.SimpleUser, 0)
	}
	response.Success(c, users)
}

// GetTenantUser 获取当前租户下的用户详情
func (h *TenantUserHandler) GetTenantUser(c *gin.Context) {
	_, user, _, roles, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}
	response.Success(c, tenantUserView(user, roles))
}
