package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ==================== Impersonation Handler ====================
// 平台管理员访问租户数据的审批制 Impersonation 机制

// ImpersonationHandler Impersonation 处理器
type ImpersonationHandler struct {
	repo              *repository.ImpersonationRepository
	tenantRepo        *repository.TenantRepository
	auditRepo         *repository.AuditLogRepository
	platformAuditRepo *repository.PlatformAuditLogRepository
	siteMessageRepo   *repository.SiteMessageRepository
}

// NewImpersonationHandler 创建 Impersonation 处理器
func NewImpersonationHandler() *ImpersonationHandler {
	return &ImpersonationHandler{
		repo:              repository.NewImpersonationRepository(),
		tenantRepo:        repository.NewTenantRepository(),
		auditRepo:         repository.NewAuditLogRepository(),
		platformAuditRepo: repository.NewPlatformAuditLogRepository(),
		siteMessageRepo:   repository.NewSiteMessageRepository(),
	}
}

// ==================== DTO ====================

type createImpersonationRequest struct {
	TenantID        string `json:"tenant_id" binding:"required"`
	Reason          string `json:"reason"`
	DurationMinutes int    `json:"duration_minutes" binding:"required,min=1,max=1440"`
}

type setApproversRequest struct {
	UserIDs []string `json:"user_ids" binding:"required"`
}

// ==================== 平台管理员 API ====================

// CreateRequest 提交 Impersonation 申请
// POST /api/v1/platform/impersonation/requests
func (h *ImpersonationHandler) CreateRequest(c *gin.Context) {
	var req createImpersonationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 解析租户 ID
	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	// 验证租户存在且活跃
	tenant, err := h.tenantRepo.GetByID(c.Request.Context(), tenantID)
	if err != nil {
		response.NotFound(c, "租户不存在")
		return
	}
	if tenant.Status != model.TenantStatusActive {
		response.BadRequest(c, "该租户已禁用，无法申请访问")
		return
	}

	// 检查是否已有 pending 或 active 状态的申请
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	existing, err := h.repo.GetActiveSession(c.Request.Context(), requesterID, tenantID)
	if err == nil && existing != nil {
		response.Conflict(c, "您已有该租户的活跃会话，无需重复申请")
		return
	}

	// 创建申请
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
		response.InternalError(c, "创建申请失败: "+err.Error())
		return
	}

	// 异步发送站内消息通知审批人
	go h.notifyApproversNewRequest(c.Request.Context(), impReq)

	response.Created(c, impReq)
}

// ListMyRequests 查询我的申请列表
// GET /api/v1/platform/impersonation/requests?page=1&page_size=10&status=xxx&tenant_name=xxx&reason=xxx
func (h *ImpersonationHandler) ListMyRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	status := c.Query("status")
	tenantName := GetStringFilter(c, "tenant_name")
	reason := GetStringFilter(c, "reason")

	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	requests, total, err := h.repo.ListByRequester(c.Request.Context(), requesterID, status, tenantName, reason, page, pageSize)
	if err != nil {
		response.InternalError(c, "查询申请列表失败: "+err.Error())
		return
	}

	// 在查询时顺便清理已过期的 approved / active 会话（写穿到数据库）
	// 这是一个 lazy 过期机制，确保用户看到的状态是最新的
	if affected, err := h.repo.ExpireOverdueSessions(c.Request.Context()); err == nil && affected > 0 {
		zap.L().Info("查询时自动过期 impersonation 会话", zap.Int64("affected", affected))
		// 重新查询以获取最新状态
		requests, total, err = h.repo.ListByRequester(c.Request.Context(), requesterID, status, tenantName, reason, page, pageSize)
		if err != nil {
			response.InternalError(c, "查询申请列表失败: "+err.Error())
			return
		}
	}

	response.List(c, requests, total, page, pageSize)
}

// GetRequest 获取申请详情
// GET /api/v1/platform/impersonation/requests/:id
func (h *ImpersonationHandler) GetRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return
	}

	// 验证是自己的申请
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权查看该申请")
		return
	}

	response.Success(c, req)
}

