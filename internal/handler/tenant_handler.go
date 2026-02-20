package handler

import (
	"errors"
	"strconv"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== 租户管理 Handler ====================
// 平台管理员（super_admin / is_platform_admin）专用

// TenantHandler 租户处理器
type TenantHandler struct {
	repo     *repository.TenantRepository
	roleRepo *repository.RoleRepository
	userRepo *repository.UserRepository
	authSvc  *authService.Service
}

// NewTenantHandler 创建租户处理器
func NewTenantHandler(authSvc *authService.Service) *TenantHandler {
	return &TenantHandler{
		repo:     repository.NewTenantRepository(),
		roleRepo: repository.NewRoleRepository(),
		userRepo: repository.NewUserRepository(),
		authSvc:  authSvc,
	}
}

// ==================== DTO ====================

type createTenantRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type setTenantAdminRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

type updateMemberRoleRequest struct {
	RoleID string `json:"role_id" binding:"required"`
}

type updateTenantRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Status      string `json:"status"` // active / disabled
}

// ==================== 租户 CRUD ====================

// ListTenants 租户列表
// GET /api/v1/platform/tenants?keyword=xxx&page=1&page_size=10
func (h *TenantHandler) ListTenants(c *gin.Context) {
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tenants, total, err := h.repo.List(c.Request.Context(), keyword, page, pageSize)
	if err != nil {
		response.InternalError(c, "查询租户列表失败")
		return
	}

	response.List(c, tenants, total, page, pageSize)
}

// GetTenant 获取租户详情
// GET /api/v1/platform/tenants/:id
func (h *TenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}

	response.Success(c, tenant)
}

// CreateTenant 创建租户
// POST /api/v1/platform/tenants
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：name 和 code 为必填")
		return
	}

	// 检查 code 唯一性
	existing, _ := h.repo.GetByCode(c.Request.Context(), req.Code)
	if existing != nil {
		response.Conflict(c, "租户编码已存在: "+req.Code)
		return
	}

	tenant := &model.Tenant{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Icon:        req.Icon,
		Status:      model.TenantStatusActive,
	}

	if err := h.repo.Create(c.Request.Context(), tenant); err != nil {
		response.InternalError(c, "创建租户失败")
		return
	}

	response.Created(c, tenant)
}

// UpdateTenant 更新租户
// PUT /api/v1/platform/tenants/:id
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}

	// 禁止修改 default 租户的 code
	if tenant.Code == "default" {
		var req updateTenantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Name != "" {
			tenant.Name = req.Name
		}
		if req.Description != "" {
			tenant.Description = req.Description
		}
		if req.Icon != "" {
			tenant.Icon = req.Icon
		}
		// default 租户不允许禁用
		if req.Status == model.TenantStatusDisabled {
			response.BadRequest(c, "默认租户不能被禁用")
			return
		}
	} else {
		var req updateTenantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Name != "" {
			tenant.Name = req.Name
		}
		if req.Description != "" {
			tenant.Description = req.Description
		}
		if req.Icon != "" {
			tenant.Icon = req.Icon
		}
		if req.Status != "" {
			tenant.Status = req.Status
		}
	}

	if err := h.repo.Update(c.Request.Context(), tenant); err != nil {
		response.InternalError(c, "更新租户失败")
		return
	}

	response.Success(c, tenant)
}

// DeleteTenant 删除租户
// DELETE /api/v1/platform/tenants/:id
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}

	// 禁止删除默认租户
	if tenant.Code == "default" {
		response.BadRequest(c, "默认租户不能被删除")
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除租户失败")
		return
	}

	response.Message(c, "租户已删除")
}

// ==================== 成员管理 ====================

