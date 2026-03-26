package handler

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
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
		response.NotFound(c, "租户不存在")
		return
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "该租户已禁用，无法申请访问")
		return
	}

	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	existing, err := h.repo.GetActiveSession(c.Request.Context(), requesterID, tenantID)
	if err == nil && existing != nil {
		response.Conflict(c, "您已有该租户的活跃会话，无需重复申请")
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

	goHandlerTask(func(rootCtx context.Context) {
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
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))

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

func (h *ImpersonationHandler) refreshRequesterSessions(c *gin.Context, requesterID uuid.UUID, status string, tenantName, reason query.StringFilter, page, pageSize int, requests []model.ImpersonationRequest, total int64, err error) ([]model.ImpersonationRequest, int64, error) {
	if affected, expireErr := h.repo.ExpireOverdueSessions(c.Request.Context()); expireErr == nil && affected > 0 {
		logger.API("IMPERSONATION").Info("查询时自动过期 impersonation 会话: affected=%d", affected)
		return h.repo.ListByRequester(c.Request.Context(), requesterID, status, tenantName, reason, page, pageSize)
	}
	return requests, total, err
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
	updated, _ := h.repo.GetByID(c.Request.Context(), id)
	goHandlerTask(func(rootCtx context.Context) {
		h.writeImpersonationAudit(rootCtx, &requesterID, middleware.GetUsername(c), middleware.NormalizeIP(c.ClientIP()), c.Request.UserAgent(), req.TenantID, req.TenantName, "impersonation_enter", id)
	})
	response.Success(c, updated)
}

func (h *ImpersonationHandler) enterOrResumeSession(c *gin.Context, req *model.ImpersonationRequest, id uuid.UUID) error {
	if req.SessionExpiresAt != nil && time.Now().Before(*req.SessionExpiresAt) {
		return h.repo.ResumeSession(c.Request.Context(), id)
	}
	if req.SessionExpiresAt != nil && !time.Now().Before(*req.SessionExpiresAt) {
		return errImpersonationSessionExpired
	}
	return h.repo.StartSession(c.Request.Context(), id, req.DurationMinutes)
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
		goHandlerTask(func(rootCtx context.Context) {
			h.writeImpersonationAudit(rootCtx, &requesterID, middleware.GetUsername(c), middleware.NormalizeIP(c.ClientIP()), c.Request.UserAgent(), req.TenantID, req.TenantName, "impersonation_exit", id)
		})
		response.Message(c, "已暂离租户视角，可在到期前重新进入")
		return
	}

	if err := h.repo.CompleteSession(c.Request.Context(), id); err != nil {
		respondInternalError(c, "IMPERSONATION", "结束会话失败", err)
		return
	}
	goHandlerTask(func(rootCtx context.Context) {
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
	goHandlerTask(func(rootCtx context.Context) {
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
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if err := h.repo.CancelRequest(c.Request.Context(), id, requesterID); err != nil {
		response.BadRequest(c, "无法撤销：申请不存在或状态不允许")
		return
	}
	response.Message(c, "申请已撤销")
}

func parseImpersonationRequestID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return uuid.Nil, false
	}
	return id, true
}

func (h *ImpersonationHandler) loadRequesterOwnedRequest(c *gin.Context, id uuid.UUID) (*model.ImpersonationRequest, bool) {
	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return nil, false
	}
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权查看该申请")
		return nil, false
	}
	return req, true
}

func (h *ImpersonationHandler) loadOperableRequest(c *gin.Context) (*model.ImpersonationRequest, uuid.UUID, uuid.UUID, bool) {
	id, ok := parseImpersonationRequestID(c)
	if !ok {
		return nil, uuid.Nil, uuid.Nil, false
	}
	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return nil, uuid.Nil, uuid.Nil, false
	}
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权操作该申请")
		return nil, uuid.Nil, uuid.Nil, false
	}
	return req, requesterID, id, true
}
