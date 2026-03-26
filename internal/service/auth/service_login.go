package auth

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

// Login 用户登录
func (s *Service) Login(ctx context.Context, req *LoginRequest, clientIP string) (*LoginResponse, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := s.ensureLoginAllowed(ctx, user, req.Password); err != nil {
		return nil, err
	}
	if err := s.recordSuccessfulLogin(ctx, user.ID, clientIP); err != nil {
		return nil, err
	}

	roles, permissions, isPlatformAdmin, err := s.resolveUserAccess(ctx, user)
	if err != nil {
		return nil, err
	}
	tenants, currentTenantID, tenantIDs, err := s.loginTenants(ctx, user.ID, isPlatformAdmin)
	if err != nil {
		return nil, err
	}

	tokenPair, err := s.issueLoginToken(user, roles, permissions, isPlatformAdmin, tenantIDs, currentTenantID)
	if err != nil {
		return nil, err
	}
	return &LoginResponse{
		AccessToken:     tokenPair.AccessToken,
		RefreshToken:    tokenPair.RefreshToken,
		TokenType:       tokenPair.TokenType,
		ExpiresIn:       tokenPair.ExpiresIn,
		User:            buildUserInfo(user, roles, permissions, isPlatformAdmin),
		Tenants:         tenants,
		CurrentTenantID: currentTenantID,
	}, nil
}

func (s *Service) ensureLoginAllowed(ctx context.Context, user *model.User, password string) error {
	if user.Status == "locked" {
		if user.LockedUntil == nil || user.LockedUntil.After(time.Now()) {
			return ErrUserLocked
		}
		if err := s.clearExpiredLoginLock(ctx, user.ID); err != nil {
			return err
		}
	}
	if user.Status == "inactive" {
		return ErrUserInactive
	}
	if !crypto.CheckPassword(password, user.PasswordHash) {
		if err := s.recordFailedLogin(ctx, user.ID); err != nil {
			return err
		}
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) recordSuccessfulLogin(ctx context.Context, userID uuid.UUID, clientIP string) error {
	return s.userRepo.UpdateLoginInfo(ctx, userID, clientIP)
}

func (s *Service) clearExpiredLoginLock(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.UpdateLoginInfo(ctx, userID, "")
}

func (s *Service) recordFailedLogin(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.IncrementFailedLogin(ctx, userID)
}

func (s *Service) loginTenants(ctx context.Context, userID uuid.UUID, isPlatformAdmin bool) ([]TenantBrief, string, []string, error) {
	if isPlatformAdmin {
		return []TenantBrief{}, "", nil, nil
	}
	tenants, err := s.tenantRepo.GetUserTenants(ctx, userID, "")
	if err != nil {
		return nil, "", nil, err
	}
	briefs := make([]TenantBrief, len(tenants))
	tenantIDs := make([]string, len(tenants))
	for i, tenant := range tenants {
		briefs[i] = TenantBrief{ID: tenant.ID.String(), Name: tenant.Name, Code: tenant.Code}
		tenantIDs[i] = tenant.ID.String()
	}
	currentTenantID := ""
	if len(tenants) > 0 {
		currentTenantID = tenants[0].ID.String()
	}
	return briefs, currentTenantID, tenantIDs, nil
}

func (s *Service) issueLoginToken(user *model.User, roles, permissions []string, isPlatformAdmin bool, tenantIDs []string, currentTenantID string) (*jwt.TokenPair, error) {
	var tokenOpts []func(*jwt.Claims)
	if isPlatformAdmin {
		tokenOpts = append(tokenOpts, func(c *jwt.Claims) { c.IsPlatformAdmin = true })
	}
	tokenOpts = append(tokenOpts, func(c *jwt.Claims) {
		c.TenantIDs = tenantIDs
		c.DefaultTenantID = currentTenantID
	})
	return s.jwtSvc.GenerateTokenPair(user.ID.String(), user.Username, roles, permissions, tokenOpts...)
}

func buildUserInfo(user *model.User, roles, permissions []string, isPlatformAdmin bool) UserInfo {
	return UserInfo{
		ID:              user.ID,
		Username:        user.Username,
		Email:           user.Email,
		DisplayName:     user.DisplayName,
		Roles:           roles,
		Permissions:     permissions,
		IsPlatformAdmin: isPlatformAdmin,
	}
}
