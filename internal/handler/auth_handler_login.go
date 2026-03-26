package handler

import (
	"errors"
	"net/http"

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
		errors.Is(err, errLogoutRefreshTokenSessionMismatch) ||
		errors.Is(err, errLogoutLegacyRefreshUnsupported) ||
		errors.Is(err, errLogoutSessionMetadataMissing) ||
		errors.Is(err, jwt.ErrInvalidToken) ||
		errors.Is(err, jwt.ErrExpiredToken)
}

func loginFailureStatusCode(err error) int {
	if isLoginUnauthorizedError(err) {
		return http.StatusUnauthorized
	}
	return http.StatusInternalServerError
}

func loginAuditErrorMessage(err error) string {
	if isLoginUnauthorizedError(err) {
		return ToBusinessError(err)
	}
	return "登录失败，请稍后重试"
}

func sanitizeLoginHistoryErrorMessage(status string, statusCode *int, message string) string {
	if status != "failed" || message == "" {
		return ""
	}
	if statusCode != nil && *statusCode == http.StatusUnauthorized {
		return message
	}
	return "登录失败，请稍后重试"
}
