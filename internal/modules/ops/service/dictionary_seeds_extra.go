package service

import "github.com/company/auto-healing/internal/modules/ops/model"

// auditActionSeeds 审计操作类型 Seed（约35种）
func auditActionSeeds() []model.Dictionary {
	return []model.Dictionary{
		d("audit_action", "create", "创建", "Create", "#52c41a", "green", "", "", "", 0),
		d("audit_action", "update", "更新", "Update", "#1890ff", "blue", "", "", "", 1),
		d("audit_action", "delete", "删除", "Delete", "#f5222d", "red", "", "", "", 2),
		d("audit_action", "read", "查看", "Read", "#8c8c8c", "default", "", "", "", 3),
		d("audit_action", "login", "登录", "Login", "#1890ff", "blue", "", "", "", 4),
		d("audit_action", "logout", "登出", "Logout", "#8c8c8c", "default", "", "", "", 5),
		d("audit_action", "execute", "执行", "Execute", "#722ed1", "purple", "", "", "", 6),
		d("audit_action", "sync", "同步", "Sync", "#13c2c2", "cyan", "", "", "", 7),
		d("audit_action", "test", "测试", "Test", "#fa8c16", "orange", "", "", "", 8),
		d("audit_action", "activate", "启用", "Activate", "#52c41a", "green", "", "", "", 9),
		d("audit_action", "deactivate", "停用", "Deactivate", "#8c8c8c", "default", "", "", "", 10),
		d("audit_action", "approve", "审批通过", "Approve", "#52c41a", "green", "", "", "", 11),
		d("audit_action", "reject", "审批拒绝", "Reject", "#f5222d", "red", "", "", "", 12),
		d("audit_action", "export", "导出", "Export", "#1890ff", "blue", "", "", "", 13),
		d("audit_action", "scan", "扫描", "Scan", "#13c2c2", "cyan", "", "", "", 14),
		d("audit_action", "assign_role", "分配角色", "Assign Role", "#722ed1", "purple", "", "", "", 15),
		d("audit_action", "reset_password", "重置密码", "Reset Password", "#fa8c16", "orange", "", "", "", 16),
		d("audit_action", "enable", "启用", "Enable", "#52c41a", "green", "", "", "", 17),
		d("audit_action", "disable", "禁用", "Disable", "#8c8c8c", "default", "", "", "", 18),
		d("audit_action", "trigger", "触发", "Trigger", "#722ed1", "purple", "", "", "", 19),
		d("audit_action", "dismiss", "忽略", "Dismiss", "#8c8c8c", "default", "", "", "", 20),
		d("audit_action", "cancel", "取消", "Cancel", "#8c8c8c", "default", "", "", "", 21),
		d("audit_action", "retry", "重试", "Retry", "#fa8c16", "orange", "", "", "", 22),
		d("audit_action", "dry_run", "模拟执行", "Dry Run", "#13c2c2", "cyan", "", "", "", 23),
		d("audit_action", "reset_status", "重置状态", "Reset Status", "#fa8c16", "orange", "", "", "", 24),
		d("audit_action", "send", "发送", "Send", "#1890ff", "blue", "", "", "", 25),
		d("audit_action", "offline", "下线", "Offline", "#8c8c8c", "default", "", "", "", 26),
		d("audit_action", "patch", "部分更新", "Patch", "#1890ff", "blue", "", "", "", 27),
		d("audit_action", "assign_workspace", "分配工作区", "Assign Workspace", "#722ed1", "purple", "", "", "", 28),
		d("audit_action", "maintenance", "进入维护", "Maintenance", "#fa8c16", "orange", "", "", "", 29),
		d("audit_action", "resume", "恢复服务", "Resume", "#52c41a", "green", "", "", "", 30),
		d("audit_action", "close", "关闭", "Close", "#8c8c8c", "default", "", "", "", 31),
		d("audit_action", "batch_reset_scan", "批量重置", "Batch Reset", "#fa8c16", "orange", "", "", "", 32),
		d("audit_action", "confirm_review", "确认审核", "Confirm Review", "#52c41a", "green", "", "", "", 33),
		d("audit_action", "ready", "设为就绪", "Ready", "#52c41a", "green", "", "", "", 34),
		d("audit_action", "impersonation_enter", "提权进入", "Impersonation Enter", "#722ed1", "purple", "", "", "", 35),
		d("audit_action", "impersonation_exit", "提权退出", "Impersonation Exit", "#8c8c8c", "default", "", "", "", 36),
		d("audit_action", "impersonation_terminate", "提权终止", "Impersonation Terminate", "#f5222d", "red", "", "", "", 37),
		d("audit_action", "batch_create", "批量创建", "Batch Create", "#52c41a", "green", "", "", "", 38),
		d("audit_action", "update_variables", "更新变量", "Update Variables", "#1890ff", "blue", "", "", "", 39),
	}
}

