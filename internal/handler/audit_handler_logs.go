package handler

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListAuditLogs 获取审计日志列表
func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts, err := buildAuditListOptions(c, page, pageSize)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	logs, total, err := h.repo.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计日志列表失败", err)
		return
	}

	result := make([]gin.H, len(logs))
	for i, log := range logs {
		riskLevel, riskReason := auditRiskFields(log.Action, log.ResourceType)
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
			"request_body":    sanitizeAuditPayload(log.RequestBody),
			"response_status": log.ResponseStatus,
			"changes":         sanitizeAuditPayload(log.Changes),
			"status":          log.Status,
			"error_message":   log.ErrorMessage,
			"risk_level":      riskLevel,
			"risk_reason":     riskReason,
			"created_at":      log.CreatedAt,
			"user":            log.User,
		}
	}
	response.List(c, result, total, page, pageSize)
}

// GetAuditLog 获取审计日志详情
func (h *AuditHandler) GetAuditLog(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}
	log, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计日志详情失败", err)
		return
	}
	if log == nil {
		response.NotFound(c, "审计日志不存在")
		return
	}
	riskLevel, riskReason := auditRiskFields(log.Action, log.ResourceType)
	response.Success(c, gin.H{
		"id":              log.ID,
		"user_id":         log.UserID,
		"username":        log.Username,
		"ip_address":      log.IPAddress,
		"user_agent":      log.UserAgent,
		"action":          log.Action,
		"resource_type":   log.ResourceType,
		"resource_id":     log.ResourceID,
		"resource_name":   log.ResourceName,
		"request_method":  log.RequestMethod,
		"request_path":    log.RequestPath,
		"request_body":    sanitizeAuditPayload(log.RequestBody),
		"response_status": log.ResponseStatus,
		"changes":         sanitizeAuditPayload(log.Changes),
		"status":          log.Status,
		"error_message":   log.ErrorMessage,
		"risk_level":      riskLevel,
		"risk_reason":     riskReason,
		"created_at":      log.CreatedAt,
		"user":            log.User,
	})
}

// GetHighRiskLogs 获取高危操作日志
func (h *AuditHandler) GetHighRiskLogs(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	logs, total, err := h.repo.GetHighRiskLogs(c.Request.Context(), page, pageSize)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取高危审计日志失败", err)
		return
	}

	result := make([]gin.H, len(logs))
	for i, log := range logs {
		_, riskReason := auditRiskFields(log.Action, log.ResourceType)
		result[i] = gin.H{
			"id":            log.ID,
			"username":      log.Username,
			"action":        log.Action,
			"resource_type": log.ResourceType,
			"resource_name": log.ResourceName,
			"status":        log.Status,
			"ip_address":    log.IPAddress,
			"risk_reason":   riskReason,
			"created_at":    log.CreatedAt,
			"user":          log.User,
		}
	}
	response.List(c, result, total, page, pageSize)
}

// ExportAuditLogs 导出审计日志为 CSV
func (h *AuditHandler) ExportAuditLogs(c *gin.Context) {
	opts, err := buildAuditListOptions(c, 1, 10000)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	opts.SortBy = "created_at"
	opts.SortOrder = "desc"
	logs, _, err := h.repo.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "AUDIT", "导出审计日志失败", err)
		return
	}
	writeAuditCSV(c, logs)
}
