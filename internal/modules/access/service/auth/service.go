package auth

import (
	"errors"
	"time"

	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserLocked         = errors.New("账户已锁定，请稍后再试")
	ErrUserInactive       = errors.New("账户已禁用")
	ErrPasswordMismatch   = errors.New("原密码错误")
	ErrUsernameExists     = errors.New("用户名已存在")
	ErrEmailExists        = errors.New("邮箱已存在")
)

// Service 认证服务
type Service struct {
	userRepo   *accessrepo.UserRepository
	roleRepo   *accessrepo.RoleRepository
	permRepo   *accessrepo.PermissionRepository
	tenantRepo *accessrepo.TenantRepository
	jwtSvc     *jwt.Service
	db         *gorm.DB
}

type ServiceDeps struct {
	UserRepo       *accessrepo.UserRepository
	RoleRepo       *accessrepo.RoleRepository
	PermissionRepo *accessrepo.PermissionRepository
	TenantRepo     *accessrepo.TenantRepository
	JWTService     *jwt.Service
	DB             *gorm.DB
}

func NewServiceWithDB(db *gorm.DB, jwtSvc *jwt.Service) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db, jwtSvc))
}

func DefaultServiceDepsWithDB(db *gorm.DB, jwtSvc *jwt.Service) ServiceDeps {
	return ServiceDeps{
		UserRepo:       accessrepo.NewUserRepositoryWithDB(db),
		RoleRepo:       accessrepo.NewRoleRepositoryWithDB(db),
		PermissionRepo: accessrepo.NewPermissionRepositoryWithDB(db),
		TenantRepo:     accessrepo.NewTenantRepositoryWithDB(db),
		JWTService:     jwtSvc,
		DB:             db,
	}
}

func NewServiceWithDeps(deps ServiceDeps) *Service {
	return &Service{
		userRepo:   deps.UserRepo,
		roleRepo:   deps.RoleRepo,
		permRepo:   deps.PermissionRepo,
		tenantRepo: deps.TenantRepo,
		jwtSvc:     deps.JWTService,
		db:         deps.DB,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken     string        `json:"access_token"`
	RefreshToken    string        `json:"refresh_token"`
	TokenType       string        `json:"token_type"`
	ExpiresIn       int64         `json:"expires_in"`
	User            UserInfo      `json:"user"`
	Tenants         []TenantBrief `json:"tenants"`           // 用户所属的租户列表
	CurrentTenantID string        `json:"current_tenant_id"` // 当前默认租户 ID
}

// UserInfo 用户信息
type UserInfo struct {
	ID              uuid.UUID `json:"id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	DisplayName     string    `json:"display_name"`
	Roles           []string  `json:"roles"`
	Permissions     []string  `json:"permissions"`
	IsPlatformAdmin bool      `json:"is_platform_admin"`
}

// TenantBrief 租户简要信息
type TenantBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username    string      `json:"username" binding:"required,min=3,max=50"`
	Email       string      `json:"email" binding:"required,email"`
	Password    string      `json:"password" binding:"required,min=8"`
	DisplayName string      `json:"display_name"`
	Phone       string      `json:"phone"`
	RoleIDs     []uuid.UUID `json:"role_ids"`
	TenantID    *uuid.UUID  `json:"tenant_id,omitempty"` // 可选：指定用户所属租户
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// UserProfile 用户详细资料（个人中心使用）
type UserProfile struct {
	ID                uuid.UUID    `json:"id"`
	Username          string       `json:"username"`
	Email             string       `json:"email"`
	DisplayName       string       `json:"display_name"`
	Phone             string       `json:"phone"`
	AvatarURL         string       `json:"avatar_url"`
	Status            string       `json:"status"`
	LastLoginAt       *time.Time   `json:"last_login_at"`
	LastLoginIP       string       `json:"last_login_ip"`
	PasswordChangedAt time.Time    `json:"password_changed_at"`
	CreatedAt         time.Time    `json:"created_at"`
	Roles             []RoleDetail `json:"roles"`
	Permissions       []string     `json:"permissions"`
	IsPlatformAdmin   bool         `json:"is_platform_admin"`
}

// RoleDetail 角色详情
type RoleDetail struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	IsSystem    bool      `json:"is_system"`
}