// auditRiskLevelSeeds 审计风险等级 Seed（4级）
func auditRiskLevelSeeds() []model.Dictionary {
	return []model.Dictionary{
		d("audit_risk_level", "critical", "极高", "Critical", "#f5222d", "red", "", "", "", 0),
		d("audit_risk_level", "high", "高危", "High", "#fa8c16", "orange", "", "", "", 1),
		d("audit_risk_level", "medium", "中", "Medium", "#1890ff", "blue", "", "", "", 2),
		d("audit_risk_level", "low", "低", "Low", "#8c8c8c", "default", "", "", "", 3),
	}
}

// auditResourceSeeds 审计资源类型 Seed
//
// dict_key 必须与审计中间件 inferResourceType() 实际产出的 resource_type 一致。
// 审计中间件在推断 resource_type 时会去掉 "platform/" 前缀（audit.go:260），
// 所以平台路由 /api/v1/platform/users 的 resource_type 是 "users" 而非 "platform-users"。
// 平台级 vs 租户级通过 dict_type 区分（audit_resource_platform / audit_resource_tenant），
// 前端分别使用对应的 LABELS Map，不会混淆。
func auditResourceSeeds() []model.Dictionary {
	return auditResourceSeedValues
}

var auditResourceSeedValues = []model.Dictionary{
	// ==================== 租户级资源（实际 key 格式：tenant-X）====================
	// inferResourceType 对 /tenant/X 路径产出 "tenant-X"（tenant 在 nestedPrefixes 中）
	d("audit_resource_tenant", "tenant-users", "用户管理", "User", "", "", "", "", "", 0),
	d("audit_resource_tenant", "tenant-roles", "角色管理", "Role", "", "", "", "", "", 1),
	d("audit_resource_tenant", "tenant-plugins", "插件管理", "Plugin", "", "", "", "", "", 2),
	d("audit_resource_tenant", "tenant-cmdb", "资产管理", "CMDB", "", "", "", "", "", 3),
	d("audit_resource_tenant", "tenant-secrets-sources", "凭据管理", "Credential", "", "", "", "", "", 4),
	d("audit_resource_tenant", "tenant-secrets", "密钥查询", "Secret Query", "", "", "", "", "", 5),
	d("audit_resource_tenant", "tenant-git-repos", "代码仓库", "Code Repository", "", "", "", "", "", 6),
	d("audit_resource_tenant", "tenant-playbooks", "自动化剧本", "Playbook", "", "", "", "", "", 7),
	d("audit_resource_tenant", "tenant-execution-tasks", "执行任务", "Execution Task", "", "", "", "", "", 8),
	d("audit_resource_tenant", "tenant-execution-runs", "执行记录", "Execution Run", "", "", "", "", "", 9),
	d("audit_resource_tenant", "tenant-execution-schedules", "定时任务", "Schedule", "", "", "", "", "", 10),
	d("audit_resource_tenant", "tenant-channels", "通知渠道", "Channel", "", "", "", "", "", 11),
	d("audit_resource_tenant", "tenant-templates", "通知模板", "Template", "", "", "", "", "", 12),
	d("audit_resource_tenant", "tenant-notifications", "通知记录", "Notification", "", "", "", "", "", 13),
	// /tenant/healing/flows|rules|instances|approvals|pending 都产出 "tenant-healing"
	d("audit_resource_tenant", "tenant-healing", "自愈管理", "Healing", "", "", "", "", "", 14),
	d("audit_resource_tenant", "tenant-incidents", "事件工单", "Incident", "", "", "", "", "", 15),
	d("audit_resource_tenant", "tenant-dashboard", "监控面板", "Dashboard", "", "", "", "", "", 16),
	d("audit_resource_tenant", "tenant-site-messages", "站内信", "Site Message", "", "", "", "", "", 17),
	d("audit_resource_tenant", "tenant-impersonation", "临时提权", "Impersonation", "", "", "", "", "", 18),
	d("audit_resource_tenant", "tenant-settings", "租户设置", "Tenant Settings", "", "", "", "", "", 19),
	d("audit_resource_tenant", "tenant-command-blacklist", "命令黑名单", "Command Blacklist", "", "", "", "", "", 20),
	d("audit_resource_tenant", "tenant-blacklist-exemptions", "豁免规则", "Blacklist Exemption", "", "", "", "", "", 21),
	// auth / common 路由（任何角色都可能触发）
	d("audit_resource_tenant", "auth-register", "用户注册", "Register", "", "", "", "", "", 22),
	d("audit_resource_tenant", "auth-logout", "登出", "Logout", "", "", "", "", "", 23),
	d("audit_resource_tenant", "auth-profile", "个人资料", "Profile", "", "", "", "", "", 24),
	d("audit_resource_tenant", "auth-password", "密码修改", "Password", "", "", "", "", "", 25),
	d("audit_resource_tenant", "common-user", "用户个人设置", "User Settings", "", "", "", "", "", 26),
	d("audit_resource_tenant", "common", "通用操作", "Common", "", "", "", "", "", 27),
	d("audit_resource_tenant", "git-repos", "代码仓库", "Git Repository", "", "", "", "", "", 28),
	d("audit_resource_tenant", "healing-flows", "自愈流程", "Healing Flow", "", "", "", "", "", 29),
	d("audit_resource_tenant", "incidents", "事件工单", "Incident", "", "", "", "", "", 30),
	d("audit_resource_tenant", "playbooks", "自动化剧本", "Playbook", "", "", "", "", "", 31),

	// ==================== 平台级资源 ====================
	// 平台管理员操作 /platform/* 路由（已去掉 platform/ 前缀）
	d("audit_resource_platform", "users", "用户管理", "User Management", "", "", "", "", "", 0),
	d("audit_resource_platform", "roles", "角色管理", "Role Management", "", "", "", "", "", 1),
	d("audit_resource_platform", "permissions", "权限管理", "Permission", "", "", "", "", "", 2),
	d("audit_resource_platform", "settings", "平台设置", "Platform Settings", "", "", "", "", "", 3),
	d("audit_resource_platform", "tenants", "租户管理", "Tenant Management", "", "", "", "", "", 4),
	d("audit_resource_platform", "impersonation", "临时提权", "Impersonation", "", "", "", "", "", 5),
	d("audit_resource_platform", "site-messages", "站内信", "Site Message", "", "", "", "", "", 6),
	d("audit_resource_platform", "dictionaries", "字典管理", "Dictionary", "", "", "", "", "", 7),
	d("audit_resource_platform", "tenant", "租户", "Tenant", "", "", "", "", "", 8),
	// auth / common 路由
	d("audit_resource_platform", "auth", "认证", "Auth", "", "", "", "", "", 9),
	d("audit_resource_platform", "auth-logout", "登出", "Logout", "", "", "", "", "", 10),
	d("audit_resource_platform", "auth-profile", "个人资料", "Profile", "", "", "", "", "", 11),
	d("audit_resource_platform", "auth-password", "密码修改", "Password", "", "", "", "", "", 12),
	d("audit_resource_platform", "user", "个人设置", "User Settings", "", "", "", "", "", 13),
	d("audit_resource_platform", "common-user", "用户个人设置", "User Settings", "", "", "", "", "", 14),
	d("audit_resource_platform", "search", "搜索", "Search", "", "", "", "", "", 15),
	d("audit_resource_platform", "workbench", "工作台", "Workbench", "", "", "", "", "", 16),
	d("audit_resource_platform", "command-blacklist", "命令黑名单", "Command Blacklist", "", "", "", "", "", 17),
	d("audit_resource_platform", "blacklist-exemptions", "豁免规则", "Blacklist Exemption", "", "", "", "", "", 18),
	d("audit_resource_platform", "common", "通用操作", "Common", "", "", "", "", "", 19),
	d("audit_resource_platform", "auth-register", "用户注册", "Register", "", "", "", "", "", 20),
	d("audit_resource_platform", "git-repos", "代码仓库", "Git Repository", "", "", "", "", "", 21),
	d("audit_resource_platform", "healing-flows", "自愈流程", "Healing Flow", "", "", "", "", "", 22),
	d("audit_resource_platform", "incidents", "事件工单", "Incident", "", "", "", "", "", 23),
	d("audit_resource_platform", "playbooks", "自动化剧本", "Playbook", "", "", "", "", "", 24),
	// 平台管理员通过 Impersonation 操作租户资源时产出 tenant-X 格式
	d("audit_resource_platform", "tenant-users", "租户用户管理", "Tenant User", "", "", "", "", "", 30),
	d("audit_resource_platform", "tenant-roles", "租户角色管理", "Tenant Role", "", "", "", "", "", 31),
	d("audit_resource_platform", "tenant-plugins", "插件管理", "Plugin", "", "", "", "", "", 32),
	d("audit_resource_platform", "tenant-cmdb", "资产管理", "CMDB", "", "", "", "", "", 33),
	d("audit_resource_platform", "tenant-secrets-sources", "凭据管理", "Credential", "", "", "", "", "", 34),
	d("audit_resource_platform", "tenant-secrets", "密钥查询", "Secret Query", "", "", "", "", "", 35),
	d("audit_resource_platform", "tenant-git-repos", "代码仓库", "Code Repository", "", "", "", "", "", 36),
	d("audit_resource_platform", "tenant-playbooks", "自动化剧本", "Playbook", "", "", "", "", "", 37),
	d("audit_resource_platform", "tenant-execution-tasks", "执行任务", "Execution Task", "", "", "", "", "", 38),
	d("audit_resource_platform", "tenant-execution-runs", "执行记录", "Execution Run", "", "", "", "", "", 39),
	d("audit_resource_platform", "tenant-execution-schedules", "定时任务", "Schedule", "", "", "", "", "", 40),
	d("audit_resource_platform", "tenant-channels", "通知渠道", "Channel", "", "", "", "", "", 41),
	d("audit_resource_platform", "tenant-templates", "通知模板", "Template", "", "", "", "", "", 42),
	d("audit_resource_platform", "tenant-notifications", "通知记录", "Notification", "", "", "", "", "", 43),
	d("audit_resource_platform", "tenant-healing", "自愈管理", "Healing", "", "", "", "", "", 44),
	d("audit_resource_platform", "tenant-incidents", "事件工单", "Incident", "", "", "", "", "", 45),
	d("audit_resource_platform", "tenant-dashboard", "监控面板", "Dashboard", "", "", "", "", "", 46),
	d("audit_resource_platform", "tenant-site-messages", "站内信", "Site Message", "", "", "", "", "", 47),
	d("audit_resource_platform", "tenant-impersonation", "临时提权", "Impersonation", "", "", "", "", "", 48),
	d("audit_resource_platform", "tenant-settings", "租户设置", "Tenant Settings", "", "", "", "", "", 49),
	d("audit_resource_platform", "tenant-command-blacklist", "命令黑名单", "Command Blacklist", "", "", "", "", "", 50),
	d("audit_resource_platform", "tenant-blacklist-exemptions", "豁免规则", "Blacklist Exemption", "", "", "", "", "", 51),
}