// EnterTenant 进入租户（开始或恢复 Impersonation 会话）
// POST /api/v1/platform/impersonation/requests/:id/enter
func (h *ImpersonationHandler) EnterTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return
	}

	// 验证权限
	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权操作该申请")
		return
	}
	if req.Status != model.ImpersonationStatusApproved {
		response.BadRequest(c, "该申请尚未批准或已使用")
		return
	}

	// 判断是首次进入还是重新进入
	if req.SessionExpiresAt != nil && time.Now().Before(*req.SessionExpiresAt) {
		// 重新进入：会话时间未过期，直接恢复 active 状态（不重设过期时间）
		if err := h.repo.ResumeSession(c.Request.Context(), id); err != nil {
			response.InternalError(c, "恢复会话失败: "+err.Error())
			return
		}
	} else if req.SessionExpiresAt != nil && !time.Now().Before(*req.SessionExpiresAt) {
		// 会话已过期，不允许进入
		response.BadRequest(c, "会话已过期，请重新申请")
		return
	} else {
		// 首次进入：创建新会话
		if err := h.repo.StartSession(c.Request.Context(), id, req.DurationMinutes); err != nil {
			response.InternalError(c, "开始会话失败: "+err.Error())
			return
		}
	}

	// 重新获取更新后的记录
	updated, _ := h.repo.GetByID(c.Request.Context(), id)

	// 异步写入审计日志（进入 = 登录租户）
	userIDPtr := &requesterID
	username := middleware.GetUsername(c)
	ipAddress := middleware.NormalizeIP(c.ClientIP())
	userAgent := c.Request.UserAgent()
	go h.writeImpersonationAudit(userIDPtr, username, ipAddress, userAgent, req.TenantID, req.TenantName, "impersonation_enter", id)

	response.Success(c, updated)
}

// ExitTenant 退出租户（暂离 Impersonation 视角）
// 如果会话时间未到期，状态回退到 approved（可重新进入）
// 如果已过期，则正常完结
// POST /api/v1/platform/impersonation/requests/:id/exit
func (h *ImpersonationHandler) ExitTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return
	}

	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权操作该申请")
		return
	}
	if req.Status != model.ImpersonationStatusActive {
		response.BadRequest(c, "该会话未处于活跃状态")
		return
	}

	// 检查会话是否仍在有效期内
	if req.SessionExpiresAt != nil && time.Now().Before(*req.SessionExpiresAt) {
		// 未过期 → 回退到 approved 状态（保留 session_expires_at，允许重新进入）
		if err := h.repo.PauseSession(c.Request.Context(), id); err != nil {
			response.InternalError(c, "暂离失败: "+err.Error())
			return
		}
		// 异步写入审计日志（退出 = 登出租户）
		userIDPtr := &requesterID
		username := middleware.GetUsername(c)
		ipAddress := middleware.NormalizeIP(c.ClientIP())
		userAgent := c.Request.UserAgent()
		go h.writeImpersonationAudit(userIDPtr, username, ipAddress, userAgent, req.TenantID, req.TenantName, "impersonation_exit", id)

		response.Message(c, "已暂离租户视角，可在到期前重新进入")
	} else {
		// 已过期 → 正常完结
		if err := h.repo.CompleteSession(c.Request.Context(), id); err != nil {
			response.InternalError(c, "结束会话失败: "+err.Error())
			return
		}
		// 异步写入审计日志（退出 = 登出租户）
		userIDPtr := &requesterID
		username := middleware.GetUsername(c)
		ipAddress := middleware.NormalizeIP(c.ClientIP())
		userAgent := c.Request.UserAgent()
		go h.writeImpersonationAudit(userIDPtr, username, ipAddress, userAgent, req.TenantID, req.TenantName, "impersonation_exit", id)

		response.Message(c, "会话已过期，已退出租户视角")
	}
}

// TerminateSession 终止会话（彻底结束，不可再进入）
// POST /api/v1/platform/impersonation/requests/:id/terminate
func (h *ImpersonationHandler) TerminateSession(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return
	}

	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if req.RequesterID != requesterID {
		response.Forbidden(c, "无权操作该申请")
		return
	}

	// 只有 active（进行中）或 approved（暂离/已批准且有会话）状态可以终止
	if req.Status != model.ImpersonationStatusActive && req.Status != model.ImpersonationStatusApproved {
		response.BadRequest(c, "该会话状态不允许终止")
		return
	}

	// 彻底结束会话 → completed
	if err := h.repo.CompleteSession(c.Request.Context(), id); err != nil {
		response.InternalError(c, "终止会话失败: "+err.Error())
		return
	}

	// 异步写入审计日志
	userIDPtr := &requesterID
	username := middleware.GetUsername(c)
	ipAddress := middleware.NormalizeIP(c.ClientIP())
	userAgent := c.Request.UserAgent()
	go h.writeImpersonationAudit(userIDPtr, username, ipAddress, userAgent, req.TenantID, req.TenantName, "impersonation_terminate", id)

	response.Message(c, "会话已终止")
}

