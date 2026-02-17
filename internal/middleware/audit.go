package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// responseBodyWriter 包装 gin.ResponseWriter 以捕获响应体
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// AuditMiddleware 审计中间件 — 自动记录写操作的审计日志
func AuditMiddleware() gin.HandlerFunc {
	repo := repository.NewAuditLogRepository()

	return func(c *gin.Context) {
		// 只记录写操作
		method := c.Request.Method
		if method == "GET" || method == "OPTIONS" || method == "HEAD" {
			c.Next()
			return
		}

		// 跳过不需要审计的路由
		path := c.Request.URL.Path
		if shouldSkipAudit(path) {
			c.Next()
			return
		}

		// 读取请求体（供后续记录）
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 记录开始时间
		startTime := time.Now()

		// ===== 在 handler 执行前：提取资源 ID 并捕获旧状态 =====
		resourceID := extractResourceID(path)
		var oldState map[string]interface{}
		if method == "PUT" || method == "PATCH" || method == "DELETE" {
			oldState = captureOldState(path, resourceID)
		}

		// 包装 ResponseWriter 以捕获响应体（用于提取错误信息）
		bodyWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = bodyWriter

		// ===== 执行后续处理 =====
		c.Next()

		// ===== 在 goroutine 之前提取所有 gin.Context 的值 =====
		// 注意：gin.Context 会在请求结束后被回收，goroutine 不能访问 c
		userIDStr := GetUserID(c)
		username := GetUsername(c)
		statusCode := c.Writer.Status()
		ipAddress := NormalizeIP(c.ClientIP())
		userAgent := c.Request.UserAgent()
		responseBody := bodyWriter.body.Bytes()

		// ===== 异步记录审计日志 =====
		go func() {
			defer func() {
				if r := recover(); r != nil {
					zap.L().Error("审计日志记录失败 (panic)", zap.Any("error", r))
				}
			}()

			var userID *uuid.UUID
			if userIDStr != "" {
				if uid, err := uuid.Parse(userIDStr); err == nil {
					userID = &uid
				}
			}

			// 推断操作类型和资源类型
			action, resourceType := inferActionAndResource(method, path)

			// 判断状态 + 提取错误信息
			status := "success"
			errorMsg := ""
			if statusCode >= 400 {
				status = "failed"
				errorMsg = extractErrorMessage(responseBody)
			}

			// 限制请求体大小（避免存储过大数据）
			var bodyJSON model.JSON
			if len(requestBody) > 0 && len(requestBody) < 10240 {
				_ = json.Unmarshal(requestBody, &bodyJSON)
			}

			// 解析资源名称
			resourceName := resolveResourceName(path, resourceID, bodyJSON)

			// 计算变更记录（PUT/PATCH/DELETE）
			var changes model.JSON
			if status == "success" {
				changes = computeChanges(method, oldState, bodyJSON)
			}

			auditLog := &model.AuditLog{
				UserID:         userID,
				Username:       username,
				IPAddress:      ipAddress,
				UserAgent:      userAgent,
				Action:         action,
				ResourceType:   resourceType,
				ResourceID:     resourceID,
				ResourceName:   resourceName,
				RequestMethod:  method,
				RequestPath:    path,
				RequestBody:    bodyJSON,
				ResponseStatus: &statusCode,
				Changes:        changes,
				Status:         status,
				ErrorMessage:   errorMsg,
				CreatedAt:      startTime,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := repo.Create(ctx, auditLog); err != nil {
				zap.L().Error("审计日志写入失败", zap.Error(err))
			}
		}()
	}
}

// ==================== 操作和资源类型推断 ====================

// inferActionAndResource 从 HTTP 方法和路径推断操作类型和资源类型
func inferActionAndResource(method, path string) (action, resourceType string) {
	// 去掉 /api/v1/ 前缀
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	// 获取资源类型 — 处理嵌套路径（如 healing/rules → healing-rules）
	resourceType = inferResourceType(parts)

	// 基于 HTTP 方法推断操作
	switch method {
	case "POST":
		action = "create"
		// 检查特殊后缀操作
		if len(parts) >= 3 {
			lastPart := parts[len(parts)-1]
			switch lastPart {
			case "activate":
				action = "activate"
			case "deactivate":
				action = "deactivate"
			case "test":
				action = "test"
			case "sync":
				action = "sync"
			case "execute":
				action = "execute"
			case "enable":
				action = "enable"
			case "disable":
				action = "disable"
			case "approve":
				action = "approve"
			case "reject":
				action = "reject"
			case "cancel":
				action = "cancel"
			case "retry":
				action = "retry"
			case "reset-password":
				action = "reset_password"
			case "trigger":
				action = "trigger"
			case "confirm-review":
				action = "confirm_review"
			case "dry-run", "dry-run-stream":
				action = "dry_run"
			case "reset-scan", "batch-reset-scan":
				action = "reset_scan"
			case "reset-status":
				action = "reset_status"
			case "send":
				action = "send"
			case "preview":
				action = "preview"
			case "ready":
				action = "ready"
			case "offline":
				action = "offline"
			case "scan":
				action = "scan"
			case "maintenance":
				action = "maintenance"
			case "resume":
				action = "resume"
			}
		}

		// 批量操作
		if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-1], "batch") {
			action = "batch_" + action
		}
	case "PUT":
		action = "update"
		// 特殊：角色分配
		if len(parts) >= 3 {
			lastPart := parts[len(parts)-1]
			switch lastPart {
			case "roles":
				action = "assign_role"
			case "permissions":
				action = "assign_permission"
			case "variables":
				action = "update_variables"
			case "workspaces":
				action = "assign_workspace"
			}
		}
	case "DELETE":
		action = "delete"
	case "PATCH":
		action = "patch"
	}

	return action, resourceType
}

