package handler

import (
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AddMember 添加已有用户到租户
func (h *TenantHandler) AddMember(c *gin.Context) {
	tenantID, userID, roleID, ok := parseAddMemberParams(c)
	if !ok {
		return
	}

	tenant, role, targetUser, ok := h.validateAddMemberRequest(c, tenantID, userID, roleID)
	if !ok {
		return
	}
	_ = tenant
	_ = targetUser
	_ = role

	existingMember, _ := h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if existingMember != nil {
		response.Conflict(c, "该用户已是租户成员")
		return
	}
	if err := h.repo.AddMember(c.Request.Context(), userID, tenantID, roleID); err != nil {
		response.InternalError(c, "添加成员失败")
		return
	}
	response.Message(c, "成员添加成功")
}

func parseAddMemberParams(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：user_id 和 role_id 为必填")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return tenantID, userID, roleID, true
}

func (h *TenantHandler) validateAddMemberRequest(c *gin.Context, tenantID, userID, roleID uuid.UUID) (*model.Tenant, *model.Role, *model.User, bool) {
	tenant, err := h.repo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return nil, nil, nil, false
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "租户已禁用，无法添加成员")
		return nil, nil, nil, false
	}

	targetUser, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return nil, nil, nil, false
	}
	if targetUser.IsPlatformAdmin {
		response.BadRequest(c, "平台管理员不能加入租户，请选择其他用户")
		return nil, nil, nil, false
	}

	role, err := h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, roleID)
	if err != nil {
		response.BadRequest(c, "角色不存在")
		return nil, nil, nil, false
	}
	if !isValidTenantRole(role) {
		response.BadRequest(c, "只能分配系统级租户角色（如管理员、运维人员、只读用户等）")
		return nil, nil, nil, false
	}
	return tenant, role, targetUser, true
}

// RemoveMember 从租户移除成员
func (h *TenantHandler) RemoveMember(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	member, err := h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if err != nil {
		response.NotFound(c, "该用户不属于此租户")
		return
	}
	if err := h.validateTenantAdminRemoval(c, tenantID, member.RoleID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.repo.RemoveMember(c.Request.Context(), userID, tenantID); err != nil {
		response.InternalError(c, "移除成员失败")
		return
	}
	response.Message(c, "成员已移除")
}

func (h *TenantHandler) validateTenantAdminRemoval(c *gin.Context, tenantID, roleID uuid.UUID) error {
	adminRole, err := h.roleRepo.GetTenantRoleByName(c.Request.Context(), tenantID, "admin")
	if err != nil || roleID != adminRole.ID {
		return nil
	}
	members, err := h.repo.ListMembers(c.Request.Context(), tenantID)
	if err != nil {
		return fmt.Errorf("查询成员失败")
	}
	adminCount := 0
	for _, member := range members {
		if member.RoleID == adminRole.ID {
			adminCount++
		}
	}
	if adminCount <= 1 {
		return fmt.Errorf("不能移除最后一个管理员，请先设置其他管理员")
	}
	return nil
}