// writeImpersonationAudit 异步写入 Impersonation 进入/退出审计日志
// 双写：platform_audit_logs + audit_logs（租户侧也能看到）
func (h *ImpersonationHandler) writeImpersonationAudit(
	userID *uuid.UUID, username, ipAddress, userAgent string,
	tenantID uuid.UUID, tenantName, action string, requestID uuid.UUID,
) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("Impersonation 审计日志写入失败 (panic)", zap.Any("error", r))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	statusCode := http.StatusOK

	// 1. 写入 platform_audit_logs
	platformLog := &model.PlatformAuditLog{
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
		RequestPath:    "/api/v1/platform/impersonation/requests/" + requestID.String() + "/" + action[len("impersonation_"):],
		ResponseStatus: &statusCode,
		Status:         "success",
		CreatedAt:      now,
	}
	if err := h.platformAuditRepo.Create(ctx, platformLog); err != nil {
		zap.L().Error("Impersonation 平台审计日志写入失败", zap.Error(err))
	}

	// 2. 写入 audit_logs（租户侧）
	auditLog := &model.AuditLog{
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
		RequestPath:    "/api/v1/platform/impersonation/requests/" + requestID.String() + "/" + action[len("impersonation_"):],
		ResponseStatus: &statusCode,
		Status:         "success",
		CreatedAt:      now,
	}
	if err := h.auditRepo.Create(ctx, auditLog); err != nil {
		zap.L().Error("Impersonation 租户审计日志写入失败", zap.Error(err))
	}
}

// CancelRequest 撤销申请
// POST /api/v1/platform/impersonation/requests/:id/cancel
func (h *ImpersonationHandler) CancelRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	requesterID, _ := uuid.Parse(middleware.GetUserID(c))
	if err := h.repo.CancelRequest(c.Request.Context(), id, requesterID); err != nil {
		response.BadRequest(c, "无法撤销：申请不存在或状态不允许")
		return
	}

	response.Message(c, "申请已撤销")
}

// ==================== 租户审批人 API ====================

// ListPending 查询当前租户的待审批申请
// GET /api/v1/tenant/impersonation/pending
func (h *ImpersonationHandler) ListPending(c *gin.Context) {
	tenantID := middleware.GetTenantUUID(c)

	requests, err := h.repo.ListPendingByTenant(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "查询待审批列表失败: "+err.Error())
		return
	}

	response.Success(c, requests)
}

// ListHistory 查询当前租户的所有审批记录（分页 + 搜索）
// GET /api/v1/tenant/impersonation/history
func (h *ImpersonationHandler) ListHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	tenantID := middleware.GetTenantUUID(c)

	// 搜索过滤参数
	filters := map[string]string{
		"requester_name": c.Query("requester_name"),
		"reason":         c.Query("reason"),
		"status":         c.Query("status"),
	}

	// 先清理过期会话，确保状态最新
	if affected, err := h.repo.ExpireOverdueSessions(c.Request.Context()); err == nil && affected > 0 {
		zap.L().Info("查询审批记录时自动过期 impersonation 会话", zap.Int64("affected", affected))
	}

	requests, total, err := h.repo.ListByTenant(c.Request.Context(), tenantID, page, pageSize, filters)
	if err != nil {
		response.InternalError(c, "查询审批记录失败: "+err.Error())
		return
	}

	response.List(c, requests, total, page, pageSize)
}

// Approve 审批通过
// POST /api/v1/tenant/impersonation/:id/approve
func (h *ImpersonationHandler) Approve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return
	}

	// 验证申请属于当前租户
	tenantID := middleware.GetTenantUUID(c)
	if req.TenantID != tenantID {
		response.Forbidden(c, "无权操作该申请")
		return
	}

	// 验证审批权限
	userID, _ := uuid.Parse(middleware.GetUserID(c))
	isApprover, err := h.repo.IsApprover(c.Request.Context(), tenantID, userID)
	if err != nil || !isApprover {
		response.Forbidden(c, "您没有审批权限")
		return
	}

	if req.Status != model.ImpersonationStatusPending {
		response.BadRequest(c, "该申请已处理")
		return
	}

	if err := h.repo.UpdateStatus(c.Request.Context(), id, model.ImpersonationStatusApproved, &userID); err != nil {
		response.InternalError(c, "审批失败: "+err.Error())
		return
	}

	// 异步发送站内消息通知申请人（平台管理员）
	go h.notifyRequesterDecision(c.Request.Context(), req, true, middleware.GetUsername(c))

	response.Message(c, "已批准")
}

