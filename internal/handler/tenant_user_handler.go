package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantUserHandler 租户级用户管理处理器
type TenantUserHandler struct {
	authSvc    *authService.Service
	tenantRepo *repository.TenantRepository
	userRepo   *repository.UserRepository
	roleRepo   *repository.RoleRepository
}

// NewTenantUserHandler 创建租户级用户处理器
func NewTenantUserHandler(authSvc *authService.Service) *TenantUserHandler {
	return &TenantUserHandler{
		authSvc:    authSvc,
		tenantRepo: repository.NewTenantRepository(),
		userRepo:   repository.NewUserRepository(),
		roleRepo:   repository.NewRoleRepository(),
	}
}

// ListTenantUsers 租户级用户列表
// GET /api/v1/tenant/users
// 自动从 context 获取当前租户 ID，列出该租户下的所有用户（带角色信息）
func (h *TenantUserHandler) ListTenantUsers(c *gin.Context) {
	tenantID := repository.TenantIDFromContext(c.Request.Context())

	members, err := h.tenantRepo.ListMembers(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "获取租户用户列表失败")
		return
	}

	roleIDStr := c.Query("role_id")
	var filterRoleID uuid.UUID
	if roleIDStr != "" {
		if id, err := uuid.Parse(roleIDStr); err == nil {
			filterRoleID = id
		}
	}

	// 转换为前端需要的格式（与 platform/users 返回结构对齐）
	type TenantUserItem struct {
		ID          uuid.UUID `json:"id"`
		Username    string    `json:"username"`
		Email       string    `json:"email"`
		DisplayName string    `json:"display_name"`
		Phone       string    `json:"phone,omitempty"`
		AvatarURL   string    `json:"avatar_url,omitempty"`
		Status      string    `json:"status"`
		RoleName    string    `json:"role_name"`
		RoleID      uuid.UUID `json:"role_id"`
		CreatedAt   string    `json:"created_at"`
		LastLoginAt *string   `json:"last_login_at,omitempty"`
	}

	var items []TenantUserItem
	for _, m := range members {
		// role_id 过滤
		if filterRoleID != uuid.Nil && m.RoleID != filterRoleID {
			continue
		}

		item := TenantUserItem{
			ID:          m.User.ID,
			Username:    m.User.Username,
			Email:       m.User.Email,
			DisplayName: m.User.DisplayName,
			Phone:       m.User.Phone,
			AvatarURL:   m.User.AvatarURL,
			Status:      m.User.Status,
			RoleName:    m.Role.DisplayName,
			RoleID:      m.RoleID,
			CreatedAt:   m.User.CreatedAt.Format("2006-01-02T15:04:05+08:00"),
		}
		if m.User.LastLoginAt != nil {
			t := m.User.LastLoginAt.Format("2006-01-02T15:04:05+08:00")
			item.LastLoginAt = &t
		}
		items = append(items, item)
	}

	// 内存分页
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	page := 1
	pageSize := 20
	if p, err := parseQueryInt(pageStr); err == nil && p > 0 {
		page = p
	}
	if ps, err := parseQueryInt(pageSizeStr); err == nil && ps > 0 {
		pageSize = ps
	}

	total := len(items)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	paginatedItems := items[start:end]

	if paginatedItems == nil {
		paginatedItems = make([]TenantUserItem, 0)
	}
	response.List(c, paginatedItems, int64(total), page, pageSize)
}

