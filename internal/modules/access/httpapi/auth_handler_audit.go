package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

type authAuditSubject struct {
	UserID            *uuid.UUID
	PrincipalUsername string
	SubjectScope      string
	SubjectTenantID   *uuid.UUID
	SubjectTenantName string
	UserFound         bool
}

// writeLoginAuditLog 异步写入认证登录审计日志。
func (h *AuthHandler) writeLoginAuditLog(parentCtx context.Context, userID *uuid.UUID, username, ipAddress, userAgent, status string, auditErr error, createdAt time.Time, statusCode int, currentTenantID string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("LOGIN").Error("登录审计日志记录失败 (panic): %v", r)
		}
	}()

	subject := h.resolveAuthAuditSubject(parentCtx, userID, username, currentTenantID)
	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	log := &platformmodel.PlatformAuditLog{
		ID:                uuid.New(),
		UserID:            subject.UserID,
		Username:          subject.PrincipalUsername,
		PrincipalUsername: subject.PrincipalUsername,
		SubjectScope:      subject.SubjectScope,
		SubjectTenantID:   subject.SubjectTenantID,
		SubjectTenantName: subject.SubjectTenantName,
		FailureReason:     resolveLoginFailureReason(status, auditErr, subject.UserFound),
		AuthMethod:        authMethodPassword,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
		Category:          authAuditCategory,
		Action:            authActionLogin,
		ResourceType:      authResourceType,
		RequestMethod:     http.MethodPost,
		RequestPath:       "/api/v1/auth/login",
		ResponseStatus:    &statusCode,
		Status:            status,
		ErrorMessage:      authAuditErrorMessage(status, auditErr),
		CreatedAt:         createdAt,
	}
	if err := h.platformAuditRepo.Create(ctx, log); err != nil {
		logger.Auth("LOGIN").Error("平台登录审计日志写入失败: %v", err)
	}
}

// writeLogoutAuditLog 异步写入认证登出审计日志。
func (h *AuthHandler) writeLogoutAuditLog(parentCtx context.Context, userIDStr, username, ipAddress, userAgent string, createdAt time.Time, isPlatformAdmin bool, tenantID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("LOGIN").Error("登出审计日志记录失败 (panic): %v", r)
		}
	}()

	userID := parseAuditUserID(userIDStr)
	preferredTenantID := ""
	if tenantID != uuid.Nil {
		preferredTenantID = tenantID.String()
	}
	subject := h.resolveAuthAuditSubject(parentCtx, userID, username, preferredTenantID)
	if isPlatformAdmin {
		subject.SubjectScope = authSubjectScopePlatformAdmin
		subject.SubjectTenantID = nil
		subject.SubjectTenantName = ""
	}

	statusCode := http.StatusOK
	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	log := &platformmodel.PlatformAuditLog{
		ID:                uuid.New(),
		UserID:            subject.UserID,
		Username:          subject.PrincipalUsername,
		PrincipalUsername: subject.PrincipalUsername,
		SubjectScope:      subject.SubjectScope,
		SubjectTenantID:   subject.SubjectTenantID,
		SubjectTenantName: subject.SubjectTenantName,
		AuthMethod:        authMethodToken,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
		Category:          authAuditCategory,
		Action:            authActionLogout,
		ResourceType:      authResourceType,
		RequestMethod:     http.MethodPost,
		RequestPath:       "/api/v1/auth/logout",
		ResponseStatus:    &statusCode,
		Status:            "success",
		CreatedAt:         createdAt,
	}
	if err := h.platformAuditRepo.Create(ctx, log); err != nil {
		logger.Auth("LOGIN").Error("平台登出审计日志写入失败: %v", err)
	}
}

func (h *AuthHandler) resolveAuthAuditSubject(parentCtx context.Context, userID *uuid.UUID, username string, preferredTenantID string) authAuditSubject {
	ctx, cancel := detachedTimeoutContext(parentCtx, 3*time.Second)
	defer cancel()

	user := h.loadAuthAuditUser(ctx, userID, username)
	if user == nil {
		return authAuditSubject{
			UserID:            nil,
			PrincipalUsername: username,
			SubjectScope:      authSubjectScopeUnknown,
			UserFound:         false,
		}
	}

	subject := authAuditSubject{
		UserID:            &user.ID,
		PrincipalUsername: firstNonEmpty(username, user.Username),
		SubjectScope:      authSubjectScopeTenantUser,
		UserFound:         true,
	}
	if user.IsPlatformAdmin {
		subject.SubjectScope = authSubjectScopePlatformAdmin
		return subject
	}
	subject.SubjectTenantID, subject.SubjectTenantName = h.resolveAuditSubjectTenant(ctx, user.ID, preferredTenantID)
	return subject
}

func (h *AuthHandler) loadAuthAuditUser(ctx context.Context, userID *uuid.UUID, username string) *platformmodelAuditUser {
	if userID != nil {
		user, err := h.userRepo.GetByID(ctx, *userID)
		if err == nil && user != nil {
			return &platformmodelAuditUser{ID: user.ID, Username: user.Username, IsPlatformAdmin: user.IsPlatformAdmin}
		}
	}
	if username == "" {
		return nil
	}
	user, err := h.userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil
	}
	return &platformmodelAuditUser{ID: user.ID, Username: user.Username, IsPlatformAdmin: user.IsPlatformAdmin}
}

type platformmodelAuditUser struct {
	ID              uuid.UUID
	Username        string
	IsPlatformAdmin bool
}

func (h *AuthHandler) resolveAuditSubjectTenant(ctx context.Context, userID uuid.UUID, preferredTenantID string) (*uuid.UUID, string) {
	tenants, err := h.tenantRepo.GetUserTenants(ctx, userID, "")
	if err != nil || len(tenants) == 0 {
		return nil, ""
	}
	if preferredTenantID != "" {
		for _, tenant := range tenants {
			if tenant.ID.String() == preferredTenantID {
				tenantID := tenant.ID
				return &tenantID, tenant.Name
			}
		}
	}
	tenantID := tenants[0].ID
	return &tenantID, tenants[0].Name
}

func resolveLoginFailureReason(status string, auditErr error, userFound bool) string {
	if status != "failed" {
		return ""
	}
	return loginFailureReason(auditErr, userFound)
}

func authAuditErrorMessage(status string, auditErr error) string {
	if status != "failed" || auditErr == nil {
		return ""
	}
	return loginAuditErrorMessage(auditErr)
}

func parseAuditUserID(userIDStr string) *uuid.UUID {
	if userIDStr == "" {
		return nil
	}
	if userID, err := uuid.Parse(userIDStr); err == nil {
		return &userID
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
