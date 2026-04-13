package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/access/model"
	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/response"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ValidateInvitation 验证邀请 token
func (h *TenantHandler) ValidateInvitation(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		response.BadRequest(c, "邀请令牌不能为空")
		return
	}

	inv, ok := h.loadValidInvitation(c, token)
	if !ok {
		return
	}
	if err := h.ensureInvitationTargetsValid(c.Request.Context(), inv); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, invitationValidationResponse{
		ID:         inv.ID,
		Email:      inv.Email,
		TenantName: inv.Tenant.Name,
		TenantCode: inv.Tenant.Code,
		RoleName:   inv.Role.DisplayName,
		ExpiresAt:  inv.ExpiresAt,
	})
}

// RegisterByInvitation 通过邀请注册
func (h *TenantHandler) RegisterByInvitation(c *gin.Context) {
	clientIP := middleware.NormalizeIP(c.ClientIP())
	userAgent := c.Request.UserAgent()
	startTime := time.Now()
	auditUsername := ""
	auditStatus := "failed"
	auditError := "注册失败"
	auditFailureReason := authFailureReasonSystemError
	auditStatusCode := http.StatusBadRequest
	var auditUserID *uuid.UUID
	var auditTenantID *uuid.UUID
	auditTenantName := ""
	defer func() {
		platformlifecycle.Go(func(rootCtx context.Context) {
			h.writeRegisterAuditLog(rootCtx, auditUserID, auditUsername, auditTenantID, auditTenantName, clientIP, userAgent, auditStatus, auditError, auditFailureReason, startTime, auditStatusCode)
		})
	}()

	var req RegisterByInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auditError = "请求参数错误"
		auditFailureReason = registerRequestFailureReason()
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}
	auditUsername = req.Username

	inv, ok := h.loadValidInvitation(c, req.Token)
	if !ok {
		auditError = "邀请不存在或已过期"
		auditFailureReason = invitationFailureReason()
		return
	}
	auditTenantID = &inv.TenantID
	if err := h.ensureInvitationTargetsValid(c.Request.Context(), inv); err != nil {
		auditError = err.Error()
		auditFailureReason = invitationFailureReason()
		response.BadRequest(c, err.Error())
		return
	}
	auditTenantName = inv.Tenant.Name

	user, err := h.authSvc.Register(c.Request.Context(), buildInvitationRegisterRequest(req, inv))
	if err != nil {
		auditError = ToBusinessError(err)
		auditFailureReason = registerFailureReason(err)
		response.BadRequest(c, auditError)
		return
	}
	auditUserID = &user.ID

	if err := h.completeInvitationRegistration(c.Request.Context(), user.ID, inv); err != nil {
		auditStatusCode = http.StatusInternalServerError
		auditError = "完成邀请注册失败"
		auditFailureReason = authFailureReasonSystemError
		respondInternalError(c, "TENANT", "完成邀请注册失败", err)
		return
	}
	auditStatus = "success"
	auditStatusCode = http.StatusCreated
	auditError = ""
	auditFailureReason = ""
	response.Created(c, invitationRegisterResponse{
		User:    user,
		Message: "注册成功，请登录",
	})
}

func buildInvitationRegisterRequest(req RegisterByInvitationRequest, inv *model.TenantInvitation) *authService.RegisterRequest {
	return &authService.RegisterRequest{
		Username:    req.Username,
		Email:       inv.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	}
}

func (h *TenantHandler) loadValidInvitation(c *gin.Context, token string) (*model.TenantInvitation, bool) {
	if _, err := h.invRepo.ExpireOldInvitations(c.Request.Context()); err != nil {
		respondInternalError(c, "TENANT", "更新邀请过期状态失败", err)
		return nil, false
	}

	inv, err := h.invRepo.GetByTokenHash(c.Request.Context(), hashToken(token))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "邀请不存在或已过期")
			return nil, false
		}
		respondInternalError(c, "TENANT", "查询邀请失败", err)
		return nil, false
	}
	if time.Now().After(inv.ExpiresAt) {
		if err := h.invRepo.UpdateStatus(c.Request.Context(), inv.ID, model.InvitationStatusExpired); err != nil {
			respondInternalError(c, "TENANT", "更新邀请过期状态失败", err)
			return nil, false
		}
		response.BadRequest(c, "邀请已过期")
		return nil, false
	}
	return inv, true
}

func (h *TenantHandler) ensureInvitationTargetsValid(ctx context.Context, inv *model.TenantInvitation) error {
	tenant, err := h.repo.GetByID(ctx, inv.TenantID)
	if err != nil {
		return businessError("邀请已失效，请联系管理员重新发起邀请")
	}
	if tenant.Status != model.TenantStatusActive {
		return businessError("邀请已失效，请联系管理员重新发起邀请")
	}

	role, err := h.roleRepo.GetTenantRoleByID(ctx, inv.TenantID, inv.RoleID)
	if err != nil {
		return businessError("邀请已失效，请联系管理员重新发起邀请")
	}

	inv.Tenant = tenant
	inv.Role = role
	return nil
}

func (h *TenantHandler) completeInvitationRegistration(ctx context.Context, userID uuid.UUID, inv *model.TenantInvitation) error {
	if err := h.repo.AddMember(ctx, userID, inv.TenantID, inv.RoleID); err != nil {
		return h.rollbackInvitationRegistration(ctx, userID, inv.TenantID, fmt.Errorf("关联邀请角色失败: %w", err))
	}
	if err := h.invRepo.UpdateStatus(ctx, inv.ID, model.InvitationStatusAccepted); err != nil {
		return h.rollbackInvitationRegistration(ctx, userID, inv.TenantID, fmt.Errorf("更新邀请状态失败: %w", err))
	}
	return nil
}

func (h *TenantHandler) rollbackInvitationRegistration(ctx context.Context, userID, tenantID uuid.UUID, cause error) error {
	if err := h.repo.RemoveMember(ctx, userID, tenantID); err != nil {
		return fmt.Errorf("%w; 回滚租户成员失败: %v", cause, err)
	}
	if err := h.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("%w; 回滚用户失败: %v", cause, err)
	}
	return cause
}

// getScheme 获取请求协议
func getScheme(c *gin.Context) string {
	if c.Request.TLS != nil {
		return "https"
	}
	if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