// inferResourceType 从路径段推断可读的资源类型
// 例: ["healing","rules","uuid"] → "healing-rules"
//
//	["plugins","uuid","activate"] → "plugins"
func inferResourceType(parts []string) string {
	if len(parts) == 0 {
		return "unknown"
	}

	// 已知的嵌套资源前缀 — 第一段是父级，第二段是子资源
	nestedPrefixes := map[string]bool{
		"healing": true, // healing/flows, healing/rules, healing/instances, healing/approvals
		"auth":    true, // auth/login, auth/logout
	}

	first := parts[0]

	// 如果是嵌套资源且有第二段（非 UUID）
	if nestedPrefixes[first] && len(parts) >= 2 {
		second := parts[1]
		// 确保第二段不是 UUID
		if _, err := uuid.Parse(second); err != nil {
			return first + "-" + second // healing-rules, healing-flows
		}
	}

	return first
}

// extractResourceID 从 URL 路径中提取资源 ID
func extractResourceID(path string) *uuid.UUID {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	// 遍历路径片段，查找 UUID
	for _, part := range parts {
		if uid, err := uuid.Parse(part); err == nil {
			return &uid
		}
	}
	return nil
}

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
	"token": true, "api_key": true,
}

// captureOldState 在修改前获取资源的当前状态
func captureOldState(path string, resourceID *uuid.UUID) map[string]interface{} {
	if resourceID == nil {
		return nil
	}
	info := matchTableInfo(path)
	if info == nil {
		return nil
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 LIMIT 1", info.table)
	rows, err := database.DB.Raw(query, *resourceID).Rows()
	if err != nil {
		return nil
	}
	defer rows.Close()

	if !rows.Next() {
		return nil
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil
	}

	// 创建 interface{} 切片来接收值
	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	if err := rows.Scan(values...); err != nil {
		return nil
	}

	result := make(map[string]interface{})
	for i, col := range columns {
		val := *(values[i].(*interface{}))
		// 将 []byte 转为 string
		if b, ok := val.([]byte); ok {
			result[col] = string(b)
		} else {
			result[col] = val
		}
	}

	return result
}

// computeChanges 比较旧状态和请求体，生成变更记录
func computeChanges(method string, oldState map[string]interface{}, requestBody model.JSON) model.JSON {
	if oldState == nil {
		return nil
	}

	changes := make(map[string]interface{})

	if method == "DELETE" {
		// 删除操作：记录被删除资源的关键信息
		deleteInfo := map[string]interface{}{}
		infoFields := []string{"name", "username", "title", "hostname", "flow_name", "description", "status"}
		for _, key := range infoFields {
			if v, ok := oldState[key]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				deleteInfo[key] = v
			}
		}
		if len(deleteInfo) > 0 {
			changes["deleted"] = deleteInfo
		}
		if len(changes) == 0 {
			return nil
		}
		return model.JSON(changes)
	}

	// PUT/PATCH：解析请求体
	if requestBody == nil {
		return nil
	}
	var reqBody map[string]interface{}
	raw, _ := json.Marshal(requestBody)
	if json.Unmarshal(raw, &reqBody) != nil {
		return nil
	}

	// 只对比请求体中出现且在旧状态中也存在的字段
	for key, newVal := range reqBody {
		if sensitiveFields[key] {
			continue
		}
		oldVal, exists := oldState[key]
		if !exists {
			continue
		}

		// 用 JSON 序列化后的字符串比较，消除类型差异（int64 vs float64 等）
		oldStr := formatForCompare(oldVal)
		newStr := formatForCompare(newVal)
		if oldStr != newStr {
			changes[key] = map[string]interface{}{
				"old": formatForDisplay(oldVal),
				"new": formatForDisplay(newVal),
			}
		}
	}

	if len(changes) == 0 {
		return nil
	}
	return model.JSON(changes)
}

