package handler

import (
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListMembers 查询租户成员
func (h *TenantHandler) ListMembers(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}
	if _, err := h.repo.GetByID(c.Request.Context(), tenantID); err != nil {
		respondTenantLookupError(c, err)
		return
	}

	members, err := h.repo.ListMembers(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "查询租户成员失败")
		return
	}
	response.Success(c, members)
}

// SetTenantAdmin 为租户设置管理员
func (h *TenantHandler) SetTenantAdmin(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	var req setTenantAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：user_id 为必填")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	targetUser, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.BadRequest(c, "用户不存在")
		return
	}
	if targetUser.IsPlatformAdmin {
		response.BadRequest(c, "平台管理员不能设为租户管理员，请选择其他用户")
		return
	}

	adminRole, err := h.roleRepo.GetTenantRoleByName(c.Request.Context(), tenantID, "admin")
	if err != nil {
		response.InternalError(c, "查找 admin 角色失败")
		return
	}

	if err := h.assignTenantAdmin(c, userID, tenantID, adminRole.ID); err != nil {
		response.InternalError(c, "设置租户管理员失败")
		return
	}
	response.Message(c, "租户管理员设置成功")
}

func (h *TenantHandler) assignTenantAdmin(c *gin.Context, userID, tenantID, adminRoleID uuid.UUID) error {
	existingMember, err := h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return h.addTenantAdminMember(c, userID, tenantID, adminRoleID)
		}
		return fmt.Errorf("查询成员关系失败")
	}
	if existingMember != nil {
		if updateErr := h.repo.UpdateMemberRole(c.Request.Context(), userID, tenantID, adminRoleID); updateErr != nil {
			if errors.Is(updateErr, repository.ErrUserNotFound) {
				return h.addTenantAdminMember(c, userID, tenantID, adminRoleID)
			}
			return fmt.Errorf("升级管理员角色失败")
		}
		return nil
	}
	return h.addTenantAdminMember(c, userID, tenantID, adminRoleID)
}

func (h *TenantHandler) addTenantAdminMember(c *gin.Context, userID, tenantID, adminRoleID uuid.UUID) error {
	if addErr := h.repo.AddMember(c.Request.Context(), userID, tenantID, adminRoleID); addErr != nil {
		return fmt.Errorf("加入租户并设置管理员失败")
	}
	return nil
}

// UpdateMemberRole 变更成员角色（升级/降级）
func (h *TenantHandler) UpdateMemberRole(c *gin.Context) {
	tenantID, userID, roleID, ok := parseTenantMemberRoleParams(c)
	if !ok {
		return
	}

	if _, err := h.roleRepo.GetTenantRoleByID(c.Request.Context(), tenantID, roleID); err != nil {
		response.BadRequest(c, "只能分配当前租户可用的租户角色")
		return
	}
	if _, err := h.repo.GetMember(c.Request.Context(), userID, tenantID); err != nil {
		response.NotFound(c, "该用户不属于此租户")
		return
	}
	if err := h.repo.UpdateMemberRole(c.Request.Context(), userID, tenantID, roleID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			response.NotFound(c, "该用户不属于此租户")
			return
		}
		response.InternalError(c, "变更角色失败")
		return
	}
	response.Message(c, "角色变更成功")
}

func parseTenantMemberRoleParams(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	var req updateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：role_id 为必填")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return tenantID, userID, roleID, true
}

// CreateTenantUser 平台级创建租户用户
func (h *TenantHandler) CreateTenantUser(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		respondTenantLookupError(c, err)
		return
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "租户已禁用，无法创建用户")
		return
	}

	var req authService.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}
	if err := validateTenantScopedRegisterRequest(&req); err != nil {
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

// GetUserTenants 获取当前用户所属的租户列表
func (h *TenantHandler) GetUserTenants(c *gin.Context) {
	name := c.Query("name")
	if middleware.IsPlatformAdmin(c) && canListAllTenants(middleware.GetPermissions(c)) {
		tenants, _, err := h.repo.List(c.Request.Context(), name, query.StringFilter{}, query.StringFilter{}, "", 1, 1000)
		if err != nil {
			response.InternalError(c, "查询租户列表失败")
			return
		}
		response.Success(c, tenants)
		return
	}

	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}
	tenants, err := h.repo.GetUserTenants(c.Request.Context(), userID, name)
	if err != nil {
		response.InternalError(c, "查询用户租户失败")
		return
	}
	response.Success(c, tenants)
}

func canListAllTenants(permissions []string) bool {
	for _, permission := range permissions {
		if permission == "platform:tenants:manage" || permission == "platform:tenants:list" || permission == "*" {
			return true
		}
	}
	return false
}
