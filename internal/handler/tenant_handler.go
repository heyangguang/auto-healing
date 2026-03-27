package handler

import (
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
)

// TenantHandler 租户处理器
type TenantHandler struct {
	repo     *repository.TenantRepository
	roleRepo *repository.RoleRepository
	userRepo *repository.UserRepository
	authSvc  *authService.Service
}

type TenantHandlerDeps struct {
	TenantRepo  *repository.TenantRepository
	RoleRepo    *repository.RoleRepository
	UserRepo    *repository.UserRepository
	AuthService *authService.Service
}

// NewTenantHandler 创建租户处理器
func NewTenantHandler(authSvc *authService.Service) *TenantHandler {
	return NewTenantHandlerWithDeps(TenantHandlerDeps{
		TenantRepo:  repository.NewTenantRepository(),
		RoleRepo:    repository.NewRoleRepository(),
		UserRepo:    repository.NewUserRepository(),
		AuthService: authSvc,
	})
}

func NewTenantHandlerWithDeps(deps TenantHandlerDeps) *TenantHandler {
	return &TenantHandler{
		repo:     deps.TenantRepo,
		roleRepo: deps.RoleRepo,
		userRepo: deps.UserRepo,
		authSvc:  deps.AuthService,
	}
}

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
	Status      string `json:"status"`
}
