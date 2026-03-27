package httpapi

import (
	"errors"
	"net/http"

	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/jwt"
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
