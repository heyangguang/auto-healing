package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/company/auto-healing/internal/pkg/response"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errImpersonationSessionExpired = errors.New("impersonation session expired")

// CreateRequest 提交 Impersonation 申请
func (h *ImpersonationHandler) CreateRequest(c *gin.Context) {
	var req createImpersonationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.tenantRepo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		respondImpersonationTenantLookupError(c, err)
		return
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "该租户已禁用，无法申请访问")
		return
	}

	requesterID, ok := requireImpersonationUserID(c)
	if !ok {
		return
	}
	existing, err := h.repo.GetOpenRequest(c.Request.Context(), requesterID, tenantID)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "检查现有申请失败", err)
		return
	}
	if existing != nil {
		response.Conflict(c, openImpersonationRequestMessage(existing))
		return
	}

	impReq := &model.ImpersonationRequest{
		RequesterID:     requesterID,
		RequesterName:   middleware.GetUsername(c),
		TenantID:        tenantID,
		TenantName:      tenant.Name,
		Reason:          req.Reason,
		DurationMinutes: req.DurationMinutes,
		Status:          model.ImpersonationStatusPending,
	}
	if err := h.repo.Create(c.Request.Context(), impReq); err != nil {
		respondInternalError(c, "IMPERSONATION", "创建申请失败", err)
		return
	}

	platformlifecycle.Go(func(rootCtx context.Context) {
		h.notifyApproversNewRequest(rootCtx, impReq)
	})
	response.Created(c, impReq)
}

// ListMyRequests 查询我的申请列表
func (h *ImpersonationHandler) ListMyRequests(c *gin.Context) {
	page, pageSize := parsePagination(c, 10)
	status := c.Query("status")
	tenantName := GetStringFilter(c, "tenant_name")
	reason := GetStringFilter(c, "reason")
	requesterID, ok := requireImpersonationUserID(c)
	if !ok {
		return
	}

	requests, total, err := h.repo.ListByRequester(c.Request.Context(), requesterID, status, tenantName, reason, page, pageSize)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "查询申请列表失败", err)
		return
	}
	if requests, total, err = h.refreshRequesterSessions(c, requesterID, status, tenantName, reason, page, pageSize, requests, total, err); err != nil {
		respondInternalError(c, "IMPERSONATION", "查询申请列表失败", err)
		return
	}
	response.List(c, requests, total, page, pageSize)
}

// GetRequest 获取申请详情
func (h *ImpersonationHandler) GetRequest(c *gin.Context) {
	id, ok := parseImpersonationRequestID(c)
	if !ok {
		return
	}

	req, ok := h.loadRequesterOwnedRequest(c, id)
	if !ok {
		return
	}
	response.Success(c, req)
}

// EnterTenant 进入租户（开始或恢复 Impersonation 会话）
func (h *ImpersonationHandler) EnterTenant(c *gin.Context) {
	req, requesterID, id, ok := h.loadOperableRequest(c)
	if !ok {
		return
	}
	if req.Status != model.ImpersonationStatusApproved {
		response.BadRequest(c, "该申请尚未批准或已使用")
		return
	}

	if err := h.enterOrResumeSession(c, req, id); err != nil {
		if errors.Is(err, errImpersonationSessionExpired) {
			response.BadRequest(c, "会话已过期，请重新申请")
			return
		}
		respondInternalError(c, "IMPERSONATION", "开始会话失败", err)
		return
	}
	updated, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "获取会话详情失败", err)
		return
	}
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.writeImpersonationAudit(rootCtx, &requesterID, middleware.GetUsername(c), middleware.NormalizeIP(c.ClientIP()), c.Request.UserAgent(), req.TenantID, req.TenantName, "impersonation_enter", id)
	})
	response.Success(c, updated)
}

// ExitTenant 退出租户（暂离 Impersonation 视角）
func (h *ImpersonationHandler) ExitTenant(c *gin.Context) {
	req, requesterID, id, ok := h.loadOperableRequest(c)
	if !ok {
		return
	}
	if req.Status != model.ImpersonationStatusActive {
		response.BadRequest(c, "该会话未处于活跃状态")
		return
	}

	if req.SessionExpiresAt != nil && time.Now().Before(*req.SessionExpiresAt) {
		if err := h.repo.PauseSession(c.Request.Context(), id); err != nil {
			respondInternalError(c, "IMPERSONATION", "暂离失败", err)
			return
		}
		platformlifecycle.Go(func(rootCtx context.Context) {
			h.writeImpersonationAudit(rootCtx, &requesterID, middleware.GetUsername(c), middleware.NormalizeIP(c.ClientIP()), c.Request.UserAgent(), req.TenantID, req.TenantName, "impersonation_exit", id)
		})
		response.Message(c, "已暂离租户视角，可在到期前重新进入")
		return
	}

	if err := h.repo.CompleteSession(c.Request.Context(), id); err != nil {
		respondInternalError(c, "IMPERSONATION", "结束会话失败", err)
		return
	}
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.writeImpersonationAudit(rootCtx, &requesterID, middleware.GetUsername(c), middleware.NormalizeIP(c.ClientIP()), c.Request.UserAgent(), req.TenantID, req.TenantName, "impersonation_exit", id)
	})
	response.Message(c, "会话已过期，已退出租户视角")
}

// TerminateSession 终止会话
func (h *ImpersonationHandler) TerminateSession(c *gin.Context) {
	req, requesterID, id, ok := h.loadOperableRequest(c)
	if !ok {
		return
	}
	if req.Status != model.ImpersonationStatusActive && req.Status != model.ImpersonationStatusApproved {
		response.BadRequest(c, "该会话状态不允许终止")
		return
	}
	if err := h.repo.CompleteSession(c.Request.Context(), id); err != nil {
		respondInternalError(c, "IMPERSONATION", "终止会话失败", err)
		return
	}
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.writeImpersonationAudit(rootCtx, &requesterID, middleware.GetUsername(c), middleware.NormalizeIP(c.ClientIP()), c.Request.UserAgent(), req.TenantID, req.TenantName, "impersonation_terminate", id)
	})
	response.Message(c, "会话已终止")
}

// CancelRequest 撤销申请
func (h *ImpersonationHandler) CancelRequest(c *gin.Context) {
	id, ok := parseImpersonationRequestID(c)
	if !ok {
		return
	}
	requesterID, ok := requireImpersonationUserID(c)
	if !ok {
		return
	}
	if err := h.repo.CancelRequest(c.Request.Context(), id, requesterID); err != nil {
		respondImpersonationCancelError(c, err)
		return
	}
	response.Message(c, "申请已撤销")
}
