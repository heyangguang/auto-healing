package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuditHandler 审计日志处理器
type AuditHandler struct {
	repo *auditrepo.AuditLogRepository
}

type AuditHandlerDeps struct {
	Repo *auditrepo.AuditLogRepository
}

func NewAuditHandlerWithDeps(deps AuditHandlerDeps) *AuditHandler {
	return &AuditHandler{repo: deps.Repo}
}

func buildAuditListOptions(c *gin.Context, page, pageSize int) (*auditrepo.AuditLogListOptions, error) {
	opts := &auditrepo.AuditLogListOptions{
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
	if ea := c.Query("exclude_action"); ea != "" {
		opts.ExcludeActions = splitAndTrim(ea)
	}
	if ert := c.Query("exclude_resource_type"); ert != "" {
		opts.ExcludeResourceTypes = splitAndTrim(ert)
	}
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, errors.New("user_id 必须是合法 UUID")
		}
		opts.UserID = &uid
	}
	createdAfter, err := parseOptionalAuditTime(c.Query("created_after"), "created_after")
	if err != nil {
		return nil, err
	}
	createdBefore, err := parseOptionalAuditTime(c.Query("created_before"), "created_before")
	if err != nil {
		return nil, err
	}
	opts.CreatedAfter = createdAfter
	opts.CreatedBefore = createdBefore
	return opts, nil
}

func parseOptionalAuditTime(value, key string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%s 必须是合法 RFC3339 时间", key)
	}
	return &parsed, nil
}

func auditRiskFields(action, resourceType string) (string, string) {
	return auditrepo.GetRiskLevel(action, resourceType), auditrepo.GetRiskReason(action, resourceType)
}

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

func sanitizeAuditPayload(payload modeltypes.JSON) modeltypes.JSON {
	if payload == nil {
		return nil
	}
	masked := make(modeltypes.JSON, len(payload))
	for k, v := range payload {
		masked[k] = sanitizeAuditValue(k, v)
	}
	return masked
}

func sanitizeAuditValue(key string, value interface{}) interface{} {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "old_password", "new_password", "secret", "secret_id", "token", "api_key", "private_key", "passphrase", "credential":
		return "***"
	}

	switch typed := value.(type) {
	case modeltypes.JSON:
		return sanitizeAuditPayload(typed)
	case map[string]interface{}:
		masked := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			masked[k] = sanitizeAuditValue(k, v)
		}
		return masked
	case []interface{}:
		masked := make([]interface{}, len(typed))
		for i, v := range typed {
			masked[i] = sanitizeAuditValue("", v)
		}
		return masked
	default:
		return value
	}
}

func writeAuditCSV(c *gin.Context, logs []platformmodel.AuditLog) {
	filename := fmt.Sprintf("audit_logs_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	writer.Write([]string{"时间", "用户", "操作", "资源类型", "资源名称", "请求方法", "请求路径", "状态", "风险等级", "IP 地址", "错误信息"})
	for _, log := range logs {
		writer.Write(auditCSVRow(log))
	}
}

func auditCSVRow(log platformmodel.AuditLog) []string {
	riskLevelLabels := map[string]string{"low": "低", "medium": "中", "high": "高危", "critical": "极高"}
	riskLevel, _ := auditRiskFields(log.Action, log.ResourceType)
	label := riskLevelLabels[riskLevel]
	if label == "" {
		label = "低"
	}
	status := log.Status
	switch status {
	case "success":
		status = "成功"
	case "failed":
		status = "失败"
	}
	return []string{
		log.CreatedAt.Format("2006-01-02 15:04:05"),
		log.Username,
		log.Action,
		log.ResourceType,
		log.ResourceName,
		log.RequestMethod,
		log.RequestPath,
		status,
		label,
		log.IPAddress,
		log.ErrorMessage,
	}
}
