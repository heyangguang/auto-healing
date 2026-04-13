package httpapi

import (
	"errors"

	authservice "github.com/company/auto-healing/internal/modules/access/service/auth"
)

const (
	authAuditCategory = "auth"
	authResourceType  = "auth"

	authActionLogin    = "login"
	authActionLogout   = "logout"
	authActionRegister = "register"

	authMethodPassword           = "password"
	authMethodToken              = "token"
	authMethodInvitationRegister = "invitation_register"

	authSubjectScopePlatformAdmin = "platform_admin"
	authSubjectScopeTenantUser    = "tenant_user"
	authSubjectScopeUnknown       = "unknown"

	authFailureReasonUnknownUsername   = "unknown_username"
	authFailureReasonInvalidPassword   = "invalid_password"
	authFailureReasonUserLocked        = "user_locked"
	authFailureReasonUserInactive      = "user_inactive"
	authFailureReasonInvitationInvalid = "invitation_invalid"
	authFailureReasonValidationFailed  = "validation_failed"
	authFailureReasonUsernameExists    = "username_exists"
	authFailureReasonEmailExists       = "email_exists"
	authFailureReasonSystemError       = "system_error"
)

func loginFailureReason(err error, userFound bool) string {
	switch {
	case errors.Is(err, authservice.ErrUserLocked):
		return authFailureReasonUserLocked
	case errors.Is(err, authservice.ErrUserInactive):
		return authFailureReasonUserInactive
	case errors.Is(err, authservice.ErrInvalidCredentials):
		if !userFound {
			return authFailureReasonUnknownUsername
		}
		return authFailureReasonInvalidPassword
	default:
		return authFailureReasonSystemError
	}
}

func registerFailureReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, authservice.ErrUsernameExists):
		return authFailureReasonUsernameExists
	case errors.Is(err, authservice.ErrEmailExists):
		return authFailureReasonEmailExists
	default:
		return authFailureReasonSystemError
	}
}

func registerRequestFailureReason() string {
	return authFailureReasonValidationFailed
}

func invitationFailureReason() string {
	return authFailureReasonInvitationInvalid
}
