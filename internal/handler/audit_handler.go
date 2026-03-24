package handler

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuditHandler 审计日志处理器
type AuditHandler struct {
	repo *repository.AuditLogRepository
}

// NewAuditHandler 创建审计日志处理器
func NewAuditHandler() *AuditHandler {
	return &AuditHandler{
		repo: repository.NewAuditLogRepository(),
	}
}

// ListAuditLogs 获取审计日志列表
// GET /api/v1/audit-logs?page=1&page_size=20&action=create&resource_type=plugin&username=admin&status=success&risk_level=high&search=xxx&created_after=xxx&created_before=xxx&sort_by=created_at&sort_order=desc
func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.AuditLogListOptions{
		Page:         page,
		PageSize:     pageSize,
		Search:       GetStringFilter(c, "search"),
		Category:     c.Query("category"),
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		Username:     GetStringFilter(c, "username"),
		Status:       c.Query("status"),
		RiskLevel:    c.Query("risk_level"),
		RequestPath:  GetStringFilter(c, "request_path"),
		SortBy:       c.Query("sort_by"),
		SortOrder:    c.Query("sort_order"),
	}

	// 解析排除过滤参数（逗号分隔）
	if ea := c.Query("exclude_action"); ea != "" {
		opts.ExcludeActions = splitAndTrim(ea)
	}
	if ert := c.Query("exclude_resource_type"); ert != "" {
		opts.ExcludeResourceTypes = splitAndTrim(ert)
	}

	// 解析 user_id
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := uuid.Parse(userIDStr); err == nil {
			opts.UserID = &uid
		}
	}

	// 解析时间参数
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

	// 为每条记录添加 risk_level 和 risk_reason
	result := make([]gin.H, len(logs))
	for i, log := range logs {
		riskLevel := repository.GetRiskLevel(log.Action, log.ResourceType)
		riskReason := repository.GetRiskReason(log.Action, log.ResourceType)

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
			"user":            log.User,
		}
	}

	response.List(c, result, total, page, pageSize)
}

// GetAuditLog 获取审计日志详情
// GET /api/v1/audit-logs/:id
func (h *AuditHandler) GetAuditLog(c *gin.Context) {
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
		response.NotFound(c, "审计日志不存在")
		return
	}

	// 添加风险标记
	riskLevel := repository.GetRiskLevel(log.Action, log.ResourceType)
	riskReason := repository.GetRiskReason(log.Action, log.ResourceType)

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
		"request_body":    log.RequestBody,
		"response_status": log.ResponseStatus,
		"changes":         log.Changes,
		"status":          log.Status,
		"error_message":   log.ErrorMessage,
		"risk_level":      riskLevel,
		"risk_reason":     riskReason,
		"created_at":      log.CreatedAt,
		"user":            log.User,
	})
}

// GetAuditStats 获取审计统计概览
// GET /api/v1/audit-logs/stats
func (h *AuditHandler) GetAuditStats(c *gin.Context) {
	stats, err := h.repo.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, stats)
}

// GetUserRanking 获取用户操作排行榜
// GET /api/v1/audit-logs/user-ranking?limit=10&days=7
func (h *AuditHandler) GetUserRanking(c *gin.Context) {
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

// GetActionGrouping 按操作类型分组统计
// GET /api/v1/audit-logs/action-grouping?action=delete&days=30
func (h *AuditHandler) GetActionGrouping(c *gin.Context) {
	action := c.Query("action")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	items, err := h.repo.GetActionGrouping(c.Request.Context(), action, days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items":  items,
		"action": action,
		"days":   days,
	})
}

// GetResourceTypeStats 获取资源类型统计
// GET /api/v1/audit-logs/resource-stats?days=30
func (h *AuditHandler) GetResourceTypeStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	items, err := h.repo.GetResourceTypeStats(c.Request.Context(), days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items": items,
		"days":  days,
	})
}

// GetTrend 获取操作趋势
// GET /api/v1/audit-logs/trend?days=30
func (h *AuditHandler) GetTrend(c *gin.Context) {
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

// GetHighRiskLogs 获取高危操作日志
// GET /api/v1/audit-logs/high-risk?page=1&page_size=20
func (h *AuditHandler) GetHighRiskLogs(c *gin.Context) {
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
			"action":        log.Action,
			"resource_type": log.ResourceType,
			"resource_name": log.ResourceName,
			"status":        log.Status,
			"ip_address":    log.IPAddress,
			"risk_reason":   repository.GetRiskReason(log.Action, log.ResourceType),
			"created_at":    log.CreatedAt,
			"user":          log.User,
		}
	}

	response.List(c, result, total, page, pageSize)
}

// ExportAuditLogs 导出审计日志为 CSV
// GET /api/v1/audit-logs/export?action=delete&resource_type=user&...
func (h *AuditHandler) ExportAuditLogs(c *gin.Context) {
	opts := &repository.AuditLogListOptions{
		Page:         1,
		PageSize:     10000, // 最多导出 10000 条
		Category:     c.Query("category"),
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		Username:     GetStringFilter(c, "username"),
		Status:       c.Query("status"),
		RiskLevel:    c.Query("risk_level"),
		RequestPath:  GetStringFilter(c, "request_path"),
		SortBy:       "created_at",
		SortOrder:    "desc",
	}

	// 解析时间参数
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

	logs, _, err := h.repo.List(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 生成 CSV
	filename := fmt.Sprintf("audit_logs_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	// UTF-8 BOM for Excel compatibility
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// 表头
	writer.Write([]string{
		"时间", "用户", "操作", "资源类型", "资源名称",
		"请求方法", "请求路径", "状态", "风险等级", "IP 地址", "错误信息",
	})

	// 数据行
	riskLevelLabels := map[string]string{"low": "低", "medium": "中", "high": "高危", "critical": "极高"}
	for _, log := range logs {
		rl := repository.GetRiskLevel(log.Action, log.ResourceType)
		riskLevel := riskLevelLabels[rl]
		if riskLevel == "" {
			riskLevel = "低"
		}

		status := log.Status
		switch status {
		case "success":
			status = "成功"
		case "failed":
			status = "失败"
		}

		writer.Write([]string{
			log.CreatedAt.Format("2006-01-02 15:04:05"),
			log.Username,
			log.Action,
			log.ResourceType,
			log.ResourceName,
			log.RequestMethod,
			log.RequestPath,
			status,
			riskLevel,
			log.IPAddress,
			log.ErrorMessage,
		})
	}
}

// splitAndTrim 按逗号分隔并去除空白
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
