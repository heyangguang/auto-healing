package handler

import (
	"context"
	"errors"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListPending 查询当前租户的待审批申请
func (h *ImpersonationHandler) ListPending(c *gin.Context) {
	tenantID, ok := requireTenantID(c, "IMPERSONATION")
	if !ok {
		return
	}
	requests, err := h.repo.ListPendingByTenant(c.Request.Context(), tenantID)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "查询待审批列表失败", err)
		return
	}
	response.Success(c, requests)
}

// ListHistory 查询当前租户的所有审批记录
func (h *ImpersonationHandler) ListHistory(c *gin.Context) {
	page, pageSize := parsePagination(c, 10)
	tenantID, ok := requireTenantID(c, "IMPERSONATION")
	if !ok {
		return
	}
	filters := map[string]string{
		"requester_name": c.Query("requester_name"),
		"reason":         c.Query("reason"),
		"status":         c.Query("status"),
	}

	affected, err := h.repo.ExpireOverdueSessions(c.Request.Context())
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "查询审批记录失败", err)
		return
	}
	if affected > 0 {
		logger.API("IMPERSONATION").Info("查询审批记录时自动过期 impersonation 会话: affected=%d", affected)
	}
	requests, total, err := h.repo.ListByTenant(c.Request.Context(), tenantID, page, pageSize, filters)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "查询审批记录失败", err)
		return
	}
	response.List(c, requests, total, page, pageSize)
}

// Approve 审批通过
func (h *ImpersonationHandler) Approve(c *gin.Context) {
	req, userID, ok := h.loadTenantApprovalRequest(c)
	if !ok {
		return
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), req.ID, model.ImpersonationStatusApproved, &userID); err != nil {
		if errors.Is(err, repository.ErrImpersonationRequestNotPending) {
			response.Conflict(c, "该申请已被其他审批人处理")
			return
		}
		respondInternalError(c, "IMPERSONATION", "审批失败", err)
		return
	}
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.notifyRequesterDecision(rootCtx, req, true, middleware.GetUsername(c))
	})
	response.Message(c, "已批准")
}

// Reject 审批拒绝
func (h *ImpersonationHandler) Reject(c *gin.Context) {
	req, userID, ok := h.loadTenantApprovalRequest(c)
	if !ok {
		return
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), req.ID, model.ImpersonationStatusRejected, &userID); err != nil {
		if errors.Is(err, repository.ErrImpersonationRequestNotPending) {
			response.Conflict(c, "该申请已被其他审批人处理")
			return
		}
		respondInternalError(c, "IMPERSONATION", "拒绝失败", err)
		return
	}
	platformlifecycle.Go(func(rootCtx context.Context) {
		h.notifyRequesterDecision(rootCtx, req, false, middleware.GetUsername(c))
	})
	response.Message(c, "已拒绝")
}

func (h *ImpersonationHandler) loadTenantApprovalRequest(c *gin.Context) (*model.ImpersonationRequest, uuid.UUID, bool) {
	id, ok := parseImpersonationRequestID(c)
	if !ok {
		return nil, uuid.Nil, false
	}
	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondImpersonationLookupError(c, err)
		return nil, uuid.Nil, false
	}

	tenantID, ok := requireTenantID(c, "IMPERSONATION")
	if !ok {
		return nil, uuid.Nil, false
	}
	if req.TenantID != tenantID {
		response.Forbidden(c, "无权操作该申请")
		return nil, uuid.Nil, false
	}

	userID, ok := requireImpersonationUserID(c)
	if !ok {
		return nil, uuid.Nil, false
	}
	isApprover, err := h.repo.IsApprover(c.Request.Context(), tenantID, userID)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "校验审批权限失败", err)
		return nil, uuid.Nil, false
	}
	if !isApprover {
		response.Forbidden(c, "您没有审批权限")
		return nil, uuid.Nil, false
	}
	if req.Status != model.ImpersonationStatusPending {
		response.BadRequest(c, "该申请已处理")
		return nil, uuid.Nil, false
	}
	return req, userID, true
}

// GetApprovers 获取当前租户的审批人列表
func (h *ImpersonationHandler) GetApprovers(c *gin.Context) {
	tenantID, ok := requireTenantID(c, "IMPERSONATION")
	if !ok {
		return
	}
	approvers, err := h.repo.GetApprovers(c.Request.Context(), tenantID)
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "查询审批人列表失败", err)
		return
	}
	response.Success(c, approvers)
}

// SetApprovers 设置当前租户的审批人
func (h *ImpersonationHandler) SetApprovers(c *gin.Context) {
	var req setApproversRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userIDs := make([]uuid.UUID, 0, len(req.UserIDs))
	for _, idStr := range req.UserIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.BadRequest(c, "无效的用户 ID: "+idStr)
			return
		}
		userIDs = append(userIDs, id)
	}
	tenantID, ok := requireTenantID(c, "IMPERSONATION")
	if !ok {
		return
	}
	if err := h.repo.SetApprovers(c.Request.Context(), tenantID, userIDs); err != nil {
		respondInternalError(c, "IMPERSONATION", "设置审批人失败", err)
		return
	}
	response.Message(c, "审批人已更新")
}
