package httpapi

import (
	"errors"

	"github.com/company/auto-healing/internal/middleware"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errDefaultPlatformRole = errors.New("获取默认角色失败")
var errPlatformAdminStateCheck = errors.New("检查平台管理员状态失败")

// CreateUser 创建平台用户，支持选择平台角色（不传则默认 platform_admin）
func (h *UserHandler) CreateUser(c *gin.Context) {
	var body struct {
		authService.RegisterRequest
		RoleID *uuid.UUID `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}
	roleIDs, err := h.resolveCreateUserRoleIDs(c, body.RoleID, body.RegisterRequest.RoleIDs)
	if err != nil {
		if errors.Is(err, errDefaultPlatformRole) {
			response.InternalError(c, "获取默认平台角色失败")
		} else {
			response.BadRequest(c, err.Error())
		}
		return
	}
	body.RegisterRequest.RoleIDs = roleIDs
	body.RegisterRequest.TenantID = nil

	user, err := h.authSvc.Register(c.Request.Context(), &body.RegisterRequest)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}
	h.respondCreatedUser(c, user.ID, user)
}

// UpdateUser 更新用户
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	targetRole, err := h.validatePlatformRole(c, req.RoleID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.validatePlatformAdminMutation(c, id, req.Status, targetRole); err != nil {
		if errors.Is(err, errPlatformAdminStateCheck) {
			respondInternalError(c, "USER", "检查平台管理员状态失败", err)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	applyPlatformUserUpdate(user, &req)
	var targetRoleID *uuid.UUID
	if targetRole != nil {
		targetRoleID = &targetRole.ID
	}
	if err := h.userRepo.UpdatePlatformUserWithRole(c.Request.Context(), user, targetRoleID); err != nil {
		response.InternalError(c, "更新失败")
		return
	}
	h.respondUpdatedUser(c, user.ID, user)
}

// DeleteUser 删除平台管理员账号
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}
	if middleware.GetUserID(c) == id.String() {
		response.BadRequest(c, "不能删除自己的账户")
		return
	}
	protected, err := h.isProtectedPlatformAdmin(c, id)
	if err != nil {
		respondInternalError(c, "USER", "检查平台管理员状态失败", err)
		return
	}
	if protected {
		response.BadRequest(c, "系统中必须保留至少一个平台管理员，无法删除")
		return
	}
	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除失败")
		return
	}
	response.Message(c, "删除成功")
}

// ResetPassword 重置密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.authSvc.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		response.InternalError(c, "重置密码失败")
		return
	}
	response.Message(c, "密码重置成功")
}

// AssignUserRoles 分配用户角色
func (h *UserHandler) AssignUserRoles(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req AssignRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	roleIDs, err := h.validateAssignedPlatformRoles(c, id, req.RoleIDs)
	if err != nil {
		if errors.Is(err, errPlatformAdminStateCheck) {
			respondInternalError(c, "USER", "检查平台管理员状态失败", err)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.userRepo.AssignRoles(c.Request.Context(), id, roleIDs); err != nil {
		response.InternalError(c, "分配角色失败")
		return
	}

	userWithRoles, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "USER", "重新加载用户失败", err)
		return
	}
	response.Success(c, userWithRoles)
}