// ListMembers 查询租户成员
// GET /api/v1/platform/tenants/:id/members
func (h *TenantHandler) ListMembers(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
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
// POST /api/v1/platform/tenants/:id/admin
// 若用户已在租户内则升级角色，否则加入租户并赋予 admin 角色
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

	// 互斥校验：platform_admin 不能兼任租户管理员
	targetUser, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.BadRequest(c, "用户不存在")
		return
	}
	for _, role := range targetUser.Roles {
		if role.Name == "platform_admin" {
			response.BadRequest(c, "平台管理员不能设为租户管理员，请选择其他用户")
			return
		}
	}

	// 查找 admin 系统角色
	adminRole, err := h.roleRepo.GetByName(c.Request.Context(), "admin")
	if err != nil {
		response.InternalError(c, "查找 admin 角色失败")
		return
	}

	// 判断用户是否已在租户内
	existingMember, err := h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		// 尝试用 gorm ErrRecordNotFound
		// 记录不存在则新增，其他错误返回 500
		if existingMember == nil {
			// record not found — 用户不在租户，新增关联
			if addErr := h.repo.AddMember(c.Request.Context(), userID, tenantID, adminRole.ID); addErr != nil {
				response.InternalError(c, "加入租户并设置管理员失败")
				return
			}
		} else {
			response.InternalError(c, "查询成员关系失败")
			return
		}
	} else if existingMember != nil {
		// 已在租户内，升级为 admin
		if updateErr := h.repo.UpdateMemberRole(c.Request.Context(), userID, tenantID, adminRole.ID); updateErr != nil {
			response.InternalError(c, "升级管理员角色失败")
			return
		}
	} else {
		// err != nil 且 existingMember == nil — record not found
		if addErr := h.repo.AddMember(c.Request.Context(), userID, tenantID, adminRole.ID); addErr != nil {
			response.InternalError(c, "加入租户并设置管理员失败")
			return
		}
	}

	response.Message(c, "租户管理员设置成功")
}

// UpdateMemberRole 变更成员角色（升级/降级）
// PUT /api/v1/platform/tenants/:id/members/:userId/role
func (h *TenantHandler) UpdateMemberRole(c *gin.Context) {
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

	var req updateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：role_id 为必填")
		return
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		response.BadRequest(c, "无效的角色 ID")
		return
	}

	// 验证角色存在
	_, err = h.roleRepo.GetByID(c.Request.Context(), roleID)
	if err != nil {
		response.BadRequest(c, "角色不存在")
		return
	}

	// 验证用户在租户内
	_, err = h.repo.GetMember(c.Request.Context(), userID, tenantID)
	if err != nil {
		response.NotFound(c, "该用户不属于此租户")
		return
	}

	if err := h.repo.UpdateMemberRole(c.Request.Context(), userID, tenantID, roleID); err != nil {
		response.InternalError(c, "变更角色失败")
		return
	}

	response.Message(c, "角色变更成功")
}

// CreateTenantUser 平台级创建租户用户
// POST /api/v1/platform/tenants/:id/users
// 创建一个普通用户（is_platform_admin=false）并自动关联到指定租户
func (h *TenantHandler) CreateTenantUser(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	// 验证租户存在且状态为 active
	tenant, err := h.repo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		response.NotFound(c, "租户不存在")
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

	// 强制关联到当前租户，确保不会被前端覆盖
	req.TenantID = &tenantID

	user, err := h.authSvc.Register(c.Request.Context(), &req)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Created(c, user)
}

// ==================== 用户租户列表 ====================

// GetUserTenants 获取当前用户所属的租户列表
// - 平台管理员（platform_admin 角色）返回所有租户
// - 普通用户只返回自己加入的租户
// 支持可选查询参数 search，对 name/code 做模糊匹配
// GET /api/v1/user/tenants?search=xxx
func (h *TenantHandler) GetUserTenants(c *gin.Context) {
	search := c.Query("search")

	// 平台管理员直接返回所有租户
	if middleware.IsPlatformAdmin(c) {
		tenants, _, err := h.repo.List(c.Request.Context(), search, 1, 1000)
		if err != nil {
			response.InternalError(c, "查询租户列表失败")
			return
		}
		response.Success(c, tenants)
		return
	}

	// 普通用户：返回自己加入的租户
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户 ID")
		return
	}

	tenants, err := h.repo.GetUserTenants(c.Request.Context(), userID, search)
	if err != nil {
		response.InternalError(c, "查询用户租户失败")
		return
	}

	response.Success(c, tenants)
}