// permissionModuleSeeds 权限模块 Seed
func permissionModuleSeeds() []model.Dictionary {
	return []model.Dictionary{
		d("permission_module", "user", "用户管理", "User", "#1890ff", "", "", "", "", 0),
		d("permission_module", "role", "角色权限", "Role", "#722ed1", "", "", "", "", 1),
		d("permission_module", "plugin", "插件管理", "Plugin", "#13c2c2", "", "", "", "", 2),
		d("permission_module", "execution", "执行管理", "Execution", "#fa8c16", "", "", "", "", 3),
		d("permission_module", "notification", "通知管理", "Notification", "#eb2f96", "", "", "", "", 4),
		d("permission_module", "healing", "自愈管理", "Healing", "#52c41a", "", "", "", "", 5),
		d("permission_module", "workflow", "工作流", "Workflow", "#2f54eb", "", "", "", "", 6),
		d("permission_module", "system", "系统管理", "System", "#595959", "", "", "", "", 7),
		d("permission_module", "dashboard", "仪表板", "Dashboard", "#d48806", "", "", "", "", 8),
		d("permission_module", "site-message", "站内信", "Site Message", "#389e0d", "", "", "", "", 9),
		d("permission_module", "platform", "平台管理", "Platform", "#531dab", "", "", "", "", 10),
		d("permission_module", "tenant", "租户管理", "Tenant", "#08979c", "", "", "", "", 11),
	}
}

// httpMethodSeeds HTTP方法 Seed
func httpMethodSeeds() []model.Dictionary {
	return []model.Dictionary{
		d("http_method", "GET", "GET", "GET", "#52c41a", "green", "", "", "", 0),
		d("http_method", "POST", "POST", "POST", "#1890ff", "blue", "", "", "", 1),
		d("http_method", "PUT", "PUT", "PUT", "#fa8c16", "orange", "", "", "", 2),
		d("http_method", "DELETE", "DELETE", "DELETE", "#f5222d", "red", "", "", "", 3),
		d("http_method", "PATCH", "PATCH", "PATCH", "#722ed1", "purple", "", "", "", 4),
	}
}
