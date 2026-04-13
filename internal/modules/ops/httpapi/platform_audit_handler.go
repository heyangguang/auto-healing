package httpapi

import (
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/pkg/response"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PlatformAuditHandler 平台审计日志处理器
type PlatformAuditHandler struct {
	repo *auditrepo.PlatformAuditLogRepository
}

type PlatformAuditHandlerDeps struct {
	Repo *auditrepo.PlatformAuditLogRepository
}

func NewPlatformAuditHandlerWithDeps(deps PlatformAuditHandlerDeps) *PlatformAuditHandler {
	return &PlatformAuditHandler{
		repo: deps.Repo,
	}
}

// ListPlatformAuditLogs 获取平台审计日志列表
// GET /api/v1/platform/audit-logs?page=1&page_size=20&category=login&action=create&...
func (h *PlatformAuditHandler) ListPlatformAuditLogs(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts, err := buildPlatformAuditListOptions(c, page, pageSize)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	logs, total, err := h.repo.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取平台审计日志列表失败", err)
		return
	}
	response.List(c, formatPlatformAuditLogs(logs), total, page, pageSize)
}

func buildPlatformAuditListOptions(c *gin.Context, page, pageSize int) (*auditrepo.PlatformAuditListOptions, error) {
	userID, err := parsePlatformAuditUserID(c.Query("user_id"))
	if err != nil {
		return nil, err
	}
	createdAfter, err := parseOptionalRFC3339Time(c.Query("created_after"), "created_after")
	if err != nil {
		return nil, err
	}
	createdBefore, err := parseOptionalRFC3339Time(c.Query("created_before"), "created_before")
	if err != nil {
		return nil, err
	}
	return &auditrepo.PlatformAuditListOptions{
		Page:          page,
		PageSize:      pageSize,
		Search:        GetStringFilter(c, "search"),
		Category:      c.Query("category"),
		Action:        c.Query("action"),
		ResourceType:  c.Query("resource_type"),
		Username:      GetStringFilter(c, "username"),
		Status:        c.Query("status"),
		RequestPath:   GetStringFilter(c, "request_path"),
		SortBy:        c.Query("sort_by"),
		SortOrder:     c.Query("sort_order"),
		UserID:        userID,
		CreatedAfter:  createdAfter,
		CreatedBefore: createdBefore,
	}, nil
}

func parsePlatformAuditUserID(value string) (*uuid.UUID, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("user_id 必须是合法 UUID")
	}
	return &parsed, nil
}

func parseOptionalRFC3339Time(value, key string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%s 必须是合法 RFC3339 时间", key)
	}
	return &parsed, nil
}

func formatPlatformAuditLogs(logs []platformmodel.PlatformAuditLog) []platformAuditLogResponse {
	result := make([]platformAuditLogResponse, len(logs))
	for i, log := range logs {
		result[i] = newPlatformAuditLogResponse(log)
	}
	return result
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
		respondInternalError(c, "AUDIT", "获取平台审计日志详情失败", err)
		return
	}
	if log == nil {
		response.NotFound(c, "平台审计日志不存在")
		return
	}

	response.Success(c, newPlatformAuditLogResponse(*log))
}

// GetPlatformAuditStats 获取平台审计统计
// GET /api/v1/platform/audit-logs/stats
func (h *PlatformAuditHandler) GetPlatformAuditStats(c *gin.Context) {
	stats, err := h.repo.GetStats(c.Request.Context(), c.Query("category"))
	if err != nil {
		respondInternalError(c, "AUDIT", "获取平台审计统计失败", err)
		return
	}
	response.Success(c, stats)
}

// GetPlatformAuditTrend 获取平台审计趋势
// GET /api/v1/platform/audit-logs/trend?days=30
func (h *PlatformAuditHandler) GetPlatformAuditTrend(c *gin.Context) {
	days := parsePositiveIntQuery(c, "days", 30, 365)

	items, err := h.repo.GetTrend(c.Request.Context(), days, c.Query("category"))
	if err != nil {
		respondInternalError(c, "AUDIT", "获取平台审计趋势失败", err)
		return
	}

	response.Success(c, items)
}

// GetPlatformUserRanking 获取平台用户操作排行
// GET /api/v1/platform/audit-logs/user-ranking?limit=10&days=7
func (h *PlatformAuditHandler) GetPlatformUserRanking(c *gin.Context) {
	limit := parsePositiveIntQuery(c, "limit", 10, 100)
	days := parsePositiveIntQuery(c, "days", 7, 365)

	rankings, err := h.repo.GetUserRanking(c.Request.Context(), limit, days)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取平台审计用户排行失败", err)
		return
	}

	response.Success(c, rankings)
}

// GetPlatformHighRiskLogs 获取平台高危操作日志
// GET /api/v1/platform/audit-logs/high-risk?page=1&page_size=20
func (h *PlatformAuditHandler) GetPlatformHighRiskLogs(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)

	logs, total, err := h.repo.GetHighRiskLogs(c.Request.Context(), page, pageSize)
	if err != nil {
		respondInternalError(c, "AUDIT", "获取平台高危审计日志失败", err)
		return
	}

	result := make([]platformHighRiskAuditLogResponse, len(logs))
	for i, log := range logs {
		result[i] = newPlatformHighRiskAuditLogResponse(log)
	}

	response.List(c, result, total, page, pageSize)
}
