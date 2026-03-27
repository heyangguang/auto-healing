package httpapi

import (
	"time"

	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/google/uuid"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authSvc           *authService.Service
	jwtSvc            *jwt.Service
	auditRepo         *auditrepo.AuditLogRepository
	platformAuditRepo *auditrepo.PlatformAuditLogRepository
	userRepo          *accessrepo.UserRepository
	tenantRepo        *accessrepo.TenantRepository
	permissionRepo    *accessrepo.PermissionRepository
}

type AuthHandlerDeps struct {
	AuthService       *authService.Service
	JWTService        *jwt.Service
	AuditRepo         *auditrepo.AuditLogRepository
	PlatformAuditRepo *auditrepo.PlatformAuditLogRepository
	UserRepo          *accessrepo.UserRepository
	TenantRepo        *accessrepo.TenantRepository
	PermissionRepo    *accessrepo.PermissionRepository
}

func NewAuthHandlerWithDeps(deps AuthHandlerDeps) *AuthHandler {
	return &AuthHandler{
		authSvc:           deps.AuthService,
		jwtSvc:            deps.JWTService,
		auditRepo:         deps.AuditRepo,
		platformAuditRepo: deps.PlatformAuditRepo,
		userRepo:          deps.UserRepo,
		tenantRepo:        deps.TenantRepo,
		permissionRepo:    deps.PermissionRepo,
	}
}

// GetJWTService 获取 JWT 服务
func (h *AuthHandler) GetJWTService() *jwt.Service {
	return h.jwtSvc
}

// GetAuthService 获取认证服务（供邀请注册等公开接口使用）
func (h *AuthHandler) GetAuthService() *authService.Service {
	return h.authSvc
}

// UpdateProfileRequest 更新个人资料请求
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

// LoginHistoryItem 登录历史条目
type LoginHistoryItem struct {
	ID           uuid.UUID `json:"id"`
	Action       string    `json:"action"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProfileActivityItem 操作记录条目
type ProfileActivityItem struct {
	ID           uuid.UUID `json:"id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name,omitempty"`
	Status       string    `json:"status"`
	IPAddress    string    `json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
}
