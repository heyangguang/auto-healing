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

// NewTenantHandler 创建租户处理器
func NewTenantHandler(authSvc *authService.Service) *TenantHandler {
	return &TenantHandler{
		repo:     repository.NewTenantRepository(),
		roleRepo: repository.NewRoleRepository(),
		userRepo: repository.NewUserRepository(),
		authSvc:  authSvc,
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
