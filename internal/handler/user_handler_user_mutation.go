package handler

import (
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errDefaultPlatformRole = errors.New("获取默认角色失败")

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

	user, err := h.authSvc.Register(c.Request.Context(), &body.RegisterRequest)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	assignRoleID, err := h.platformRoleID(c, body.RoleID)
	if err != nil {
		if errors.Is(err, errDefaultPlatformRole) {
			response.InternalError(c, "获取默认平台角色失败")
		} else {
			response.BadRequest(c, err.Error())
		}
		return
	}
	if err := h.userRepo.AssignRoles(c.Request.Context(), user.ID, []uuid.UUID{assignRoleID}); err != nil {
		response.InternalError(c, "分配平台角色失败")
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
	if h.isProtectedPlatformAdmin(c, id) {
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
	if err := h.validateAssignedPlatformRoles(c, id, req.RoleIDs); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.userRepo.AssignRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		response.InternalError(c, "分配角色失败")
		return
	}

	userWithRoles, _ := h.userRepo.GetByID(c.Request.Context(), id)
	response.Success(c, userWithRoles)
}

func (h *UserHandler) platformRoleID(c *gin.Context, roleID *uuid.UUID) (uuid.UUID, error) {
	if roleID != nil {
		role, err := h.validatePlatformRole(c, roleID)
		if err != nil {
			return uuid.Nil, err
		}
		return role.ID, nil
	}

	platformAdminRole, err := h.roleRepo.GetByName(c.Request.Context(), "platform_admin")
	if err != nil {
		return uuid.Nil, errDefaultPlatformRole
	}
	return platformAdminRole.ID, nil
}

func (h *UserHandler) validatePlatformRole(c *gin.Context, roleID *uuid.UUID) (*model.Role, error) {
	if roleID == nil {
		return nil, nil
	}
	role, err := h.roleRepo.GetByID(c.Request.Context(), *roleID)
	if err != nil {
		return nil, fmt.Errorf("指定的角色不存在")
	}
	if role.Scope != "platform" {
		return nil, fmt.Errorf("只能分配平台级别角色")
	}
	return role, nil
}

func (h *UserHandler) validatePlatformAdminMutation(c *gin.Context, userID uuid.UUID, status string, targetRole *model.Role) error {
	if status != "" && status != "active" && h.isLastPlatformAdmin(c, userID) {
		return fmt.Errorf("系统中必须保留至少一个可用的平台管理员，无法禁用")
	}
	if targetRole != nil && targetRole.Name != "platform_admin" && h.isLastPlatformAdmin(c, userID) {
		return fmt.Errorf("系统中必须保留至少一个平台管理员，无法变更角色")
	}
	return nil
}

func applyPlatformUserUpdate(user *model.User, req *UpdateUserRequest) {
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != "" {
		user.Status = req.Status
	}
}

func (h *UserHandler) respondCreatedUser(c *gin.Context, userID uuid.UUID, fallback *model.User) {
	if userWithRoles, _ := h.userRepo.GetByID(c.Request.Context(), userID); userWithRoles != nil {
		response.Created(c, userWithRoles)
		return
	}
	response.Created(c, fallback)
}

func (h *UserHandler) respondUpdatedUser(c *gin.Context, userID uuid.UUID, fallback *model.User) {
	if userWithRoles, _ := h.userRepo.GetByID(c.Request.Context(), userID); userWithRoles != nil {
		response.Success(c, userWithRoles)
		return
	}
	response.Success(c, fallback)
}

func (h *UserHandler) validateAssignedPlatformRoles(c *gin.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if !h.isLastPlatformAdmin(c, userID) {
		return nil
	}
	for _, roleID := range roleIDs {
		if role, err := h.roleRepo.GetByID(c.Request.Context(), roleID); err == nil && role.Name == "platform_admin" {
			return nil
		}
	}
	return fmt.Errorf("系统中必须保留至少一个平台管理员，无法移除 platform_admin 角色")
}

func (h *UserHandler) isProtectedPlatformAdmin(c *gin.Context, userID uuid.UUID) bool {
	platformAdmins, _, err := h.userRepo.List(c.Request.Context(), &repository.UserListParams{
		Page:         1,
		PageSize:     2,
		PlatformOnly: true,
	})
	if err != nil || len(platformAdmins) > 1 {
		return false
	}
	for _, user := range platformAdmins {
		if user.ID == userID {
			return true
		}
	}
	return false
}

// isLastPlatformAdmin 判断指定用户是否是最后一个平台管理员
func (h *UserHandler) isLastPlatformAdmin(c *gin.Context, userID uuid.UUID) bool {
	platformAdmins, _, err := h.userRepo.List(c.Request.Context(), &repository.UserListParams{
		Page:         1,
		PageSize:     2,
		PlatformOnly: true,
		Status:       "active",
	})
	if err != nil || len(platformAdmins) > 1 {
		return false
	}
	return len(platformAdmins) == 1 && platformAdmins[0].ID == userID
}
