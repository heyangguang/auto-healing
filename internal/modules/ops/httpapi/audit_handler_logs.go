package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/response"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
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
	if auditrepo.IsAuthCategoryFilter(opts.Category) {
		logs, total, listErr := h.platformRepo.ListTenantVisibleAuthLogs(c.Request.Context(), opts)
		if listErr != nil {
			respondInternalError(c, "AUDIT", "获取认证审计日志列表失败", listErr)
			return
		}
		result := make([]auditLogResponse, len(logs))
		for i, log := range logs {
			result[i] = newAuditLogResponseFromPlatformLog(log)
		}
		response.List(c, result, total, page, pageSize)
		return
	}
	logs, total, err := h.repo.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取审计日志列表失败", err)
		return
	}

	result := make([]auditLogResponse, len(logs))
	for i, log := range logs {
		result[i] = newAuditLogResponse(log)
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
	platformLog, err := h.platformRepo.GetTenantVisibleAuthLogByID(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取认证审计日志详情失败", err)
		return
	}
	if platformLog != nil {
		response.Success(c, newAuditLogResponseFromPlatformLog(*platformLog))
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
	response.Success(c, newAuditLogResponse(*log))
}

// GetHighRiskLogs 获取高危操作日志
func (h *AuditHandler) GetHighRiskLogs(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	logs, total, err := h.repo.GetHighRiskLogs(c.Request.Context(), page, pageSize)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取高危审计日志失败", err)
		return
	}

	result := make([]highRiskAuditLogResponse, len(logs))
	for i, log := range logs {
		result[i] = newHighRiskAuditLogResponse(log)
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
