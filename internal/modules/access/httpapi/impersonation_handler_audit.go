package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/model"
	accessmodel "github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// writeImpersonationAudit 异步写入 Impersonation 进入/退出审计日志
func (h *ImpersonationHandler) writeImpersonationAudit(parentCtx context.Context, userID *uuid.UUID, username, ipAddress, userAgent string, tenantID uuid.UUID, tenantName, action string, requestID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			logger.API("IMPERSONATION").Error("Impersonation 审计日志写入失败 (panic): %v", r)
		}
	}()

	ctx, cancel := detachedTimeoutContext(parentCtx, 5*time.Second)
	defer cancel()

	now := time.Now()
	statusCode := http.StatusOK
	requestPath := impersonationAuditRequestPath(requestID, action)

	platformLog := buildPlatformImpersonationAudit(userID, username, ipAddress, userAgent, tenantName, action, requestID, requestPath, statusCode, now)
	if err := h.platformAuditRepo.Create(ctx, platformLog); err != nil {
		logger.API("IMPERSONATION").Error("Impersonation 平台审计日志写入失败: %v", err)
	}

	auditLog := buildTenantImpersonationAudit(userID, username, ipAddress, userAgent, tenantID, tenantName, action, requestID, requestPath, statusCode, now)
	if err := h.auditRepo.Create(ctx, auditLog); err != nil {
		logger.API("IMPERSONATION").Error("Impersonation 租户审计日志写入失败: %v", err)
	}
}

func impersonationAuditRequestPath(requestID uuid.UUID, action string) string {
	return "/api/v1/platform/impersonation/requests/" + requestID.String() + "/" + action[len("impersonation_"):]
}

func buildPlatformImpersonationAudit(userID *uuid.UUID, username, ipAddress, userAgent, tenantName, action string, requestID uuid.UUID, requestPath string, statusCode int, createdAt time.Time) *model.PlatformAuditLog {
	return &model.PlatformAuditLog{
		UserID:         userID,
		Username:       username,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Category:       "login",
		Action:         action,
		ResourceType:   "impersonation",
		ResourceID:     &requestID,
		ResourceName:   tenantName,
		RequestMethod:  "POST",
		RequestPath:    requestPath,
		ResponseStatus: &statusCode,
		Status:         "success",
		CreatedAt:      createdAt,
	}
}

func buildTenantImpersonationAudit(userID *uuid.UUID, username, ipAddress, userAgent string, tenantID uuid.UUID, tenantName, action string, requestID uuid.UUID, requestPath string, statusCode int, createdAt time.Time) *model.AuditLog {
	return &model.AuditLog{
		TenantID:       &tenantID,
		UserID:         userID,
		Username:       username + " [Impersonation]",
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Category:       "login",
		Action:         action,
		ResourceType:   "impersonation",
		ResourceID:     &requestID,
		ResourceName:   tenantName,
		RequestMethod:  "POST",
		RequestPath:    requestPath,
		ResponseStatus: &statusCode,
		Status:         "success",
		CreatedAt:      createdAt,
	}
}

// notifyApproversNewRequest 异步向租户审批人发送站内消息
func (h *ImpersonationHandler) notifyApproversNewRequest(ctx context.Context, impReq *accessmodel.ImpersonationRequest) {
	defer func() {
		if r := recover(); r != nil {
			logger.API("IMPERSONATION").Error("notifyApproversNewRequest panic: %v", r)
		}
	}()
	notifyCtx, cancel := detachedTimeoutContext(ctx, 5*time.Second)
	defer cancel()

	msg := &model.SiteMessage{
		TenantID:       &impReq.TenantID,
		TargetTenantID: &impReq.TenantID,
		Category:       model.SiteMessageCategoryServiceNotice,
		Title:          "新的临时提权申请待审批",
		Content:        impReq.RequesterName + " 申请临时访问本租户（" + impReq.TenantName + "），申请时长 " + formatMinutes(impReq.DurationMinutes) + "，请及时处理。",
	}
	if err := h.siteMessageRepo.Create(notifyCtx, msg); err != nil {
		logger.API("IMPERSONATION").Error("发送审批人站内消息失败: %v", err)
	}
}

// notifyRequesterDecision 异步向申请人发送站内消息
func (h *ImpersonationHandler) notifyRequesterDecision(ctx context.Context, impReq *accessmodel.ImpersonationRequest, approved bool, approverName string) {
	defer func() {
		if r := recover(); r != nil {
			logger.API("IMPERSONATION").Error("notifyRequesterDecision panic: %v", r)
		}
	}()
	notifyCtx, cancel := detachedTimeoutContext(ctx, 5*time.Second)
	defer cancel()

	title, content := impersonationDecisionMessage(impReq, approved, approverName)
	msg := &model.SiteMessage{
		TenantID:       &impReq.TenantID,
		TargetTenantID: nil,
		Category:       model.SiteMessageCategoryServiceNotice,
		Title:          title,
		Content:        content,
	}
	if err := h.siteMessageRepo.Create(notifyCtx, msg); err != nil {
		logger.API("IMPERSONATION").Error("发送申请人站内消息失败: %v", err)
	}
}

func impersonationDecisionMessage(impReq *accessmodel.ImpersonationRequest, approved bool, approverName string) (string, string) {
	if approved {
		return "临时提权申请已批准", "您申请访问租户「" + impReq.TenantName + "」的提权请求已由 " + approverName + " 批准，请在 " + formatMinutes(impReq.DurationMinutes) + " 内完成操作。"
	}
	return "临时提权申请已拒绝", "您申请访问租户「" + impReq.TenantName + "」的提权请求已由 " + approverName + " 拒绝。"
}

// formatMinutes 将分钟转化为可读文本
func formatMinutes(minutes int) string {
	if minutes >= 60 {
		hours := minutes / 60
		remainingMinutes := minutes % 60
		if remainingMinutes == 0 {
			return fmt.Sprintf("%d 小时", hours)
		}
		return fmt.Sprintf("%d 小时 %d 分钟", hours, remainingMinutes)
	}
	return fmt.Sprintf("%d 分钟", minutes)
}
