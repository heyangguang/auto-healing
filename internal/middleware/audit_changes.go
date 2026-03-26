package middleware

import (
	"encoding/json"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 错误信息提取 ====================

// extractErrorMessage 从响应体中提取错误信息
func extractErrorMessage(responseBody []byte) string {
	if len(responseBody) == 0 || len(responseBody) > 4096 {
		return ""
	}
	var resp struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(responseBody, &resp) == nil && resp.Message != "" {
		if len(resp.Message) > 500 {
			return resp.Message[:500] + "..."
		}
		return resp.Message
	}
	return ""
}

// ==================== 变更记录 (Changes) ====================

// sensitiveFields 不应出现在变更记录中的字段
var sensitiveFields = map[string]bool{
	"id": true, "created_at": true, "updated_at": true, "deleted_at": true,
	"password_hash": true, "password": true, "old_password": true, "new_password": true,
	"access_key_hash": true, "secret_key_hash": true, "secret_key": true,
	"private_key": true, "passphrase": true, "credential": true,
	"token": true, "api_key": true, "secret_id": true,
}

// captureOldState 在修改前获取资源的当前状态
// 支持 UUID 主键和字符串主键两种资源类型
// 对平台级资源不使用 tenant_id 过滤
func captureOldState(db *gorm.DB, path string, resourceID *uuid.UUID, resourceKey string, tenantID uuid.UUID) map[string]interface{} {
	info := matchTableInfo(path)
	if info == nil {
		return nil
	}
	query, ok := buildOldStateQuery(db, info, resourceID, resourceKey, tenantID)
	if !ok {
		return nil
	}
	var result map[string]interface{}
	if err := query.Take(&result).Error; err != nil {
		return nil
	}
	return normalizeOldStateResult(result)
}

func buildOldStateQuery(db *gorm.DB, info *tableInfo, resourceID *uuid.UUID, resourceKey string, tenantID uuid.UUID) (*gorm.DB, bool) {
	query := db.Table(info.table)
	if info.primaryKey != "" && resourceKey != "" {
		return query.Where(fmt.Sprintf("%s = ?", info.primaryKey), resourceKey), true
	}
	if resourceID == nil {
		return nil, false
	}
	if info.isPlatform {
		return query.Where("id = ?", *resourceID), true
	}
	return query.Where("id = ? AND tenant_id = ?", *resourceID, tenantID), true
}

func normalizeOldStateResult(result map[string]interface{}) map[string]interface{} {
	for key, value := range result {
		if b, ok := value.([]byte); ok {
			result[key] = string(b)
		}
	}
	return result
}

// relationshipActions 关联操作 — 这些操作修改的是多对多关系，而非单行的列值
// 无法通过 oldState 和 requestBody 的列级 diff 来计算变更
// 对这些操作，直接将请求体记录为 changes 的 assigned 字段
var relationshipActions = map[string]bool{
	"assign_role":       true,
	"assign_permission": true,
	"assign_workspace":  true,
}

// computeChanges 比较旧状态和请求体，生成变更记录
func computeChanges(method string, action string, oldState map[string]interface{}, requestBody model.JSON) model.JSON {
	if method == "DELETE" {
		return buildDeleteChanges(oldState)
	}
	reqBody := decodeAuditRequestBody(requestBody)
	if reqBody == nil {
		return nil
	}
	if relationshipActions[action] {
		return buildAssignedChanges(reqBody)
	}
	if oldState == nil {
		return buildCreateChanges(reqBody)
	}
	return buildDiffChanges(oldState, reqBody)
}

func buildDeleteChanges(oldState map[string]interface{}) model.JSON {
	if oldState == nil {
		return nil
	}
	changes := make(map[string]interface{})
	for _, key := range []string{"name", "username", "title", "hostname", "flow_name", "description", "status"} {
		if v, ok := oldState[key]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
			changes[key] = map[string]interface{}{"old": formatForDisplay(v), "new": nil}
		}
	}
	return auditChangesOrNil(changes)
}

func decodeAuditRequestBody(requestBody model.JSON) map[string]interface{} {
	if requestBody == nil {
		return nil
	}
	var reqBody map[string]interface{}
	raw, _ := json.Marshal(requestBody)
	if json.Unmarshal(raw, &reqBody) != nil {
		return nil
	}
	return reqBody
}

func buildAssignedChanges(reqBody map[string]interface{}) model.JSON {
	changes := make(map[string]interface{})
	for key, value := range reqBody {
		if sensitiveFields[key] {
			continue
		}
		changes[key] = map[string]interface{}{"old": nil, "new": value}
	}
	return auditChangesOrNil(changes)
}

func buildCreateChanges(reqBody map[string]interface{}) model.JSON {
	changes := make(map[string]interface{})
	for key, value := range reqBody {
		if sensitiveFields[key] {
			continue
		}
		changes[key] = map[string]interface{}{"old": nil, "new": formatForDisplay(value)}
	}
	return auditChangesOrNil(changes)
}

func buildDiffChanges(oldState, reqBody map[string]interface{}) model.JSON {
	changes := make(map[string]interface{})
	for key, newVal := range reqBody {
		if sensitiveFields[key] {
			continue
		}
		oldVal, exists := oldState[key]
		if !exists || formatForCompare(oldVal) == formatForCompare(newVal) {
			continue
		}
		changes[key] = map[string]interface{}{
			"old": formatForDisplay(oldVal),
			"new": formatForDisplay(newVal),
		}
	}
	return auditChangesOrNil(changes)
}

func auditChangesOrNil(changes map[string]interface{}) model.JSON {
	if len(changes) == 0 {
		return nil
	}
	return model.JSON(changes)
}

// formatForCompare 将值序列化为可比较的字符串
// 对于字符串类型的值，先尝试解析为 JSON 再序列化，以规范化 JSONB 字段的字段顺序，
// 避免数据库返回的 JSON 字符串与请求体 JSON 对象因字段顺序不同被误判为变更。
func formatForCompare(v interface{}) string {
	if v == nil {
		return "null"
	}
	// 如果是字符串，尝试解析为 JSON 结构再序列化（规范化字段顺序）
	if s, ok := v.(string); ok {
		var parsed interface{}
		if json.Unmarshal([]byte(s), &parsed) == nil {
			if b, err := json.Marshal(parsed); err == nil {
				return string(b)
			}
		}
		// 不是 JSON 字符串，直接用带引号的形式
		if b, err := json.Marshal(s); err == nil {
			return string(b)
		}
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// formatForDisplay 格式化值用于显示
// 将过长的值截断，避免 changes 字段过大
func formatForDisplay(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	// 字符串类型：截断过长内容
	if s, ok := v.(string); ok {
		if len(s) > 200 {
			return s[:200] + "..."
		}
		return s
	}
	return v
}
