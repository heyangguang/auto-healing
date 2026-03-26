package database

var PlatformSystemRoles = []RoleSeed{
	{
		Name:        "platform_admin",
		DisplayName: "平台超级管理员",
		Description: "拥有平台所有权限，可管理租户、用户、角色、设置等一切平台资源",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:tenants:manage", "platform:tenants:list",
			"platform:settings:manage",
			"platform:users:list", "platform:users:create", "platform:users:update", "platform:users:delete", "platform:users:reset_password",
			"platform:roles:list", "platform:roles:manage",
			"platform:permissions:list",
			"platform:audit:list", "platform:audit:export",
			"platform:messages:send",
			"site-message:settings:view", "site-message:settings:manage",
		},
	},
	{
		Name:        "platform_ops",
		DisplayName: "平台运维",
		Description: "管理平台设置和平台角色，不能查看租户和用户数据",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:settings:manage",
			"platform:roles:list", "platform:roles:manage",
			"platform:permissions:list",
		},
	},
	{
		Name:        "platform_tenant_admin",
		DisplayName: "租户运营",
		Description: "管理租户的创建、编辑和成员配置，不能修改平台设置",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:tenants:manage", "platform:tenants:list",
			"platform:users:list",
		},
	},
	{
		Name:        "platform_user_admin",
		DisplayName: "用户运营",
		Description: "管理平台用户账号和角色分配",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:users:list", "platform:users:create", "platform:users:update", "platform:users:delete", "platform:users:reset_password",
			"platform:roles:list", "platform:roles:manage",
		},
	},
	{
		Name:        "platform_messenger",
		DisplayName: "消息运营",
		Description: "发送平台级站内信，查看租户列表（选择发送范围）",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:messages:send",
			"platform:tenants:list",
		},
	},
	{
		Name:        "platform_auditor",
		DisplayName: "平台审计员",
		Description: "查看和导出平台级审计日志，纯只读",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:audit:list", "platform:audit:export",
		},
	},
	{
		Name:        "platform_viewer",
		DisplayName: "平台只读",
		Description: "所有平台页面的只读权限，不能执行任何修改操作",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			"platform:tenants:list",
			"platform:users:list",
			"platform:roles:list",
			"platform:audit:list",
		},
	},
}
