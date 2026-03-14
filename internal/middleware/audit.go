package middleware

import (
	"bytes"
	"context"
	"database/sql"
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
	platformRepo := repository.NewPlatformAuditLogRepository()

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
		resourceKey := extractResourceKey(path) // 字符串主键（如 platform/settings 的 key）
		// 提取租户 ID 用于审计查询的租户隔离
		tenantID := repository.TenantIDFromContext(c.Request.Context())
		// 提取用户身份 — 决定审计日志写入平台表还是租户表
		isPlatformAdmin := IsPlatformAdmin(c)
		// 对 /auth/profile 路由，使用当前用户 ID 作为资源 ID
		if strings.HasSuffix(path, "/auth/profile") {
			if userIDStr := GetUserID(c); userIDStr != "" {
				if uid, err := uuid.Parse(userIDStr); err == nil {
					resourceID = &uid
				}
			}
		}
		var oldState map[string]interface{}
		if method == "PUT" || method == "PATCH" || method == "DELETE" {
			if isPlatformRoute(path) {
				// 平台路由操作的是平台级资源，不需要 tenant_id 过滤
				oldState = captureOldState(path, resourceID, resourceKey, uuid.Nil)
			} else {
				// 非平台路由（包括平台管理员操作租户资源），使用实际的 tenant_id
				oldState = captureOldState(path, resourceID, resourceKey, tenantID)
			}
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
		// isPlatformAdmin 已在上方提取，goroutine 中安全使用
		// Impersonation 标记（goroutine 安全）
		isImpersonating := IsImpersonating(c)

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
			var resourceNameTenantID uuid.UUID
			if !isPlatformRoute(path) {
				resourceNameTenantID = tenantID
			}
			resourceName := resolveResourceName(path, resourceID, resourceKey, bodyJSON, resourceNameTenantID)

			// 计算变更记录（PUT/PATCH/DELETE）
			var changes model.JSON
			if status == "success" {
				changes = computeChanges(method, action, oldState, bodyJSON)
			}

			// 审计分类：Impersonation 操作归入 "operation"（用户名已标识 [Impersonation]）
			category := "operation"

			// 根据用户身份判断写入逻辑
			// - 平台管理员（非 Impersonation） → platform_audit_logs
			// - 平台管理员（Impersonation）     → platform_audit_logs + audit_logs（双写）
			// - 租户用户                        → audit_logs
			// 构建带租户 ID 的独立 context（避免请求取消影响审计写入，同时保留正确的租户作用域）
			auditBaseCtx := repository.WithTenantID(context.Background(), tenantID)
			ctx, cancel := context.WithTimeout(auditBaseCtx, 5*time.Second)
			defer cancel()

			if isPlatformAdmin {
				// 平台管理员 — 写入 platform_audit_logs
				platformLog := &model.PlatformAuditLog{
					UserID:         userID,
					Username:       username,
					IPAddress:      ipAddress,
					UserAgent:      userAgent,
					Category:       category,
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
				if err := platformRepo.Create(ctx, platformLog); err != nil {
					zap.L().Error("平台审计日志写入失败", zap.Error(err))
				}

				// Impersonation 操作 — 同时写入 audit_logs（租户侧也能看到）
				if isImpersonating {
					auditLog := &model.AuditLog{
						UserID:         userID,
						Username:       username + " [Impersonation]",
						IPAddress:      ipAddress,
						UserAgent:      userAgent,
						Category:       category,
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
					if err := repo.Create(ctx, auditLog); err != nil {
						zap.L().Error("Impersonation 租户审计日志写入失败", zap.Error(err))
					}
				}
			} else {
				// 租户用户 — 写入 audit_logs
				auditLog := &model.AuditLog{
					UserID:         userID,
					Username:       username,
					IPAddress:      ipAddress,
					UserAgent:      userAgent,
					Category:       category,
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
				if err := repo.Create(ctx, auditLog); err != nil {
					zap.L().Error("审计日志写入失败", zap.Error(err))
				}
			}
		}()
	}
}

// ==================== 操作和资源类型推断 ====================

// isPlatformRoute 判断是否是平台级路由
func isPlatformRoute(path string) bool {
	return strings.HasPrefix(path, "/api/v1/platform/")
}

// inferActionAndResource 从 HTTP 方法和路径推断操作类型和资源类型
func inferActionAndResource(method, path string) (action, resourceType string) {
	// 去掉 /api/v1/ 前缀
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	// 去掉 platform/ 前缀（平台级路由），使得资源类型推断正确
	trimmed = strings.TrimPrefix(trimmed, "platform/")
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
			case "dismiss":
				action = "dismiss"
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
			case "roles", "role":
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
		"healing":  true, // healing/flows, healing/rules, healing/instances, healing/approvals
		"auth":     true, // auth/login, auth/logout, auth/profile
		"tenant":   true, // tenant/users, tenant/roles
		"platform": true, // platform/tenants, platform/users, platform/roles
		"common":   true, // common/user/*, common/...（通用接口）
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
// 对于嵌套路由如 /tenants/:id/members/:userId/role，需要根据末端操作判断提取哪个 UUID
func extractResourceID(path string) *uuid.UUID {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	// 收集所有 UUID
	var uuids []uuid.UUID
	for _, part := range parts {
		if uid, err := uuid.Parse(part); err == nil {
			uuids = append(uuids, uid)
		}
	}

	if len(uuids) == 0 {
		return nil
	}

	// 如果路径包含 members/和 /role（如 tenants/:id/members/:userId/role），
	// 取第二个 UUID（目标用户），因为我们要查询的是用户的状态
	if len(uuids) >= 2 && strings.Contains(path, "/members/") {
		return &uuids[1]
	}

	return &uuids[0]
}

// extractResourceKey 从 URL 路径中提取字符串资源键
// 用于 platform/settings 等使用非 UUID 主键的资源
// 路径如 /api/v1/platform/settings/site_message.retention_days → "site_message.retention_days"
func extractResourceKey(path string) string {
	info := matchTableInfo(path)
	if info == nil || info.primaryKey == "" {
		return ""
	}
	// 提取路径中最后一个非空片段作为 key
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			// 确保不是路由前缀段
			if _, err := uuid.Parse(parts[i]); err != nil {
				// 跳过已知的路由段名
				if parts[i] != "platform" && parts[i] != "settings" {
					return parts[i]
				}
			}
		}
	}
	return ""
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
// 支持 UUID 主键和字符串主键两种资源类型
// 对平台级资源不使用 tenant_id 过滤
func captureOldState(path string, resourceID *uuid.UUID, resourceKey string, tenantID uuid.UUID) map[string]interface{} {
	info := matchTableInfo(path)
	if info == nil {
		return nil
	}

	var rows *sql.Rows
	var err error

	if info.primaryKey != "" && resourceKey != "" {
		// 字符串主键（如 platform_settings 的 key 列）
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", info.table, info.primaryKey)
		rows, err = database.DB.Raw(query, resourceKey).Rows()
	} else if resourceID != nil && info.isPlatform {
		// 平台级表没有 tenant_id 字段
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 LIMIT 1", info.table)
		rows, err = database.DB.Raw(query, *resourceID).Rows()
	} else if resourceID != nil {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND tenant_id = $2 LIMIT 1", info.table)
		rows, err = database.DB.Raw(query, *resourceID, tenantID).Rows()
	} else {
		return nil
	}
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
	changes := make(map[string]interface{})

	if method == "DELETE" {
		if oldState == nil {
			return nil
		}
		// 删除操作：记录被删除资源的关键信息，统一 old/new 格式（new 为 null）
		infoFields := []string{"name", "username", "title", "hostname", "flow_name", "description", "status"}
		for _, key := range infoFields {
			if v, ok := oldState[key]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				changes[key] = map[string]interface{}{
					"old": formatForDisplay(v),
					"new": nil,
				}
			}
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

	// 关联操作（分配角色/权限/工作区）— 请求体包含的是关系数据而非直接列值
	// 无法获取旧的关联关系，所以 old 为 null，new 记录新分配的值
	if relationshipActions[action] {
		for k, v := range reqBody {
			if !sensitiveFields[k] {
				changes[k] = map[string]interface{}{
					"old": nil,
					"new": v,
				}
			}
		}
		if len(changes) > 0 {
			return model.JSON(changes)
		}
		return nil
	}

	// 标准行级 diff — 对比 oldState 和 requestBody
	if oldState == nil {
		// 无旧状态但有请求体 — 记录为 old=null, new=实际值
		for k, v := range reqBody {
			if !sensitiveFields[k] {
				changes[k] = map[string]interface{}{
					"old": nil,
					"new": formatForDisplay(v),
				}
			}
		}
		if len(changes) > 0 {
			return model.JSON(changes)
		}
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
//
// 排除原则：
//   - 安全事件（登录/登出/密码/密钥）→ 必须审计
//   - 资源增删改、状态变更、审批/触发 → 必须审计
//   - 个人偏好/阅读行为/界面配置 → 跳过（低价值噪音）
//   - 测试/校验/预览/模拟操作 → 跳过（不影响系统状态）
//     注意：密钥连接测试（/secrets-sources/*/test*）因安全敏感性保留审计
func shouldSkipAudit(path string) bool {
	// === 1. 前缀匹配 — 整类路由跳过 ===
	skipPrefixes := []string{
		"/api/v1/auth/login",                    // 登录由 auth handler 自行记录
		"/api/v1/auth/logout",                   // 登出由 auth handler 自行记录
		"/api/v1/auth/refresh",                  // Token 刷新无需审计
		"/api/v1/common/user/recents",           // 最近访问记录（导航自动触发）
		"/api/v1/common/user/favorites",         // 收藏操作（低价值）
		"/api/v1/common/user/preferences",       // 用户偏好设置
		"/api/v1/tenant/site-messages/read",     // 标记消息已读（用户阅读行为）
		"/api/v1/tenant/site-messages/read-all", // 全部已读（用户阅读行为）
		"/api/v1/tenant/dashboard/config",       // Dashboard 布局配置（个人偏好，自动保存触发）
		"/api/v1/common/workbench/favorites",    // 工作台收藏（仅 GET，自动跳过，防御性保留）
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// === 2. 后缀匹配 — 测试/校验/预览/模拟类操作（不影响系统状态） ===
	// 注意：密钥连接测试（secrets-sources）因安全敏感性，不在此排除
	skipSuffixes := []string{
		"/test-connection",       // CMDB 连接测试
		"/batch-test-connection", // CMDB 批量连接测试
		"/test-query",            // 密钥查询测试（非连接测试）— 无状态变更
		"/validate",              // Git 仓库校验
		"/preview",               // 通知模板预览
		"/dry-run",               // 自愈流程模拟执行
		"/dry-run-stream",        // 自愈流程模拟执行（SSE）
	}
	for _, suffix := range skipSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	// 通知渠道测试（/channels/:id/test）— 不影响系统状态
	// 但排除密钥连接测试（/secrets-sources/:id/test）— 安全敏感
	if strings.HasSuffix(path, "/test") && !strings.Contains(path, "/secrets-sources/") {
		return true
	}

	return false
}

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
func resolveResourceName(path string, resourceID *uuid.UUID, resourceKey string, bodyJSON model.JSON, tenantID uuid.UUID) string {
	if info := matchTableInfo(path); info != nil {
		var name string
		var err error

		if info.primaryKey != "" && resourceKey != "" {
			// 字符串主键（如 platform_settings 的 key 列查 label）
			err = database.DB.Table(info.table).
				Select(info.column).
				Where(fmt.Sprintf("%s = ?", info.primaryKey), resourceKey).
				Scan(&name).Error
		} else if resourceID != nil && info.isPlatform {
			err = database.DB.Table(info.table).
				Select(info.column).
				Where("id = ?", *resourceID).
				Scan(&name).Error
		} else if resourceID != nil {
			err = database.DB.Table(info.table).
				Select(info.column).
				Where("id = ? AND tenant_id = ?", *resourceID, tenantID).
				Scan(&name).Error
		}
		if err == nil && name != "" {
			return name
		}
	}

	// 后备：从请求体的 name/title/username 字段提取
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
			// 特殊处理：请求体含 tenant_id 时查租户名称（如 impersonation 创建请求）
			if tid, ok := body["tenant_id"]; ok {
				if tidStr, ok := tid.(string); ok && tidStr != "" {
					if tenantUUID, err := uuid.Parse(tidStr); err == nil {
						var tenantName string
						if err := database.DB.Table("tenants").Select("name").
							Where("id = ?", tenantUUID).Scan(&tenantName).Error; err == nil && tenantName != "" {
							return tenantName
						}
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
