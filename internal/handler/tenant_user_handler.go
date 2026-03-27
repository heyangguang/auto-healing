package handler

import (
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/model"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantUserHandler 租户级用户管理处理器
type TenantUserHandler struct {
	authSvc    *authService.Service
	tenantRepo *repository.TenantRepository
	userRepo   *repository.UserRepository
	roleRepo   *repository.RoleRepository
}

type TenantUserHandlerDeps struct {
	AuthService *authService.Service
	TenantRepo  *repository.TenantRepository
	UserRepo    *repository.UserRepository
	RoleRepo    *repository.RoleRepository
}

// NewTenantUserHandler 创建租户级用户处理器
func NewTenantUserHandler(authSvc *authService.Service) *TenantUserHandler {
	return NewTenantUserHandlerWithDeps(TenantUserHandlerDeps{
		AuthService: authSvc,
		TenantRepo:  repository.NewTenantRepository(),
		UserRepo:    repository.NewUserRepository(),
		RoleRepo:    repository.NewRoleRepository(),
	})
}

func NewTenantUserHandlerWithDeps(deps TenantUserHandlerDeps) *TenantUserHandler {
	return &TenantUserHandler{
		authSvc:    deps.AuthService,
		tenantRepo: deps.TenantRepo,
		userRepo:   deps.UserRepo,
		roleRepo:   deps.RoleRepo,
	}
}

// 辅助函数：解析数字参数
func parseQueryInt(val string) (int, error) {
	if val == "" {
		return 0, nil
	}
	return strconv.Atoi(val)
}

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

func (h *TenantUserHandler) loadTenantUser(c *gin.Context) (uuid.UUID, *model.User, *model.UserTenantRole, []model.Role, error) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, nil, nil, nil, err
	}

	tenantID, ok := requireTenantID(c, "TENANT_USER")
	if !ok {
		return uuid.Nil, nil, nil, nil, repository.ErrTenantContextRequired
	}
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

func tenantUserItemFromMember(member model.UserTenantRole) TenantUserItem {
	item := TenantUserItem{
		ID:          member.User.ID,
		Username:    member.User.Username,
		Email:       member.User.Email,
		DisplayName: member.User.DisplayName,
		Phone:       member.User.Phone,
		AvatarURL:   member.User.AvatarURL,
		Status:      member.User.Status,
		RoleName:    member.Role.DisplayName,
		RoleID:      member.RoleID,
		CreatedAt:   member.User.CreatedAt.Format("2006-01-02T15:04:05+08:00"),
	}
	if member.User.LastLoginAt != nil {
		lastLoginAt := member.User.LastLoginAt.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02T15:04:05+08:00")
		item.LastLoginAt = &lastLoginAt
	}
	return item
}

func parseTenantUserRoleFilter(c *gin.Context) uuid.UUID {
	roleIDStr := c.Query("role_id")
	if roleIDStr == "" {
		return uuid.Nil
	}
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return uuid.Nil
	}
	return roleID
}

func tenantUserPagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if p, err := parseQueryInt(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}
	if ps, err := parseQueryInt(c.DefaultQuery("page_size", "20")); err == nil && ps > 0 {
		pageSize = ps
	}
	return page, pageSize
}

func paginateTenantUserItems(items []TenantUserItem, page, pageSize int) []TenantUserItem {
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
		return make([]TenantUserItem, 0)
	}
	return paginatedItems
}
