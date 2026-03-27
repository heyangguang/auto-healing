package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Register 用户注册 (管理员创建用户)
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*model.User, error) {
	if err := s.ensureRegisterUnique(ctx, req); err != nil {
		return nil, err
	}
	if err := s.validateRegisterRoles(ctx, req.RoleIDs); err != nil {
		return nil, err
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		DisplayName:  req.DisplayName,
		Phone:        req.Phone,
		Status:       "active",
	}
	if err := s.registerTx(ctx, user, req.RoleIDs, req.TenantID); err != nil {
		return nil, err
	}
	return s.userRepo.GetByID(ctx, user.ID)
}

func (s *Service) ensureRegisterUnique(ctx context.Context, req *RegisterRequest) error {
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return err
	}
	if exists {
		return ErrUsernameExists
	}
	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return err
	}
	if exists {
		return ErrEmailExists
	}
	return nil
}

func (s *Service) validateRegisterRoles(ctx context.Context, roleIDs []uuid.UUID) error {
	for _, roleID := range roleIDs {
		if _, err := s.roleRepo.GetByID(ctx, roleID); err != nil {
			return errors.New("选择的角色不存在")
		}
	}
	return nil
}

func (s *Service) assignRegisterRoles(ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	return assignRegisterRolesWithRepo(s.userRepo, ctx, userID, roleIDs)
}

func assignRegisterRolesWithRepo(userRepo *accessrepo.UserRepository, ctx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if len(roleIDs) == 0 {
		return nil
	}
	if err := userRepo.AssignRoles(ctx, userID, roleIDs); err != nil {
		return fmt.Errorf("分配角色失败: %w", err)
	}
	return nil
}

func (s *Service) attachRegisterTenant(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) error {
	return attachRegisterTenantWithRepo(s.roleRepo, s.tenantRepo, ctx, userID, tenantID)
}

func attachRegisterTenantWithRepo(roleRepo *accessrepo.RoleRepository, tenantRepo *accessrepo.TenantRepository, ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) error {
	if tenantID == nil {
		return nil
	}
	viewerRole, err := roleRepo.GetByName(ctx, "viewer")
	if err != nil {
		viewerRole, err = roleRepo.GetByName(ctx, "operator")
		if err != nil {
			return fmt.Errorf("未找到可分配的默认角色（viewer/operator）")
		}
	}
	if err := tenantRepo.AddMember(ctx, userID, *tenantID, viewerRole.ID); err != nil {
		return fmt.Errorf("关联租户失败: %w", err)
	}
	return nil
}

func (s *Service) registerTx(ctx context.Context, user *model.User, roleIDs []uuid.UUID, tenantID *uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userRepo := accessrepo.NewUserRepositoryWithDB(tx)
		roleRepo := accessrepo.NewRoleRepositoryWithDB(tx)
		tenantRepo := accessrepo.NewTenantRepositoryWithDB(tx)
		if err := userRepo.Create(ctx, user); err != nil {
			return err
		}
		if err := assignRegisterRolesWithRepo(userRepo, ctx, user.ID, roleIDs); err != nil {
			return err
		}
		return attachRegisterTenantWithRepo(roleRepo, tenantRepo, ctx, user.ID, tenantID)
	})
}