// formatForCompare 将值序列化为可比较的字符串
func formatForCompare(v interface{}) string {
	if v == nil {
		return "null"
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

// ==================== IP 归一化 ====================

// NormalizeIP 将 IPv6 地址归一化为 IPv4（如果可能）
// ::1 → 127.0.0.1
// ::ffff:192.168.1.1 → 192.168.1.1
// 纯 IPv6 保持不变
func NormalizeIP(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if parsed.IsLoopback() && parsed.To4() == nil {
		return "127.0.0.1"
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return ip
}

// ==================== 路由跳过 ====================

// shouldSkipAudit 判断路由是否应跳过审计记录
func shouldSkipAudit(path string) bool {
	skipPrefixes := []string{
		"/api/v1/auth/login",       // 登录由 auth handler 自行记录
		"/api/v1/auth/refresh",     // Token 刷新无需审计
		"/api/v1/user/recents",     // 最近访问记录（导航自动触发）
		"/api/v1/user/favorites",   // 收藏操作（低价值）
		"/api/v1/user/preferences", // 用户偏好设置
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// ==================== 资源名称解析 ====================

// tableInfo 表名和 name 列的映射
type tableInfo struct {
	table  string
	column string
}

// pathSegmentToTable 根据 URL 路径段映射到数据库表和名称列
var pathSegmentToTable = map[string]tableInfo{
	"plugins":             {"plugins", "name"},
	"users":               {"users", "username"},
	"roles":               {"roles", "name"},
	"channels":            {"notification_channels", "name"},
	"templates":           {"notification_templates", "name"},
	"execution-tasks":     {"execution_tasks", "name"},
	"execution-schedules": {"execution_schedules", "name"},
	"execution-runs":      {"execution_runs", "name"},
	"cmdb":                {"cmdb_items", "name"},
	"secrets-sources":     {"secrets_sources", "name"},
	"git-repos":           {"git_repositories", "name"},
	"playbooks":           {"playbook_templates", "name"},
	"incidents":           {"incidents", "title"},
	"healing/flows":       {"healing_flows", "name"},
	"healing/rules":       {"healing_rules", "name"},
	"healing/instances":   {"flow_instances", "flow_name"},
}

// resolveResourceName 根据 URL 路径和资源 ID 查询资源名称
func resolveResourceName(path string, resourceID *uuid.UUID, bodyJSON model.JSON) string {
	// 1. 如果有资源 ID，尝试从数据库查找
	if resourceID != nil {
		if info := matchTableInfo(path); info != nil {
			var name string
			err := database.DB.Table(info.table).
				Select(info.column).
				Where("id = ?", *resourceID).
				Scan(&name).Error
			if err == nil && name != "" {
				return name
			}
		}
	}

	// 2. 后备：从请求体的 name/title/username 字段提取
	if bodyJSON != nil {
		var body map[string]interface{}
		raw, _ := json.Marshal(bodyJSON)
		if json.Unmarshal(raw, &body) == nil {
			for _, field := range []string{"name", "title", "username", "flow_name", "hostname"} {
				if v, ok := body[field]; ok {
					if s, ok := v.(string); ok && s != "" {
						return s
					}
				}
			}
		}
	}

	return ""
}

// matchTableInfo 从 URL 路径匹配出对应的表信息
func matchTableInfo(path string) *tableInfo {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	if len(parts) == 0 {
		return nil
	}

	// 先尝试二段匹配（如 healing/flows）
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
