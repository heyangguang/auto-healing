package handler

import (
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/repository"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/google/uuid"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authSvc           *authService.Service
	jwtSvc            *jwt.Service
	auditRepo         *repository.AuditLogRepository
	platformAuditRepo *repository.PlatformAuditLogRepository
	userRepo          *repository.UserRepository
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          cfg.JWT.Secret,
		AccessTokenTTL:  cfg.JWT.AccessTokenTTL(),
		RefreshTokenTTL: cfg.JWT.RefreshTokenTTL(),
		Issuer:          cfg.JWT.Issuer,
	}, newAuthTokenBlacklistStore())

	return &AuthHandler{
		authSvc:           authService.NewService(jwtSvc),
		jwtSvc:            jwtSvc,
		auditRepo:         repository.NewAuditLogRepository(database.DB),
		platformAuditRepo: repository.NewPlatformAuditLogRepository(),
		userRepo:          repository.NewUserRepository(),
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