// Reject 审批拒绝
// POST /api/v1/tenant/impersonation/:id/reject
func (h *ImpersonationHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的申请 ID")
		return
	}

	req, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "申请不存在")
		return
	}

	tenantID := middleware.GetTenantUUID(c)
	if req.TenantID != tenantID {
		response.Forbidden(c, "无权操作该申请")
		return
	}

	userID, _ := uuid.Parse(middleware.GetUserID(c))
	isApprover, err := h.repo.IsApprover(c.Request.Context(), tenantID, userID)
	if err != nil || !isApprover {
		response.Forbidden(c, "您没有审批权限")
		return
	}

	if req.Status != model.ImpersonationStatusPending {
		response.BadRequest(c, "该申请已处理")
		return
	}

	if err := h.repo.UpdateStatus(c.Request.Context(), id, model.ImpersonationStatusRejected, &userID); err != nil {
		response.InternalError(c, "拒绝失败: "+err.Error())
		return
	}

	// 异步发送站内消息通知申请人（平台管理员）
	go h.notifyRequesterDecision(c.Request.Context(), req, false, middleware.GetUsername(c))

	response.Message(c, "已拒绝")
}

// ==================== 审批组管理 API ====================

// GetApprovers 获取当前租户的审批人列表
// GET /api/v1/tenant/settings/impersonation-approvers
func (h *ImpersonationHandler) GetApprovers(c *gin.Context) {
	tenantID := middleware.GetTenantUUID(c)
	approvers, err := h.repo.GetApprovers(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "查询审批人列表失败: "+err.Error())
		return
	}
	response.Success(c, approvers)
}

// SetApprovers 设置当前租户的审批人
// PUT /api/v1/tenant/settings/impersonation-approvers
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

	tenantID := middleware.GetTenantUUID(c)
	if err := h.repo.SetApprovers(c.Request.Context(), tenantID, userIDs); err != nil {
		response.InternalError(c, "设置审批人失败: "+err.Error())
		return
	}

	response.Message(c, "审批人已更新")
}

// ==================== 站内消息通知辅助方法 ====================

// notifyApproversNewRequest 异步向租户审批人发送站内消息：有新的提权申请待审批
func (h *ImpersonationHandler) notifyApproversNewRequest(ctx context.Context, impReq *model.ImpersonationRequest) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("notifyApproversNewRequest panic", zap.Any("error", r))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg := &model.SiteMessage{
		TenantID:       &impReq.TenantID,
		TargetTenantID: &impReq.TenantID, // 只对该租户的用户可见
		Category:       model.SiteMessageCategoryServiceNotice,
		Title:          "新的临时提权申请待审批",
		Content:        impReq.RequesterName + " 申请临时访问本租户（" + impReq.TenantName + "），申请时长 " + formatMinutes(impReq.DurationMinutes) + "，请及时处理。",
	}
	if err := h.siteMessageRepo.Create(ctx, msg); err != nil {
		zap.L().Error("发送审批人站内消息失败", zap.Error(err))
	}
}

// notifyRequesterDecision 异步向申请人（平台管理员）发送站内消息：审批结果
func (h *ImpersonationHandler) notifyRequesterDecision(ctx context.Context, impReq *model.ImpersonationRequest, approved bool, approverName string) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("notifyRequesterDecision panic", zap.Any("error", r))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var title, content string
	if approved {
		title = "临时提权申请已批准"
		content = "您申请访问租户「" + impReq.TenantName + "」的提权请求已由 " + approverName + " 批准，请在 " + formatMinutes(impReq.DurationMinutes) + " 内完成操作。"
	} else {
		title = "临时提权申请已拒绝"
		content = "您申请访问租户「" + impReq.TenantName + "」的提权请求已由 " + approverName + " 拒绝。"
	}

	// TargetTenantID=nil 表示广播（平台管理员查询时 tenantID=nil 不过滤，可以看到所有消息）
	// TenantID 设为申请单所属租户，避免 Create() 自动写入 DefaultTenantID
	msg := &model.SiteMessage{
		TenantID:       &impReq.TenantID,
		TargetTenantID: nil, // nil = 全局广播，平台管理员可见
		Category:       model.SiteMessageCategoryServiceNotice,
		Title:          title,
		Content:        content,
	}
	if err := h.siteMessageRepo.Create(ctx, msg); err != nil {
		zap.L().Error("发送申请人站内消息失败", zap.Error(err))
	}
}

// formatMinutes 将分钟转化为可读文本
func formatMinutes(minutes int) string {
	if minutes >= 60 {
		h := minutes / 60
		m := minutes % 60
		if m == 0 {
			return fmt.Sprintf("%d 小时", h)
		}
		return fmt.Sprintf("%d 小时 %d 分钟", h, m)
	}
	return fmt.Sprintf("%d 分钟", minutes)
}
