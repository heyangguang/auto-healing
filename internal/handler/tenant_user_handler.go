package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantUserHandler 租户级用户管理处理器
type TenantUserHandler struct {
	authSvc    *authService.Service
	tenantRepo *repository.TenantRepository
}

// NewTenantUserHandler 创建租户级用户处理器
func NewTenantUserHandler(authSvc *authService.Service) *TenantUserHandler {
	return &TenantUserHandler{
		authSvc:    authSvc,
		tenantRepo: repository.NewTenantRepository(),
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
	search := c.Query("search")
	status := c.DefaultQuery("status", "active") // 默认只返回活跃用户

	users, err := h.tenantRepo.ListSimpleMembers(c.Request.Context(), tenantID, search, status)
	if err != nil {
		response.InternalError(c, "获取简要用户列表失败")
		return
	}

	if users == nil {
		users = make([]repository.SimpleUser, 0)
	}
	response.Success(c, users)
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
