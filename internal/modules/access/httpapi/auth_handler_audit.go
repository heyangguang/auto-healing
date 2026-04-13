package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

// writeLoginAuditLog 异步写入认证登录审计日志。
// 所有登录事件统一记录到平台审计，避免未认证阶段的事件混入租户审计。
func (h *AuthHandler) writeLoginAuditLog(parentCtx context.Context, userID *uuid.UUID, username, ipAddress, userAgent, status, errorMsg string, createdAt time.Time, statusCode int) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("LOGIN").Error("登录审计日志记录失败 (panic): %v", r)
		}
	}()

	if userID == nil && username != "" {
		h.resolveLoginAuditUser(parentCtx, &userID, username)
	}

	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	h.writePlatformLoginAudit(ctx, userID, username, ipAddress, userAgent, status, errorMsg, createdAt, statusCode)
}

func (h *AuthHandler) resolveLoginAuditUser(parentCtx context.Context, userID **uuid.UUID, username string) {
	ctx, cancel := detachedTimeoutContext(parentCtx, 3*time.Second)
	defer cancel()

	user, _ := h.userRepo.GetByUsername(ctx, username)
	if user == nil {
		return
	}

	*userID = &user.ID
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

// writeLogoutAuditLog 异步写入认证登出审计日志。
// 登出属于全局认证事件，统一记录到平台审计。
func (h *AuthHandler) writeLogoutAuditLog(parentCtx context.Context, userIDStr, username, ipAddress, userAgent string, createdAt time.Time) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("LOGIN").Error("登出审计日志记录失败 (panic): %v", r)
		}
	}()

	userID := parseAuditUserID(userIDStr)
	statusCode := http.StatusOK
	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	log := buildPlatformLogoutAuditLog(userID, username, ipAddress, userAgent, createdAt, statusCode)
	if err := h.platformAuditRepo.Create(ctx, log); err != nil {
		logger.Auth("LOGIN").Error("平台登出审计日志写入失败: %v", err)
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
