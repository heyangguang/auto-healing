package httpapi

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

func (h *TenantHandler) writeRegisterAuditLog(parentCtx context.Context, userID *uuid.UUID, username string, tenantID *uuid.UUID, tenantName, ipAddress, userAgent, status, errorMsg, failureReason string, createdAt time.Time, statusCode int) {
	defer func() {
		if r := recover(); r != nil {
			logger.Auth("REGISTER").Error("注册审计日志记录失败 (panic): %v", r)
		}
	}()

	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	log := &platformmodel.PlatformAuditLog{
		ID:                uuid.New(),
		UserID:            userID,
		Username:          username,
		PrincipalUsername: username,
		SubjectScope:      authSubjectScopeTenantUser,
		SubjectTenantID:   tenantID,
		SubjectTenantName: tenantName,
		FailureReason:     failureReason,
		AuthMethod:        authMethodInvitationRegister,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
		Category:          authAuditCategory,
		Action:            authActionRegister,
		ResourceType:      authResourceType,
		RequestMethod:     "POST",
		RequestPath:       "/api/v1/auth/register",
		ResponseStatus:    &statusCode,
		Status:            status,
		ErrorMessage:      errorMsg,
		CreatedAt:         createdAt,
	}
	if err := h.platformAuditRepo.Create(ctx, log); err != nil {
		logger.Auth("REGISTER").Error("平台注册审计日志写入失败: %v", err)
	}
}
