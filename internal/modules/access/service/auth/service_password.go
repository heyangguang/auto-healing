package auth

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/google/uuid"
)

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, req *ChangePasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !crypto.CheckPassword(req.OldPassword, user.PasswordHash) {
		return ErrPasswordMismatch
	}
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

// Logout 用户登出
func (s *Service) Logout(ctx context.Context, tokenJTI string, exp time.Time) error {
	return s.jwtSvc.Blacklist(ctx, tokenJTI, exp)
}
