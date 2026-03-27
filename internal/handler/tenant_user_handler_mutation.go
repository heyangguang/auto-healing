package handler

import (
	"errors"

	"github.com/company/auto-healing/internal/middleware"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UpdateTenantUser 更新当前租户下的用户信息
func (h *TenantUserHandler) UpdateTenantUser(c *gin.Context) {
	tenantID, _, member, _, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if hasTenantUserGlobalMutation(req) {
		response.Forbidden(c, "租户侧不能修改用户全局资料或状态，只能调整当前租户角色")
		return
	}
	if req.RoleID == nil {
		response.BadRequest(c, "租户侧仅支持调整当前租户角色，role_id 为必填")
		return
	}

	targetRoleID, err := h.resolveTenantUserRoleUpdate(c, tenantID, req.RoleID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.tenantRepo.UpdateMemberRole(c.Request.Context(), member.UserID, tenantID, *targetRoleID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			response.NotFound(c, "用户不存在或不属于当前租户")
			return
		}
		response.InternalError(c, "更新失败")
		return
	}
	h.respondUpdatedTenantUser(c)
}

func (h *TenantUserHandler) resolveTenantUserRoleUpdate(c *gin.Context, tenantID uuid.UUID, roleID *uuid.UUID) (*uuid.UUID, error) {
	if roleID == nil {
		return nil, nil
	}
	role, err := h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, *roleID)
	if err != nil {
		return nil, businessError("只能分配当前租户可用的租户角色")
	}
	return &role.ID, nil
}

type businessError string

func (e businessError) Error() string {
	return string(e)
}

func hasTenantUserGlobalMutation(req UpdateUserRequest) bool {
	return req.DisplayName != "" || req.Phone != "" || req.Status != ""
}

func (h *TenantUserHandler) respondUpdatedTenantUser(c *gin.Context) {
	_, updatedUser, _, roles, err := h.loadTenantUser(c)
	if err != nil {
		response.InternalError(c, "重新加载用户信息失败")
		return
	}
	response.Success(c, tenantUserView(updatedUser, roles))
}

// DeleteTenantUser 从当前租户移除成员
func (h *TenantUserHandler) DeleteTenantUser(c *gin.Context) {
	tenantID, _, member, _, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}
	if err := h.ensureTenantAdminRemovable(c, tenantID, member.RoleID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.tenantRepo.RemoveMember(c.Request.Context(), member.UserID, tenantID); err != nil {
		response.InternalError(c, "移除成员失败")
		return
	}
	response.Message(c, "成员已移除")
}

func (h *TenantUserHandler) ensureTenantAdminRemovable(c *gin.Context, tenantID, roleID uuid.UUID) error {
	adminRole, err := h.roleRepo.GetTenantRoleByName(c.Request.Context(), tenantID, "admin")
	if err != nil || roleID != adminRole.ID {
		return nil
	}

	members, err := h.tenantRepo.ListMembers(c.Request.Context(), tenantID)
	if err != nil {
		return businessError("查询成员失败")
	}
	adminCount := 0
	for _, member := range members {
		if member.RoleID == adminRole.ID {
			adminCount++
		}
	}
	if adminCount <= 1 {
		return businessError("不能移除最后一个管理员，请先设置其他管理员")
	}
	return nil
}

// ResetTenantUserPassword 重置当前租户下用户的密码
func (h *TenantUserHandler) ResetTenantUserPassword(c *gin.Context) {
	_, _, _, _, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}
	response.Forbidden(c, "租户侧不能重置用户全局密码")
}

// AssignTenantUserRoles 为当前租户成员更新租户角色
func (h *TenantUserHandler) AssignTenantUserRoles(c *gin.Context) {
	tenantID, user, _, _, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}

	var req AssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if len(req.RoleIDs) != 1 {
		response.BadRequest(c, "租户用户当前仅支持分配一个租户角色")
		return
	}

	role, err := h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, req.RoleIDs[0])
	if err != nil {
		response.BadRequest(c, "只能分配当前租户可用的租户角色")
		return
	}

	if err := h.tenantRepo.UpdateMemberRole(c.Request.Context(), user.ID, tenantID, role.ID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			response.NotFound(c, "用户不存在或不属于当前租户")
			return
		}
		response.InternalError(c, "分配角色失败")
		return
	}
	h.respondUpdatedTenantUser(c)
}

// CreateTenantUser 租户级创建用户
func (h *TenantUserHandler) CreateTenantUser(c *gin.Context) {
	var req authService.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}
	if err := validateTenantScopedRegisterRequest(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tenantID, err := tenantIDFromMiddleware(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req.TenantID = &tenantID

	user, err := h.authSvc.Register(c.Request.Context(), &req)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}
	response.Created(c, user)
}

func validateTenantScopedRegisterRequest(req *authService.RegisterRequest) error {
	if len(req.RoleIDs) > 0 {
		return businessError("租户侧创建用户不能直接分配平台角色，请创建后在当前租户内分配租户角色")
	}
	return nil
}

func tenantIDFromMiddleware(c *gin.Context) (uuid.UUID, error) {
	tenantIDValue, exists := c.Get(middleware.TenantIDKey)
	if !exists {
		return uuid.Nil, businessError("无法获取租户上下文")
	}
	tenantID, err := uuid.Parse(tenantIDValue.(string))
	if err != nil {
		return uuid.Nil, businessError("租户ID格式错误")
	}
	return tenantID, nil
}
