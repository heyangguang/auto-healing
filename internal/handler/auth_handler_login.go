package handler

import (
	"errors"

	"github.com/company/auto-healing/internal/pkg/jwt"
	authService "github.com/company/auto-healing/internal/service/auth"
)

func isLoginUnauthorizedError(err error) bool {
	return errors.Is(err, authService.ErrInvalidCredentials) ||
		errors.Is(err, authService.ErrUserLocked) ||
		errors.Is(err, authService.ErrUserInactive)
}

func isLogoutClientError(err error) bool {
	return errors.Is(err, errLogoutRefreshTokenInvalid) ||
		errors.Is(err, errLogoutRefreshTokenExpired) ||
		errors.Is(err, errLogoutRefreshTokenUserMismatch) ||
		errors.Is(err, jwt.ErrInvalidToken) ||
		errors.Is(err, jwt.ErrExpiredToken)
}
