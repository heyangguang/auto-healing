package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
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
	userRepo *repository.UserRepository
	roleRepo *repository.RoleRepository
	permRepo *repository.PermissionRepository
	jwtSvc   *jwt.Service
}

// NewService 创建认证服务
func NewService(jwtSvc *jwt.Service) *Service {
	return &Service{
		userRepo: repository.NewUserRepository(),
		roleRepo: repository.NewRoleRepository(),
		permRepo: repository.NewPermissionRepository(),
		jwtSvc:   jwtSvc,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int64    `json:"expires_in"`
	User         UserInfo `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
}

// Login 用户登录
func (s *Service) Login(ctx context.Context, req *LoginRequest, clientIP string) (*LoginResponse, error) {
	// 获取用户
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// 检查账户状态
	if user.Status == "locked" {
		if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
			return nil, ErrUserLocked
		}
	}
	if user.Status == "inactive" {
		return nil, ErrUserInactive
	}

	// 验证密码
	if !crypto.CheckPassword(req.Password, user.PasswordHash) {
		// 增加失败次数
		_ = s.userRepo.IncrementFailedLogin(ctx, user.ID)
		return nil, ErrInvalidCredentials
	}

	// 更新登录信息
	_ = s.userRepo.UpdateLoginInfo(ctx, user.ID, clientIP)

	// 获取用户角色和权限
	roles := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roles[i] = role.Name
	}

	permissions, err := s.permRepo.GetPermissionCodes(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 检查是否是超级管理员
	for _, role := range roles {
		if role == "super_admin" {
			permissions = []string{"*"}
			break
		}
	}

	// 生成 Token
	tokenPair, err := s.jwtSvc.GenerateTokenPair(user.ID.String(), user.Username, roles, permissions)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		User: UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Roles:       roles,
			Permissions: permissions,
		},
	}, nil
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username    string      `json:"username" binding:"required,min=3,max=50"`
	Email       string      `json:"email" binding:"required,email"`
	Password    string      `json:"password" binding:"required,min=8"`
	DisplayName string      `json:"display_name"`
	Phone       string      `json:"phone"`
	RoleIDs     []uuid.UUID `json:"role_ids"`
}

// Register 用户注册 (管理员创建用户)
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*model.User, error) {
	// 检查用户名是否存在
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// 检查邮箱是否存在
	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailExists
	}

	// 先验证角色是否都存在（在创建用户之前）
	if len(req.RoleIDs) > 0 {
		for _, roleID := range req.RoleIDs {
			_, err := s.roleRepo.GetByID(ctx, roleID)
			if err != nil {
				return nil, errors.New("选择的角色不存在")
			}
		}
	}

	// 加密密码
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		DisplayName:  req.DisplayName,
		Phone:        req.Phone,
		Status:       "active",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 分配角色（已验证过存在性）
	if len(req.RoleIDs) > 0 {
		if err := s.userRepo.AssignRoles(ctx, user.ID, req.RoleIDs); err != nil {
			return nil, fmt.Errorf("分配角色失败: %w", err)
		}
	}

	// 重新获取用户（包含角色信息）
	return s.userRepo.GetByID(ctx, user.ID)
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, req *ChangePasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// 验证原密码
	if !crypto.CheckPassword(req.OldPassword, user.PasswordHash) {
		return ErrPasswordMismatch
	}

	// 加密新密码
	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(ctx, userID, newHash)
}

// ResetPassword 重置密码 (管理员操作)
func (s *Service) ResetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	passwordHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.userRepo.UpdatePassword(ctx, userID, passwordHash)
}

// GetCurrentUser 获取当前用户信息
func (s *Service) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*UserInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roles[i] = role.Name
	}

	permissions, err := s.permRepo.GetPermissionCodes(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// 检查超级管理员
	for _, role := range roles {
		if role == "super_admin" {
			permissions = []string{"*"}
			break
		}
	}

	return &UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Roles:       roles,
		Permissions: permissions,
	}, nil
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
}

// RoleDetail 角色详情
type RoleDetail struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	IsSystem    bool      `json:"is_system"`
}

// GetUserProfile 获取用户详细资料
func (s *Service) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	roleDetails := make([]RoleDetail, len(user.Roles))
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleDetails[i] = RoleDetail{
			ID:          role.ID,
			Name:        role.Name,
			DisplayName: role.DisplayName,
			IsSystem:    role.IsSystem,
		}
		roleNames[i] = role.Name
	}

	permissions, err := s.permRepo.GetPermissionCodes(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	for _, role := range roleNames {
		if role == "super_admin" {
			permissions = []string{"*"}
			break
		}
	}

	return &UserProfile{
		ID:                user.ID,
		Username:          user.Username,
		Email:             user.Email,
		DisplayName:       user.DisplayName,
		Phone:             user.Phone,
		AvatarURL:         user.AvatarURL,
		Status:            user.Status,
		LastLoginAt:       user.LastLoginAt,
		LastLoginIP:       user.LastLoginIP,
		PasswordChangedAt: user.PasswordChangedAt,
		CreatedAt:         user.CreatedAt,
		Roles:             roleDetails,
		Permissions:       permissions,
	}, nil
}

// UpdateProfile 更新个人信息
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, email, phone string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if displayName != "" {
		user.DisplayName = displayName
	}
	if email != "" {
		user.Email = email
	}
	if phone != "" {
		user.Phone = phone
	}

	return s.userRepo.Update(ctx, user)
}

// Logout 用户登出
func (s *Service) Logout(ctx context.Context, tokenJTI string, exp time.Time) error {
	return s.jwtSvc.Blacklist(ctx, tokenJTI, exp)
}
