package httpapi

import (
	"context"

	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	engagementservice "github.com/company/auto-healing/internal/modules/engagement/service"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
)

// TenantHandler 租户处理器
type TenantHandler struct {
	repo     *accessrepo.TenantRepository
	roleRepo *accessrepo.RoleRepository
	userRepo *accessrepo.UserRepository
	authSvc  *authService.Service
	invRepo  *accessrepo.InvitationRepository
	settings *settingsrepo.PlatformSettingsRepository
	emailSvc invitationEmailService
}

type TenantHandlerDeps struct {
	TenantRepo     *accessrepo.TenantRepository
	RoleRepo       *accessrepo.RoleRepository
	UserRepo       *accessrepo.UserRepository
	AuthService    *authService.Service
	InvitationRepo *accessrepo.InvitationRepository
	SettingsRepo   *settingsrepo.PlatformSettingsRepository
	EmailService   invitationEmailService
}

func NewTenantHandlerWithDeps(deps TenantHandlerDeps) *TenantHandler {
	return &TenantHandler{
		repo:     deps.TenantRepo,
		roleRepo: deps.RoleRepo,
		userRepo: deps.UserRepo,
		authSvc:  deps.AuthService,
		invRepo:  deps.InvitationRepo,
		settings: deps.SettingsRepo,
		emailSvc: deps.EmailService,
	}
}

type invitationEmailService interface {
	IsConfigured(ctx context.Context) bool
	SendInvitationEmail(ctx context.Context, to, tenantName, roleName, inviteURL string) error
}

var _ invitationEmailService = (*engagementservice.PlatformEmailService)(nil)

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
