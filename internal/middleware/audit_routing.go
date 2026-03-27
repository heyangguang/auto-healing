package middleware

import (
	"net"
	"strings"

	"github.com/google/uuid"
)

// ==================== 操作和资源类型推断 ====================

// isPlatformRoute 判断是否是平台级路由
func isPlatformRoute(path string) bool {
	return strings.HasPrefix(path, "/api/v1/platform/")
}

// inferActionAndResource 从 HTTP 方法和路径推断操作类型和资源类型
func inferActionAndResource(method, path string) (action, resourceType string) {
	parts := auditRouteParts(path)
	return inferAuditAction(method, parts), inferResourceType(parts)
}

func auditRouteParts(path string) []string {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	trimmed = strings.TrimPrefix(trimmed, "platform/")
	return strings.Split(trimmed, "/")
}

func inferAuditAction(method string, parts []string) string {
	switch method {
	case "POST":
		return inferPostAuditAction(parts)
	case "PUT":
		return inferPutAuditAction(parts)
	case "DELETE":
		return "delete"
	case "PATCH":
		return "patch"
	default:
		return ""
	}
}

func inferPostAuditAction(parts []string) string {
	action := "create"
	if len(parts) >= 3 {
		if mapped, ok := postAuditActionOverrides()[parts[len(parts)-1]]; ok {
			action = mapped
		}
	}
	if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-1], "batch") {
		return "batch_" + action
	}
	return action
}

func postAuditActionOverrides() map[string]string {
	return map[string]string{
		"activate":         "activate",
		"deactivate":       "deactivate",
		"test":             "test",
		"sync":             "sync",
		"execute":          "execute",
		"enable":           "enable",
		"disable":          "disable",
		"approve":          "approve",
		"reject":           "reject",
		"cancel":           "cancel",
		"retry":            "retry",
		"reset-password":   "reset_password",
		"trigger":          "trigger",
		"dismiss":          "dismiss",
		"confirm-review":   "confirm_review",
		"dry-run":          "dry_run",
		"dry-run-stream":   "dry_run",
		"reset-scan":       "reset_scan",
		"batch-reset-scan": "reset_scan",
		"reset-status":     "reset_status",
		"send":             "send",
		"preview":          "preview",
		"ready":            "ready",
		"offline":          "offline",
		"scan":             "scan",
		"maintenance":      "maintenance",
		"resume":           "resume",
	}
}

func inferPutAuditAction(parts []string) string {
	if len(parts) < 3 {
		return "update"
	}
	switch parts[len(parts)-1] {
	case "roles", "role":
		return "assign_role"
	case "permissions":
		return "assign_permission"
	case "variables":
		return "update_variables"
	case "workspaces":
		return "assign_workspace"
	default:
		return "update"
	}
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
