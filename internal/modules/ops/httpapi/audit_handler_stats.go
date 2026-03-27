package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetAuditStats 获取审计统计概览
func (h *AuditHandler) GetAuditStats(c *gin.Context) {
	stats, err := h.repo.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计统计失败", err)
		return
	}
	response.Success(c, stats)
}

// GetUserRanking 获取用户操作排行榜
func (h *AuditHandler) GetUserRanking(c *gin.Context) {
	limit := parsePositiveIntQuery(c, "limit", 10, 100)
	days := parsePositiveIntQuery(c, "days", 7, 365)
	rankings, err := h.repo.GetUserRanking(c.Request.Context(), limit, days)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计用户排行失败", err)
		return
	}
	response.Success(c, gin.H{"rankings": rankings, "limit": limit, "days": days})
}

// GetActionGrouping 按操作类型分组统计
func (h *AuditHandler) GetActionGrouping(c *gin.Context) {
	action := c.Query("action")
	days := parsePositiveIntQuery(c, "days", 30, 365)
	items, err := h.repo.GetActionGrouping(c.Request.Context(), action, days)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计操作分组失败", err)
		return
	}
	response.Success(c, gin.H{"items": items, "action": action, "days": days})
}

// GetResourceTypeStats 获取资源类型统计
func (h *AuditHandler) GetResourceTypeStats(c *gin.Context) {
	days := parsePositiveIntQuery(c, "days", 30, 365)
	items, err := h.repo.GetResourceTypeStats(c.Request.Context(), days)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计资源统计失败", err)
		return
	}
	response.Success(c, gin.H{"items": items, "days": days})
}

// GetTrend 获取操作趋势
func (h *AuditHandler) GetTrend(c *gin.Context) {
	days := parsePositiveIntQuery(c, "days", 30, 365)
	items, err := h.repo.GetTrend(c.Request.Context(), days)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计趋势失败", err)
		return
	}
	response.Success(c, gin.H{"items": items, "days": days})
}