// ListSimpleUsers 获取租户下简要用户列表（轻量接口，用于下拉选择）
func (h *TenantUserHandler) ListSimpleUsers(c *gin.Context) {
	tenantID := repository.TenantIDFromContext(c.Request.Context())
	name := c.Query("name")
	status := c.DefaultQuery("status", "active") // 默认只返回活跃用户

	users, err := h.tenantRepo.ListSimpleMembers(c.Request.Context(), tenantID, name, status)
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

// UpdateTenantUser 更新当前租户下的用户信息
func (h *TenantUserHandler) UpdateTenantUser(c *gin.Context) {
	tenantID, user, _, _, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	var targetRole *model.Role
	if req.RoleID != nil {
		role, err := h.roleRepo.GetByID(c.Request.Context(), *req.RoleID)
		if err != nil || !isAssignableTenantRole(role, tenantID) {
			response.BadRequest(c, "只能分配当前租户可用的租户角色")
			return
		}
		targetRole = role
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Status != "" {
		user.Status = req.Status
	}

	if err := database.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		if targetRole != nil {
			return tx.Model(&model.UserTenantRole{}).
				Where("user_id = ? AND tenant_id = ?", user.ID, tenantID).
				Update("role_id", targetRole.ID).Error
		}
		return nil
	}); err != nil {
		response.InternalError(c, "更新失败")
		return
	}

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

	adminRole, err := h.roleRepo.GetByName(c.Request.Context(), "admin")
	if err == nil && member.RoleID == adminRole.ID {
		members, listErr := h.tenantRepo.ListMembers(c.Request.Context(), tenantID)
		if listErr != nil {
			response.InternalError(c, "查询成员失败")
			return
		}
		adminCount := 0
		for _, m := range members {
			if m.RoleID == adminRole.ID {
				adminCount++
			}
		}
		if adminCount <= 1 {
			response.BadRequest(c, "不能移除最后一个管理员，请先设置其他管理员")
			return
		}
	}

	if err := h.tenantRepo.RemoveMember(c.Request.Context(), member.UserID, tenantID); err != nil {
		response.InternalError(c, "移除成员失败")
		return
	}

	response.Message(c, "成员已移除")
}

// ResetTenantUserPassword 重置当前租户下用户的密码
func (h *TenantUserHandler) ResetTenantUserPassword(c *gin.Context) {
	_, user, _, _, err := h.loadTenantUser(c)
	if err != nil {
		response.NotFound(c, "用户不存在或不属于当前租户")
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.authSvc.ResetPassword(c.Request.Context(), user.ID, req.NewPassword); err != nil {
		response.InternalError(c, "重置密码失败")
		return
	}

	response.Message(c, "密码重置成功")
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

	role, err := h.roleRepo.GetByID(c.Request.Context(), req.RoleIDs[0])
	if err != nil || !isAssignableTenantRole(role, tenantID) {
		response.BadRequest(c, "只能分配当前租户可用的租户角色")
		return
	}

	if err := h.tenantRepo.UpdateMemberRole(c.Request.Context(), user.ID, tenantID, role.ID); err != nil {
		response.InternalError(c, "分配角色失败")
		return
	}

	_, updatedUser, _, roles, err := h.loadTenantUser(c)
	if err != nil {
		response.InternalError(c, "重新加载用户信息失败")
		return
	}
	response.Success(c, tenantUserView(updatedUser, roles))
}

// 辅助函数：解析数字参数
func parseQueryInt(val string) (int, error) {
	if val == "" {
		return 0, nil
	}
	return strconv.Atoi(val)
}

// CreateTenantUser 租户级创建用户
// 从 TenantMiddleware 获取当前租户 ID，自动将用户关联到当前租户
func (h *TenantUserHandler) CreateTenantUser(c *gin.Context) {
	var req authService.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	// 从 TenantMiddleware 获取当前租户 ID
	tenantIDStr, exists := c.Get(middleware.TenantIDKey)
	if !exists {
		response.InternalError(c, "无法获取租户上下文")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		response.InternalError(c, "租户ID格式错误")
		return
	}

	// 自动关联到当前租户
	req.TenantID = &tenantID

	user, err := h.authSvc.Register(c.Request.Context(), &req)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Created(c, user)
}

func (h *TenantUserHandler) loadTenantUser(c *gin.Context) (uuid.UUID, *model.User, *model.UserTenantRole, []model.Role, error) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, nil, nil, nil, err
	}

	tenantID := repository.TenantIDFromContext(c.Request.Context())
	member, err := h.tenantRepo.GetMember(c.Request.Context(), userID, tenantID)
	if err != nil {
		return uuid.Nil, nil, nil, nil, err
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		return uuid.Nil, nil, nil, nil, err
	}

	roles, err := h.tenantRepo.GetUserTenantRoles(c.Request.Context(), userID, tenantID)
	if err != nil {
		return uuid.Nil, nil, nil, nil, err
	}

	return tenantID, user, member, roles, nil
}

func tenantUserView(user *model.User, roles []model.Role) *model.User {
	if user == nil {
		return nil
	}
	copy := *user
	copy.Roles = roles
	return &copy
}

func isAssignableTenantRole(role *model.Role, tenantID uuid.UUID) bool {
	if role == nil || role.Scope != "tenant" {
		return false
	}
	if role.TenantID == nil {
		return isValidTenantRole(role)
	}
	return *role.TenantID == tenantID
}
