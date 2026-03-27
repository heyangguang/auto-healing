package httpapi

import (
	"errors"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func openImpersonationRequestMessage(req *model.ImpersonationRequest) string {
	switch req.Status {
	case model.ImpersonationStatusPending:
		return "您已有该租户的待审批申请，请勿重复提交"
	case model.ImpersonationStatusApproved:
		if req.SessionExpiresAt != nil {
			return "您已有该租户的可恢复会话，请直接重新进入"
		}
		return "您已有该租户的已批准申请，请直接进入租户视角"
	case model.ImpersonationStatusActive:
		return "您已有该租户的活跃会话，无需重复申请"
	default:
		return "您已有该租户的进行中申请，无需重复提交"
	}
}

func (h *ImpersonationHandler) refreshRequesterSessions(c *gin.Context, requesterID uuid.UUID, status string, tenantName, reason query.StringFilter, page, pageSize int, requests []model.ImpersonationRequest, total int64, err error) ([]model.ImpersonationRequest, int64, error) {
	affected, expireErr := h.repo.ExpireOverdueSessions(c.Request.Context())
	if expireErr != nil {
		return nil, 0, expireErr
	}
	if affected > 0 {
		logger.API("IMPERSONATION").Info("查询时自动过期 impersonation 会话: affected=%d", affected)
		return h.repo.ListByRequester(c.Request.Context(), requesterID, status, tenantName, reason, page, pageSize)
	}
	return requests, total, err
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
		respondImpersonationLookupError(c, err)
		return nil, false
	}
	requesterID, ok := requireImpersonationUserID(c)
	if !ok {
		return nil, false
	}
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
		respondImpersonationLookupError(c, err)
		return nil, uuid.Nil, uuid.Nil, false
	}
	requesterID, ok := requireImpersonationUserID(c)
	if !ok {
		return nil, uuid.Nil, uuid.Nil, false
	}
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权操作该申请")
		return nil, uuid.Nil, uuid.Nil, false
	}
	return req, requesterID, id, true
}

func requireImpersonationUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		respondInternalError(c, "IMPERSONATION", "用户上下文缺失", err)
		return uuid.Nil, false
	}
	return userID, true
}

func respondImpersonationLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "申请不存在")
		return
	}
	respondInternalError(c, "IMPERSONATION", "查询申请失败", err)
}

func respondImpersonationTenantLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "租户不存在")
		return
	}
	respondInternalError(c, "IMPERSONATION", "查询租户失败", err)
}

func respondImpersonationCancelError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.BadRequest(c, "无法撤销：申请不存在或状态不允许")
		return
	}
	respondInternalError(c, "IMPERSONATION", "撤销申请失败", err)
}
