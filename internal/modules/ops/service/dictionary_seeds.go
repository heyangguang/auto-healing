package service

import "github.com/company/auto-healing/internal/modules/ops/model"

// d 辅助函数：创建字典条目
func d(dictType, key, label, labelEn, color, tagColor, badge, icon, bg string, sort int) model.Dictionary {
	return model.Dictionary{
		DictType: dictType, DictKey: key, Label: label, LabelEn: labelEn,
		Color: color, TagColor: tagColor, Badge: badge, Icon: icon, Bg: bg,
		SortOrder: sort, IsSystem: true, IsActive: true,
	}
}

func dInactive(dictType, key, label, labelEn, color, tagColor, badge, icon, bg string, sort int) model.Dictionary {
	item := d(dictType, key, label, labelEn, color, tagColor, badge, icon, bg, sort)
	item.IsActive = false
	return item
}

// AllDictionarySeeds 全量字典 Seed 数据（53 组）
var AllDictionarySeeds = []model.Dictionary{
	// ==================== 1. incident_severity ====================
	d("incident_severity", "critical", "致命", "Critical", "#cf1322", "red", "error", "", "", 0),
	d("incident_severity", "high", "严重", "High", "#d46b08", "orange", "warning", "", "", 1),
	d("incident_severity", "medium", "中等", "Medium", "#d4b106", "gold", "warning", "", "", 2),
	d("incident_severity", "low", "低", "Low", "#1890ff", "blue", "processing", "", "", 3),
	d("incident_severity", "info", "信息", "Info", "#8c8c8c", "default", "default", "", "", 4),

	// ==================== 2. incident_category ====================
	d("incident_category", "network", "网络", "Network", "#1890ff", "blue", "", "", "", 0),
	d("incident_category", "application", "应用", "Application", "#52c41a", "green", "", "", "", 1),
	d("incident_category", "database", "数据库", "Database", "#722ed1", "purple", "", "", "", 2),
	d("incident_category", "security", "安全", "Security", "#f5222d", "red", "", "", "", 3),
	d("incident_category", "hardware", "硬件", "Hardware", "#fa8c16", "orange", "", "", "", 4),
	d("incident_category", "storage", "存储", "Storage", "#13c2c2", "cyan", "", "", "", 5),

	// ==================== 3. incident_status ====================
	d("incident_status", "open", "待处理", "Open", "#1890ff", "blue", "processing", "", "", 0),
	d("incident_status", "in_progress", "处理中", "In Progress", "#fa8c16", "orange", "processing", "", "", 1),
	d("incident_status", "resolved", "已解决", "Resolved", "#52c41a", "green", "success", "", "", 2),
	d("incident_status", "closed", "已关闭", "Closed", "#8c8c8c", "default", "default", "", "", 3),

	// ==================== 4. healing_status ====================
	d("healing_status", "pending", "待处理", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("healing_status", "matched", "已匹配", "Matched", "#1890ff", "blue", "processing", "", "", 1),
	d("healing_status", "healing", "自愈中", "Healing", "#1890ff", "blue", "processing", "", "", 2),
	d("healing_status", "processing", "执行中", "Processing", "#fa8c16", "orange", "processing", "", "", 3),
	d("healing_status", "healed", "已自愈", "Healed", "#52c41a", "green", "success", "", "", 4),
	d("healing_status", "failed", "自愈失败", "Failed", "#f5222d", "red", "error", "", "", 5),
	d("healing_status", "skipped", "跳过", "Skipped", "#8c8c8c", "default", "default", "", "", 6),
	d("healing_status", "dismissed", "已忽略", "Dismissed", "#8c8c8c", "default", "default", "", "", 7),

	// ==================== 5. instance_status ====================
	d("instance_status", "pending", "等待执行", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("instance_status", "running", "执行中", "Running", "#1890ff", "blue", "processing", "", "", 1),
	d("instance_status", "waiting_approval", "等待审批", "Waiting Approval", "#fa8c16", "orange", "warning", "", "", 2),
	d("instance_status", "completed", "已完成", "Completed", "#52c41a", "green", "success", "", "", 3),
	d("instance_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 4),
	d("instance_status", "cancelled", "已取消", "Cancelled", "#8c8c8c", "default", "default", "", "", 5),

	// ==================== 6. node_status ====================
	d("node_status", "pending", "等待执行", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("node_status", "running", "执行中", "Running", "#1890ff", "blue", "processing", "", "", 1),
	d("node_status", "success", "成功", "Success", "#52c41a", "green", "success", "", "", 2),
	d("node_status", "partial", "部分成功", "Partial", "#fa8c16", "orange", "warning", "", "", 3),
	d("node_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 4),
	d("node_status", "skipped", "已跳过", "Skipped", "#8c8c8c", "default", "default", "", "", 5),
	d("node_status", "waiting_approval", "等待审批", "Waiting Approval", "#fa8c16", "orange", "warning", "", "", 6),

	// ==================== 7. approval_status ====================
	d("approval_status", "pending", "待审批", "Pending", "#fa8c16", "orange", "warning", "", "", 0),
	d("approval_status", "approved", "已通过", "Approved", "#52c41a", "green", "success", "", "", 1),
	d("approval_status", "rejected", "已拒绝", "Rejected", "#f5222d", "red", "error", "", "", 2),
	d("approval_status", "expired", "已过期", "Expired", "#8c8c8c", "default", "default", "", "", 3),
	d("approval_status", "cancelled", "已取消", "Cancelled", "#8c8c8c", "default", "default", "", "", 4),

	// ==================== 8. node_type ====================
	d("node_type", "start", "开始", "Start", "#52c41a", "green", "", "PlayCircleOutlined", "", 0),
	d("node_type", "end", "结束", "End", "#f5222d", "red", "", "StopOutlined", "", 1),
	d("node_type", "host_extractor", "主机提取", "Host Extractor", "#1890ff", "blue", "", "CloudServerOutlined", "", 2),
	d("node_type", "cmdb_validator", "CMDB验证", "CMDB Validator", "#13c2c2", "cyan", "", "SafetyCertificateOutlined", "", 3),
	d("node_type", "approval", "审批", "Approval", "#fa8c16", "orange", "", "AuditOutlined", "", 4),
	d("node_type", "execution", "执行", "Execution", "#722ed1", "purple", "", "ThunderboltOutlined", "", 5),
	d("node_type", "notification", "通知", "Notification", "#eb2f96", "magenta", "", "BellOutlined", "", 6),
	d("node_type", "condition", "条件判断", "Condition", "#2f54eb", "geekblue", "", "BranchesOutlined", "", 7),
	d("node_type", "set_variable", "设置变量", "Set Variable", "#389e0d", "green", "", "EditOutlined", "", 8),
	d("node_type", "compute", "计算", "Compute", "#d48806", "gold", "", "CalculatorOutlined", "", 9),

	// ==================== 9. rule_trigger_mode ====================
	d("rule_trigger_mode", "auto", "自动触发", "Auto", "#52c41a", "green", "", "", "", 0),
	d("rule_trigger_mode", "manual", "手动触发", "Manual", "#1890ff", "blue", "", "", "", 1),

	// ==================== 10. rule_match_mode ====================
	d("rule_match_mode", "all", "全部匹配", "Match All", "#1890ff", "blue", "", "", "", 0),
	d("rule_match_mode", "any", "任一匹配", "Match Any", "#fa8c16", "orange", "", "", "", 1),

	// ==================== 11. log_level ====================
	d("log_level", "debug", "调试", "Debug", "#8c8c8c", "default", "", "", "", 0),
	d("log_level", "info", "信息", "Info", "#1890ff", "blue", "", "", "", 1),
	d("log_level", "warn", "警告", "Warning", "#fa8c16", "orange", "", "", "", 2),
	d("log_level", "error", "错误", "Error", "#f5222d", "red", "", "", "", 3),

	// ==================== 12. plugin_type ====================
	d("plugin_type", "itsm", "ITSM工单", "ITSM", "#1890ff", "blue", "", "", "", 0),
	d("plugin_type", "cmdb", "CMDB资产", "CMDB", "#52c41a", "green", "", "", "", 1),

	// ==================== 13. plugin_status ====================
	d("plugin_status", "active", "运行中", "Active", "#52c41a", "green", "success", "", "", 0),
	d("plugin_status", "inactive", "未启用", "Inactive", "#d9d9d9", "default", "default", "", "", 1),
	d("plugin_status", "error", "异常", "Error", "#f5222d", "red", "error", "", "", 2),

	// ==================== 14. plugin_sync_type ====================
	d("plugin_sync_type", "scheduled", "定时同步", "Scheduled", "#1890ff", "blue", "", "", "", 0),
	d("plugin_sync_type", "manual", "手动同步", "Manual", "#fa8c16", "orange", "", "", "", 1),
	d("plugin_sync_type", "webhook", "Webhook", "Webhook", "#722ed1", "purple", "", "", "", 2),

	// ==================== 15. plugin_sync_status ====================
	d("plugin_sync_status", "running", "同步中", "Running", "#1890ff", "blue", "processing", "", "", 0),
	d("plugin_sync_status", "success", "成功", "Success", "#52c41a", "green", "success", "", "", 1),
	d("plugin_sync_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 2),

	// ==================== 16. cmdb_type ====================
	d("cmdb_type", "server", "服务器", "Server", "#1890ff", "blue", "", "DesktopOutlined", "", 0),
	d("cmdb_type", "application", "应用", "Application", "#52c41a", "green", "", "AppstoreOutlined", "", 1),
	d("cmdb_type", "network", "网络设备", "Network", "#fa8c16", "orange", "", "GlobalOutlined", "", 2),
	d("cmdb_type", "database", "数据库", "Database", "#722ed1", "purple", "", "DatabaseOutlined", "", 3),

	// ==================== 17. cmdb_status ====================
	d("cmdb_status", "active", "在线", "Active", "#52c41a", "green", "success", "", "", 0),
	d("cmdb_status", "offline", "离线", "Offline", "#d9d9d9", "default", "default", "", "", 1),
	d("cmdb_status", "maintenance", "维护中", "Maintenance", "#fa8c16", "orange", "warning", "", "", 2),

	// ==================== 18. cmdb_environment ====================
	d("cmdb_environment", "production", "生产", "Production", "#f5222d", "red", "", "", "", 0),
	d("cmdb_environment", "staging", "预发布", "Staging", "#fa8c16", "orange", "", "", "", 1),
	d("cmdb_environment", "test", "测试", "Test", "#1890ff", "blue", "", "", "", 2),
	d("cmdb_environment", "dev", "开发", "Development", "#52c41a", "green", "", "", "", 3),

	// ==================== 19. notification_channel_type ====================
	d("notification_channel_type", "webhook", "Webhook", "Webhook", "#722ed1", "purple", "", "ApiOutlined", "#f9f0ff", 0),
	d("notification_channel_type", "email", "邮件", "Email", "#1890ff", "blue", "", "MailOutlined", "#e6f7ff", 1),
	d("notification_channel_type", "dingtalk", "钉钉", "DingTalk", "#1677ff", "blue", "", "DingdingOutlined", "#e6f7ff", 2),
	d("notification_channel_type", "wecom", "企业微信", "WeCom", "#07c160", "green", "", "WechatWorkOutlined", "#f6ffed", 3),
	d("notification_channel_type", "slack", "Slack", "Slack", "#4a154b", "purple", "", "SlackOutlined", "#fff0f6", 4),
	d("notification_channel_type", "teams", "Teams", "Microsoft Teams", "#6264a7", "geekblue", "", "WindowsOutlined", "#f0f5ff", 5),

	// ==================== 20. notification_event_type ====================
	d("notification_event_type", "execution_started", "执行开始", "Execution Started", "#13c2c2", "cyan", "", "", "", 0),
	d("notification_event_type", "execution_result", "执行结果", "Execution Result", "#1890ff", "blue", "", "", "", 1),
	d("notification_event_type", "flow_result", "流程结果", "Flow Result", "#722ed1", "purple", "", "", "", 2),
	d("notification_event_type", "approval_required", "等待审批", "Approval Required", "#fa8c16", "orange", "", "", "", 3),
	d("notification_event_type", "manual_notification", "手动通知", "Manual Notification", "#8c8c8c", "default", "", "", "", 4),
	dInactive("notification_event_type", "custom", "自定义", "Custom", "#8c8c8c", "default", "", "", "", 89),
	dInactive("notification_event_type", "incident_created", "事件创建", "Incident Created", "#f5222d", "red", "", "", "", 90),
	dInactive("notification_event_type", "incident_resolved", "事件解决", "Incident Resolved", "#52c41a", "green", "", "", "", 91),

	// ==================== 21. notification_format ====================
	d("notification_format", "text", "纯文本", "Text", "#8c8c8c", "default", "", "", "", 0),
	d("notification_format", "markdown", "Markdown", "Markdown", "#1890ff", "blue", "", "", "", 1),
	d("notification_format", "html", "HTML", "HTML", "#52c41a", "green", "", "", "", 2),

	// ==================== 22. notification_log_status ====================
	d("notification_log_status", "pending", "待发送", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("notification_log_status", "sent", "已发送", "Sent", "#1890ff", "blue", "processing", "", "", 1),
	d("notification_log_status", "delivered", "已送达", "Delivered", "#52c41a", "green", "success", "", "", 2),
	d("notification_log_status", "failed", "发送失败", "Failed", "#f5222d", "red", "error", "", "", 3),
	d("notification_log_status", "bounced", "被退回", "Bounced", "#fa8c16", "orange", "warning", "", "", 4),

	// ==================== 23. audit_log_category ====================
	d("audit_log_category", "login", "登录", "Login", "#1890ff", "blue", "", "", "", 0),
	d("audit_log_category", "operation", "操作", "Operation", "#fa8c16", "orange", "", "", "", 1),

	// ==================== 24. audit_log_status ====================
	d("audit_log_status", "success", "成功", "Success", "#52c41a", "green", "success", "", "", 0),
	d("audit_log_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 1),

	// ==================== 25. execution_stage ====================
	d("execution_stage", "prepare", "准备阶段", "Prepare", "#1890ff", "blue", "", "", "", 0),
	d("execution_stage", "execute", "执行阶段", "Execute", "#fa8c16", "orange", "", "", "", 1),
	d("execution_stage", "cleanup", "清理阶段", "Cleanup", "#52c41a", "green", "", "", "", 2),

	// ==================== 26. git_auth_type ====================
	d("git_auth_type", "none", "无认证", "None", "#8c8c8c", "default", "", "", "", 0),
	d("git_auth_type", "token", "Token", "Token", "#1890ff", "blue", "", "", "", 1),
	d("git_auth_type", "password", "用户名密码", "Password", "#fa8c16", "orange", "", "", "", 2),
	d("git_auth_type", "ssh_key", "SSH密钥", "SSH Key", "#52c41a", "green", "", "", "", 3),

	// ==================== 27. git_repo_status ====================
	d("git_repo_status", "pending", "待同步", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("git_repo_status", "syncing", "同步中", "Syncing", "#1890ff", "blue", "processing", "", "", 1),
	d("git_repo_status", "ready", "就绪", "Ready", "#52c41a", "green", "success", "", "", 2),
	d("git_repo_status", "error", "错误", "Error", "#f5222d", "red", "error", "", "", 3),
	dInactive("git_repo_status", "synced", "已同步", "Synced", "#52c41a", "green", "success", "", "", 4),

	// ==================== 28-30. git_sync_* ====================
	d("git_sync_trigger_type", "manual", "手动", "Manual", "#fa8c16", "orange", "", "", "", 0),
	d("git_sync_trigger_type", "branch_change", "分支变更", "Branch Change", "#1890ff", "blue", "", "", "", 1),
	dInactive("git_sync_trigger_type", "scheduled", "定时", "Scheduled", "#8c8c8c", "default", "", "", "", 2),
	d("git_sync_action", "clone", "克隆", "Clone", "#1890ff", "blue", "", "", "", 0),
	d("git_sync_action", "pull", "拉取", "Pull", "#52c41a", "green", "", "", "", 1),
	d("git_sync_status", "success", "成功", "Success", "#52c41a", "green", "success", "", "", 0),
	d("git_sync_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 1),

	// ==================== 31. playbook_config_mode ====================
	d("playbook_config_mode", "auto", "自动", "Auto", "#52c41a", "green", "", "", "", 0),
	d("playbook_config_mode", "enhanced", "增强", "Enhanced", "#1890ff", "blue", "", "", "", 1),

	// ==================== 32. playbook_status ====================
	d("playbook_status", "pending", "待扫描", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("playbook_status", "ready", "就绪", "Ready", "#52c41a", "green", "success", "", "", 1),
	d("playbook_status", "error", "错误", "Error", "#f5222d", "red", "error", "", "", 2),
	d("playbook_status", "invalid", "无效", "Invalid", "#8c8c8c", "default", "default", "", "", 3),

	// ==================== 33. playbook_scan_trigger_type ====================
	d("playbook_scan_trigger_type", "manual", "手动扫描", "Manual", "#fa8c16", "orange", "", "", "", 0),
	d("playbook_scan_trigger_type", "repo_sync", "仓库同步", "Repo Sync", "#1890ff", "blue", "", "", "", 1),

	// ==================== 34. executor_type ====================
	d("executor_type", "local", "本地执行", "Local", "#52c41a", "green", "", "", "", 0),
	d("executor_type", "docker", "Docker容器", "Docker", "#1890ff", "blue", "", "", "", 1),
	d("executor_type", "ssh", "SSH远程", "SSH", "#fa8c16", "orange", "", "", "", 2),

	// ==================== 35. run_status ====================
	d("run_status", "pending", "等待执行", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("run_status", "running", "执行中", "Running", "#1890ff", "blue", "processing", "", "", 1),
	d("run_status", "success", "成功", "Success", "#52c41a", "green", "success", "", "", 2),
	d("run_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 3),
	d("run_status", "partial", "部分成功", "Partial", "#fa8c16", "orange", "warning", "", "", 4),
	d("run_status", "cancelled", "已取消", "Cancelled", "#8c8c8c", "default", "default", "", "", 5),
	d("run_status", "timeout", "超时", "Timeout", "#fa541c", "volcano", "error", "", "", 6),

	// ==================== 36-37. schedule ====================
	d("schedule_type", "cron", "循环调度", "Cron", "#1890ff", "blue", "", "", "", 0),
	d("schedule_type", "once", "单次调度", "Once", "#fa8c16", "orange", "", "", "", 1),
	d("schedule_status", "running", "运行中", "Running", "#52c41a", "green", "success", "", "", 0),
	d("schedule_status", "pending", "待执行", "Pending", "#1890ff", "blue", "processing", "", "", 1),
	d("schedule_status", "completed", "已完成", "Completed", "#8c8c8c", "default", "default", "", "", 2),
	d("schedule_status", "disabled", "已禁用", "Disabled", "#d9d9d9", "default", "default", "", "", 3),
	d("schedule_status", "auto_paused", "自动暂停", "Auto Paused", "#fa8c16", "orange", "warning", "", "", 4),

	// ==================== 38-42. workflow ====================
	d("workflow_status", "draft", "草稿", "Draft", "#d9d9d9", "default", "default", "", "", 0),
	d("workflow_status", "active", "激活", "Active", "#52c41a", "green", "success", "", "", 1),
	d("workflow_status", "inactive", "未激活", "Inactive", "#8c8c8c", "default", "default", "", "", 2),
	d("workflow_status", "archived", "已归档", "Archived", "#8c8c8c", "default", "default", "", "", 3),
	d("workflow_trigger_type", "incident", "事件触发", "Incident", "#f5222d", "red", "", "", "", 0),
	d("workflow_trigger_type", "scheduled", "定时触发", "Scheduled", "#1890ff", "blue", "", "", "", 1),
	d("workflow_trigger_type", "manual", "手动触发", "Manual", "#fa8c16", "orange", "", "", "", 2),
	d("workflow_trigger_type", "webhook", "Webhook触发", "Webhook", "#722ed1", "purple", "", "", "", 3),
	d("workflow_node_type", "start", "开始", "Start", "#52c41a", "green", "", "", "", 0),
	d("workflow_node_type", "end", "结束", "End", "#f5222d", "red", "", "", "", 1),
	d("workflow_node_type", "condition", "条件", "Condition", "#2f54eb", "geekblue", "", "", "", 2),
	d("workflow_node_type", "approval", "审批", "Approval", "#fa8c16", "orange", "", "", "", 3),
	d("workflow_node_type", "notification", "通知", "Notification", "#eb2f96", "magenta", "", "", "", 4),
	d("workflow_node_type", "execution", "执行", "Execution", "#722ed1", "purple", "", "", "", 5),
	d("workflow_node_type", "delay", "延迟", "Delay", "#13c2c2", "cyan", "", "", "", 6),
	d("workflow_node_type", "parallel", "并行", "Parallel", "#1890ff", "blue", "", "", "", 7),
	d("workflow_instance_status", "pending", "等待", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("workflow_instance_status", "running", "运行中", "Running", "#1890ff", "blue", "processing", "", "", 1),
	d("workflow_instance_status", "paused", "已暂停", "Paused", "#fa8c16", "orange", "warning", "", "", 2),
	d("workflow_instance_status", "completed", "已完成", "Completed", "#52c41a", "green", "success", "", "", 3),
	d("workflow_instance_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 4),
	d("workflow_instance_status", "cancelled", "已取消", "Cancelled", "#8c8c8c", "default", "default", "", "", 5),
	d("node_execution_status", "pending", "等待", "Pending", "#d9d9d9", "default", "default", "", "", 0),
	d("node_execution_status", "running", "运行中", "Running", "#1890ff", "blue", "processing", "", "", 1),
	d("node_execution_status", "success", "成功", "Success", "#52c41a", "green", "success", "", "", 2),
	d("node_execution_status", "failed", "失败", "Failed", "#f5222d", "red", "error", "", "", 3),
	d("node_execution_status", "skipped", "跳过", "Skipped", "#8c8c8c", "default", "default", "", "", 4),

	// ==================== 43-44. secrets ====================
	d("secrets_source_type", "vault", "Vault", "Vault", "#722ed1", "purple", "", "", "", 0),
	d("secrets_source_type", "file", "文件", "File", "#1890ff", "blue", "", "", "", 1),
	d("secrets_source_type", "webhook", "Webhook", "Webhook", "#fa8c16", "orange", "", "", "", 2),
	d("secrets_auth_type", "ssh_key", "SSH密钥", "SSH Key", "#52c41a", "green", "", "", "", 0),
	d("secrets_auth_type", "password", "密码", "Password", "#fa8c16", "orange", "", "", "", 1),

	// ==================== 45-47. user/tenant/role ====================
	d("user_status", "active", "正常", "Active", "#52c41a", "green", "success", "", "", 0),
	d("user_status", "inactive", "未启用", "Inactive", "#d9d9d9", "default", "default", "", "", 1),
	d("user_status", "locked", "已锁定", "Locked", "#f5222d", "red", "error", "", "", 2),
	dInactive("user_status", "disabled", "已禁用", "Disabled", "#d9d9d9", "default", "default", "", "", 3),
	d("tenant_status", "active", "正常", "Active", "#52c41a", "green", "success", "", "", 0),
	d("tenant_status", "disabled", "已禁用", "Disabled", "#d9d9d9", "default", "default", "", "", 1),
	d("role_scope", "platform", "平台级", "Platform", "#722ed1", "purple", "", "", "", 0),
	d("role_scope", "tenant", "租户级", "Tenant", "#1890ff", "blue", "", "", "", 1),

	// ==================== 48. site_message_category ====================
	d("site_message_category", "system_update", "系统更新", "System Update", "#1890ff", "blue", "", "", "", 0),
	d("site_message_category", "fault_alert", "故障告警", "Fault Alert", "#f5222d", "red", "", "", "", 1),
	d("site_message_category", "service_notice", "服务通知", "Service Notice", "#fa8c16", "orange", "", "", "", 2),
	d("site_message_category", "product_news", "产品动态", "Product News", "#52c41a", "green", "", "", "", 3),
	d("site_message_category", "activity", "活动公告", "Activity", "#722ed1", "purple", "", "", "", 4),
	d("site_message_category", "security", "安全公告", "Security", "#f5222d", "red", "", "", "", 5),
	d("site_message_category", "announcement", "系统公告", "Announcement", "#1890ff", "blue", "", "", "", 6),
}

// 注意：audit_action (49), audit_resource_tenant (50), audit_resource_platform (51),
// permission_module (52), http_method (53) 将在下方的 init() 中追加
func init() {
	AllDictionarySeeds = append(AllDictionarySeeds, auditActionSeeds()...)
	AllDictionarySeeds = append(AllDictionarySeeds, auditRiskLevelSeeds()...)
	AllDictionarySeeds = append(AllDictionarySeeds, auditResourceSeeds()...)
	AllDictionarySeeds = append(AllDictionarySeeds, permissionModuleSeeds()...)
	AllDictionarySeeds = append(AllDictionarySeeds, httpMethodSeeds()...)
}
