package handler

import (
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PlatformAuditHandler 平台审计日志处理器
type PlatformAuditHandler struct {
	repo *repository.PlatformAuditLogRepository
}

// NewPlatformAuditHandler 创建平台审计日志处理器
func NewPlatformAuditHandler() *PlatformAuditHandler {
	return &PlatformAuditHandler{
		repo: repository.NewPlatformAuditLogRepository(),
	}
}

// ListPlatformAuditLogs 获取平台审计日志列表
// GET /api/v1/platform/audit-logs?page=1&page_size=20&category=login&action=create&...
func (h *PlatformAuditHandler) ListPlatformAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.PlatformAuditListOptions{
		Page:         page,
		PageSize:     pageSize,
		Search:       c.Query("search"),
		Category:     c.Query("category"),
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		Username:     c.Query("username"),
		Status:       c.Query("status"),
		SortBy:       c.Query("sort_by"),
		SortOrder:    c.Query("sort_order"),
	}

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := uuid.Parse(userIDStr); err == nil {
			opts.UserID = &uid
		}
	}

	if afterStr := c.Query("created_after"); afterStr != "" {
		if after, err := time.Parse(time.RFC3339, afterStr); err == nil {
			opts.CreatedAfter = &after
		}
	}
	if beforeStr := c.Query("created_before"); beforeStr != "" {
		if before, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			opts.CreatedBefore = &before
		}
	}

	logs, total, err := h.repo.List(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	result := make([]gin.H, len(logs))
	for i, log := range logs {
		isRisk := repository.IsHighRisk(log.Action, log.ResourceType)
		riskLevel := "normal"
		riskReason := ""
		if isRisk {
			riskLevel = "high"
			riskReason = repository.GetRiskReason(log.Action, log.ResourceType)
		}

		result[i] = gin.H{
			"id":              log.ID,
			"user_id":         log.UserID,
			"username":        log.Username,
			"ip_address":      log.IPAddress,
			"user_agent":      log.UserAgent,
			"category":        log.Category,
			"action":          log.Action,
			"resource_type":   log.ResourceType,
			"resource_id":     log.ResourceID,
			"resource_name":   log.ResourceName,
			"request_method":  log.RequestMethod,
			"request_path":    log.RequestPath,
			"request_body":    log.RequestBody,
			"response_status": log.ResponseStatus,
			"changes":         log.Changes,
			"status":          log.Status,
			"error_message":   log.ErrorMessage,
			"risk_level":      riskLevel,
			"risk_reason":     riskReason,
			"created_at":      log.CreatedAt,
		}
	}

	response.List(c, result, total, page, pageSize)
}

// GetPlatformAuditLog 获取平台审计日志详情
// GET /api/v1/platform/audit-logs/:id
func (h *PlatformAuditHandler) GetPlatformAuditLog(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	log, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if log == nil {
		response.NotFound(c, "平台审计日志不存在")
		return
	}

	isRisk := repository.IsHighRisk(log.Action, log.ResourceType)
	riskLevel := "normal"
	riskReason := ""
	if isRisk {
		riskLevel = "high"
		riskReason = repository.GetRiskReason(log.Action, log.ResourceType)
	}

	response.Success(c, gin.H{
		"id":              log.ID,
		"user_id":         log.UserID,
		"username":        log.Username,
		"ip_address":      log.IPAddress,
		"user_agent":      log.UserAgent,
		"category":        log.Category,
		"action":          log.Action,
		"resource_type":   log.ResourceType,
		"resource_id":     log.ResourceID,
		"resource_name":   log.ResourceName,
		"request_method":  log.RequestMethod,
		"request_path":    log.RequestPath,
		"request_body":    log.RequestBody,
		"response_status": log.ResponseStatus,
		"changes":         log.Changes,
		"status":          log.Status,
		"error_message":   log.ErrorMessage,
		"risk_level":      riskLevel,
		"risk_reason":     riskReason,
		"created_at":      log.CreatedAt,
	})
}

// GetPlatformAuditStats 获取平台审计统计
// GET /api/v1/platform/audit-logs/stats
func (h *PlatformAuditHandler) GetPlatformAuditStats(c *gin.Context) {
	stats, err := h.repo.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

// GetPlatformAuditTrend 获取平台审计趋势
// GET /api/v1/platform/audit-logs/trend?days=30
func (h *PlatformAuditHandler) GetPlatformAuditTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 {
		days = 30
	}

	items, err := h.repo.GetTrend(c.Request.Context(), days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items": items,
		"days":  days,
	})
}

// GetPlatformUserRanking 获取平台用户操作排行
// GET /api/v1/platform/audit-logs/user-ranking?limit=10&days=7
func (h *PlatformAuditHandler) GetPlatformUserRanking(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	rankings, err := h.repo.GetUserRanking(c.Request.Context(), limit, days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"rankings": rankings,
		"limit":    limit,
		"days":     days,
	})
}

// GetPlatformHighRiskLogs 获取平台高危操作日志
// GET /api/v1/platform/audit-logs/high-risk?page=1&page_size=20
func (h *PlatformAuditHandler) GetPlatformHighRiskLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.repo.GetHighRiskLogs(c.Request.Context(), page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	result := make([]gin.H, len(logs))
	for i, log := range logs {
		result[i] = gin.H{
			"id":            log.ID,
			"username":      log.Username,
			"category":      log.Category,
			"action":        log.Action,
			"resource_type": log.ResourceType,
			"resource_name": log.ResourceName,
			"status":        log.Status,
			"ip_address":    log.IPAddress,
			"risk_reason":   repository.GetRiskReason(log.Action, log.ResourceType),
			"created_at":    log.CreatedAt,
		}
	}

	response.List(c, result, total, page, pageSize)
}
