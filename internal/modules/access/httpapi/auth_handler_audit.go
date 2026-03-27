package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

// writeLoginAuditLog 异步写入登录审计日志
func (h *AuthHandler) writeLoginAuditLog(parentCtx context.Context, userID *uuid.UUID, username, ipAddress, userAgent, status, errorMsg string, createdAt time.Time, statusCode int, isPlatformAdmin bool, defaultTenantID string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("LOGIN").Error("登录审计日志记录失败 (panic): %v", r)
		}
	}()

	if userID == nil && username != "" {
		h.resolveLoginAuditUser(parentCtx, &userID, username, &isPlatformAdmin, &defaultTenantID)
	}

	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	if isPlatformAdmin {
		h.writePlatformLoginAudit(ctx, userID, username, ipAddress, userAgent, status, errorMsg, createdAt, statusCode)
		return
	}
	h.writeTenantLoginAudit(ctx, userID, username, ipAddress, userAgent, status, errorMsg, createdAt, statusCode, defaultTenantID)
}

func (h *AuthHandler) resolveLoginAuditUser(parentCtx context.Context, userID **uuid.UUID, username string, isPlatformAdmin *bool, defaultTenantID *string) {
	ctx, cancel := detachedTimeoutContext(parentCtx, 3*time.Second)
	defer cancel()

	user, _ := h.userRepo.GetByUsername(ctx, username)
	if user == nil {
		return
	}

	*userID = &user.ID
	*isPlatformAdmin = user.IsPlatformAdmin
}

func (h *AuthHandler) writePlatformLoginAudit(ctx context.Context, userID *uuid.UUID, username, ipAddress, userAgent, status, errorMsg string, createdAt time.Time, statusCode int) {
	log := &platformmodel.PlatformAuditLog{
		ID:             uuid.New(),
		UserID:         userID,
		Username:       username,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Category:       "login",
		Action:         "login",
		ResourceType:   "auth",
		RequestMethod:  "POST",
		RequestPath:    "/api/v1/auth/login",
		ResponseStatus: &statusCode,
		Status:         status,
		ErrorMessage:   errorMsg,
		CreatedAt:      createdAt,
	}
	if err := h.platformAuditRepo.Create(ctx, log); err != nil {
		logger.Auth("LOGIN").Error("平台登录审计日志写入失败: %v", err)
	}
}

func (h *AuthHandler) writeTenantLoginAudit(ctx context.Context, userID *uuid.UUID, username, ipAddress, userAgent, status, errorMsg string, createdAt time.Time, statusCode int, defaultTenantID string) {
	log := &platformmodel.AuditLog{
		ID:             uuid.New(),
		UserID:         userID,
		Username:       username,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Category:       "login",
		Action:         "login",
		ResourceType:   "auth",
		RequestMethod:  "POST",
		RequestPath:    "/api/v1/auth/login",
		ResponseStatus: &statusCode,
		Status:         status,
		ErrorMessage:   errorMsg,
		CreatedAt:      createdAt,
	}
	if defaultTenantID != "" {
		if tenantID, err := uuid.Parse(defaultTenantID); err == nil {
			log.TenantID = &tenantID
		}
	}
	if err := h.auditRepo.Create(ctx, log); err != nil {
		logger.Auth("LOGIN").Error("登录审计日志写入失败: %v", err)
	}
}

// writeLogoutAuditLog 异步写入登出审计日志
func (h *AuthHandler) writeLogoutAuditLog(parentCtx context.Context, userIDStr, username, ipAddress, userAgent string, createdAt time.Time, isPlatformAdmin bool, tenantID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("LOGIN").Error("登出审计日志记录失败 (panic): %v", r)
		}
	}()

	userID := parseAuditUserID(userIDStr)
	statusCode := http.StatusOK
	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	if isPlatformAdmin {
		log := buildPlatformLogoutAuditLog(userID, username, ipAddress, userAgent, createdAt, statusCode)
		if err := h.platformAuditRepo.Create(ctx, log); err != nil {
			logger.Auth("LOGIN").Error("平台登出审计日志写入失败: %v", err)
		}
		return
	}

	log := buildTenantLogoutAuditLog(userID, username, ipAddress, userAgent, createdAt, statusCode, tenantID)
	if err := h.auditRepo.Create(ctx, log); err != nil {
		logger.Auth("LOGIN").Error("登出审计日志写入失败: %v", err)
	}
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

func buildPlatformLogoutAuditLog(userID *uuid.UUID, username, ipAddress, userAgent string, createdAt time.Time, statusCode int) *platformmodel.PlatformAuditLog {
	return &platformmodel.PlatformAuditLog{
		ID:             uuid.New(),
		UserID:         userID,
		Username:       username,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Category:       "login",
		Action:         "logout",
		ResourceType:   "auth-logout",
		RequestMethod:  "POST",
		RequestPath:    "/api/v1/auth/logout",
		ResponseStatus: &statusCode,
		Status:         "success",
		CreatedAt:      createdAt,
	}
}

func buildTenantLogoutAuditLog(userID *uuid.UUID, username, ipAddress, userAgent string, createdAt time.Time, statusCode int, tenantID uuid.UUID) *platformmodel.AuditLog {
	log := &platformmodel.AuditLog{
		ID:             uuid.New(),
		UserID:         userID,
		Username:       username,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Category:       "login",
		Action:         "logout",
		ResourceType:   "auth-logout",
		RequestMethod:  "POST",
		RequestPath:    "/api/v1/auth/logout",
		ResponseStatus: &statusCode,
		Status:         "success",
		CreatedAt:      createdAt,
	}
	if tenantID != uuid.Nil {
		log.TenantID = &tenantID
	}
	return log
}
