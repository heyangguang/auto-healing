package auth

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/google/uuid"
)

// GetCurrentUser 获取当前用户信息
func (s *Service) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*UserInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, permissions, isPlatformAdmin, err := s.resolveUserAccess(ctx, user)
	if err != nil {
		return nil, err
	}
	info := buildUserInfo(user, roles, permissions, isPlatformAdmin)
	return &info, nil
}

func (s *Service) resolveUserAccess(ctx context.Context, user *model.User) ([]string, []string, bool, error) {
	roleMap := make(map[string]bool)
	for _, role := range user.Roles {
		roleMap[role.Name] = true
	}
	tenantRoles, err := s.tenantRepo.GetUserAllRoles(ctx, user.ID)
	if err != nil {
		return nil, nil, false, err
	}
	for _, role := range tenantRoles {
		roleMap[role.Name] = true
	}
	roles := make([]string, 0, len(roleMap))
	for name := range roleMap {
		roles = append(roles, name)
	}
	permissions, err := s.permRepo.GetPermissionCodes(ctx, user.ID)
	if err != nil {
		return nil, nil, false, err
	}
	isPlatformAdmin := user.IsPlatformAdmin
	if isPlatformAdmin {
		platformPerms, permErr := s.permRepo.GetPlatformPermissionCodes(ctx, user.ID)
		if permErr != nil {
			return nil, nil, false, permErr
		}
		permissions = platformPerms
	}
	return roles, permissions, isPlatformAdmin, nil
}

// GetUserProfile 获取用户详细资料
func (s *Service) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roleDetails, err := buildRoleDetails(user, s.tenantRepo, ctx)
	if err != nil {
		return nil, err
	}
	_, permissions, isPlatformAdmin, err := s.resolveUserAccess(ctx, user)
	if err != nil {
		return nil, err
	}
	return &UserProfile{
		ID:                user.ID,
		Username:          user.Username,
		Email:             user.Email,
		DisplayName:       user.DisplayName,
		Phone:             user.Phone,
		AvatarURL:         user.AvatarURL,
		Status:            string(user.Status),
		LastLoginAt:       user.LastLoginAt,
		LastLoginIP:       user.LastLoginIP,
		PasswordChangedAt: user.PasswordChangedAt,
		CreatedAt:         user.CreatedAt,
		Roles:             roleDetails,
		Permissions:       permissions,
		IsPlatformAdmin:   isPlatformAdmin,
	}, nil
}

func buildRoleDetails(user *model.User, tenantRepo *accessrepo.TenantRepository, ctx context.Context) ([]RoleDetail, error) {
	roleDetails := make([]RoleDetail, 0, len(user.Roles))
	roleNameSet := make(map[string]bool)
	for _, role := range user.Roles {
		roleDetails = append(roleDetails, RoleDetail{ID: role.ID, Name: role.Name, DisplayName: role.DisplayName, IsSystem: role.IsSystem})
		roleNameSet[role.Name] = true
	}
	tenantRoles, err := tenantRepo.GetUserAllRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	for _, role := range tenantRoles {
		if !roleNameSet[role.Name] {
			roleDetails = append(roleDetails, RoleDetail{ID: role.ID, Name: role.Name, DisplayName: role.DisplayName, IsSystem: role.IsSystem})
			roleNameSet[role.Name] = true
		}
	}
	return roleDetails, nil
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
