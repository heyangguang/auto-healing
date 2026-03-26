package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 资源名称解析 ====================

// tableInfo 表名和 name 列的映射
type tableInfo struct {
	table      string
	column     string
	isPlatform bool   // 平台级表（无 tenant_id 字段）
	primaryKey string // 非空时表示使用该列作为主键（而非默认的 "id"）
}

// pathSegmentToTable 根据 URL 路径段映射到数据库表和名称列
var pathSegmentToTable = map[string]tableInfo{
	// === 租户级资源 ===
	"plugins":             {table: "plugins", column: "name"},
	"users":               {table: "users", column: "username", isPlatform: true}, // users 表无 tenant_id
	"roles":               {table: "roles", column: "name"},
	"channels":            {table: "notification_channels", column: "name"},
	"templates":           {table: "notification_templates", column: "name"},
	"execution-tasks":     {table: "execution_tasks", column: "name"},
	"execution-schedules": {table: "execution_schedules", column: "name"},
	"execution-runs":      {table: "execution_runs", column: "name"},
	"cmdb":                {table: "cmdb_items", column: "name"},
	"secrets-sources":     {table: "secrets_sources", column: "name"},
	"git-repos":           {table: "git_repositories", column: "name"},
	"playbooks":           {table: "playbook_templates", column: "name"},
	"incidents":           {table: "incidents", column: "title"},
	"healing/flows":       {table: "healing_flows", column: "name"},
	"healing/rules":       {table: "healing_rules", column: "name"},
	"healing/instances":   {table: "flow_instances", column: "flow_name"},
	"site-messages":       {table: "site_messages", column: "title"},
	"tenant/users":        {table: "users", column: "username", isPlatform: true}, // users 表无 tenant_id
	"tenant/roles":        {table: "roles", column: "name"},                       // roles 表有 tenant_id
	"auth":                {table: "users", column: "username", isPlatform: true}, // /auth/profile → 用户表
	// === 平台级资源（无 tenant_id）===
	"platform/tenants":  {table: "tenants", column: "name", isPlatform: true},
	"platform/users":    {table: "users", column: "username", isPlatform: true},
	"platform/roles":    {table: "roles", column: "name", isPlatform: true},
	"platform/settings": {table: "platform_settings", column: "label", isPlatform: true, primaryKey: "key"},
	// === Impersonation ===
	"platform/impersonation": {table: "impersonation_requests", column: "tenant_name", isPlatform: true},
	"tenant/impersonation":   {table: "impersonation_requests", column: "tenant_name", isPlatform: true},
}

// resolveResourceName 根据 URL 路径和资源 ID 查询资源名称
// 对平台级资源不使用 tenant_id 过滤
func resolveResourceName(db *gorm.DB, path string, resourceID *uuid.UUID, resourceKey string, bodyJSON model.JSON, tenantID uuid.UUID) (string, error) {
	if info := matchTableInfo(path); info != nil {
		name, ok, err := queryResolvedResourceName(db, info, resourceID, resourceKey, tenantID)
		if err != nil {
			return "", err
		}
		if ok {
			return name, nil
		}
	}
	name, ok, err := extractResourceNameFromBody(db, bodyJSON)
	if err != nil {
		return "", err
	}
	if ok {
		return name, nil
	}
	return "", nil
}

func queryResolvedResourceName(db *gorm.DB, info *tableInfo, resourceID *uuid.UUID, resourceKey string, tenantID uuid.UUID) (string, bool, error) {
	var name string
	query := db.Table(info.table).Select(info.column)
	switch {
	case info.primaryKey != "" && resourceKey != "":
		query = query.Where(fmt.Sprintf("%s = ?", info.primaryKey), resourceKey)
	case resourceID != nil && info.isPlatform:
		query = query.Where("id = ?", *resourceID)
	case resourceID != nil:
		query = query.Where("id = ? AND tenant_id = ?", *resourceID, tenantID)
	default:
		return "", false, nil
	}
	if err := query.Scan(&name).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func extractResourceNameFromBody(db *gorm.DB, bodyJSON model.JSON) (string, bool, error) {
	body, ok := decodeAuditBody(bodyJSON)
	if !ok {
		return "", false, nil
	}
	if name := firstNamedField(body, []string{"name", "title", "username", "flow_name", "hostname"}); name != "" {
		return name, true, nil
	}
	return lookupTenantNameFromBody(db, body)
}

func decodeAuditBody(bodyJSON model.JSON) (map[string]interface{}, bool) {
	if bodyJSON == nil {
		return nil, false
	}
	var body map[string]interface{}
	raw, _ := json.Marshal(bodyJSON)
	if json.Unmarshal(raw, &body) != nil {
		return nil, false
	}
	return body, true
}

func firstNamedField(body map[string]interface{}, fields []string) string {
	for _, field := range fields {
		if value, ok := body[field].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func lookupTenantNameFromBody(db *gorm.DB, body map[string]interface{}) (string, bool, error) {
	tidStr, ok := body["tenant_id"].(string)
	if !ok || tidStr == "" {
		return "", false, nil
	}
	tenantUUID, err := uuid.Parse(tidStr)
	if err != nil {
		return "", false, nil
	}
	var tenantName string
	if err := db.Table("tenants").Select("name").Where("id = ?", tenantUUID).Scan(&tenantName).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if tenantName == "" {
		return "", false, nil
	}
	return tenantName, true, nil
}

// matchTableInfo 从 URL 路径匹配出对应的表信息
func matchTableInfo(path string) *tableInfo {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	if len(parts) == 0 {
		return nil
	}

	// 特殊处理：嵌套成员路由（如 platform/tenants/:id/members/:userId/role）
	// 这类路由操作的是用户的角色，需要映射到 users 表
	if strings.Contains(path, "/members/") {
		usersInfo := tableInfo{table: "users", column: "username", isPlatform: true}
		return &usersInfo
	}

	// 先尝试二段匹配（如 healing/flows、platform/tenants）
	if len(parts) >= 2 {
		key2 := fmt.Sprintf("%s/%s", parts[0], parts[1])
		if info, ok := pathSegmentToTable[key2]; ok {
			return &info
		}
	}

	// 再尝试一段匹配（如 plugins）
	if info, ok := pathSegmentToTable[parts[0]]; ok {
		return &info
	}

	return nil
}

func sanitizeAuditJSON(payload model.JSON) model.JSON {
	if payload == nil {
		return nil
	}
	masked := make(model.JSON, len(payload))
	for k, v := range payload {
		masked[k] = sanitizeAuditValue(k, v)
	}
	return masked
}

func sanitizeAuditMap(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	masked := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		masked[k] = sanitizeAuditValue(k, v)
	}
	return masked
}

func sanitizeAuditValue(key string, value interface{}) interface{} {
	lookupKey := strings.ToLower(strings.TrimSpace(key))
	if sensitiveFields[lookupKey] {
		return "***"
	}

	switch typed := value.(type) {
	case model.JSON:
		return sanitizeAuditJSON(typed)
	case map[string]interface{}:
		return sanitizeAuditMap(typed)
	case []interface{}:
		masked := make([]interface{}, len(typed))
		for i, item := range typed {
			masked[i] = sanitizeAuditValue("", item)
		}
		return masked
	case string:
		var parsed interface{}
		if json.Unmarshal([]byte(typed), &parsed) == nil {
			return sanitizeAuditValue(key, parsed)
		}
		return typed
	default:
		return value
	}
}
