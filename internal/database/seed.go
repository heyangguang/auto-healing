package database

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PermissionSeed 权限种子定义
type PermissionSeed struct {
	Code        string
	Name        string
	Description string
	Module      string
	Resource    string
	Action      string
}

// RoleSeed 角色种子定义
type RoleSeed struct {
	Name        string
	DisplayName string
	Description string
	IsSystem    bool
	Scope       string   // platform=平台级, tenant=租户级
	Permissions []string // 权限码列表
}

// AllPermissions 系统预置权限定义（单一信源）
var AllPermissions = []PermissionSeed{
	// ==================== 用户管理 ====================
	{Code: "user:list", Name: "查看用户列表", Module: "user", Resource: "user", Action: "read"},
	{Code: "user:create", Name: "创建用户", Module: "user", Resource: "user", Action: "create"},
	{Code: "user:update", Name: "更新用户", Module: "user", Resource: "user", Action: "update"},
	{Code: "user:delete", Name: "删除用户", Module: "user", Resource: "user", Action: "delete"},
	{Code: "user:reset_password", Name: "重置密码", Module: "user", Resource: "user", Action: "manage"},

	// ==================== 角色管理 ====================
	{Code: "role:list", Name: "查看角色列表", Module: "role", Resource: "role", Action: "read"},
	{Code: "role:create", Name: "创建角色", Module: "role", Resource: "role", Action: "create"},
	{Code: "role:update", Name: "更新角色", Module: "role", Resource: "role", Action: "update"},
	{Code: "role:delete", Name: "删除角色", Module: "role", Resource: "role", Action: "delete"},
	{Code: "role:assign", Name: "分配权限", Module: "role", Resource: "role", Action: "manage"},

	// ==================== 插件管理 ====================
	{Code: "plugin:list", Name: "查看插件列表", Module: "plugin", Resource: "plugin", Action: "read"},
	{Code: "plugin:detail", Name: "查看插件详情", Module: "plugin", Resource: "plugin", Action: "read"},
	{Code: "plugin:create", Name: "创建插件", Module: "plugin", Resource: "plugin", Action: "create"},
	{Code: "plugin:update", Name: "更新插件", Module: "plugin", Resource: "plugin", Action: "update"},
	{Code: "plugin:delete", Name: "删除插件", Module: "plugin", Resource: "plugin", Action: "delete"},
	{Code: "plugin:sync", Name: "触发同步", Module: "plugin", Resource: "plugin", Action: "execute"},
	{Code: "plugin:test", Name: "测试连接", Module: "plugin", Resource: "plugin", Action: "execute"},

	// ==================== 执行管理 ====================
	{Code: "repository:list", Name: "查看仓库列表", Module: "execution", Resource: "repository", Action: "read"},
	{Code: "repository:create", Name: "添加仓库", Module: "execution", Resource: "repository", Action: "create"},
	{Code: "repository:update", Name: "更新仓库", Module: "execution", Resource: "repository", Action: "update"},
	{Code: "repository:delete", Name: "删除仓库", Module: "execution", Resource: "repository", Action: "delete"},
	{Code: "repository:sync", Name: "同步仓库", Module: "execution", Resource: "repository", Action: "execute"},
	{Code: "playbook:list", Name: "查看Playbook列表", Module: "execution", Resource: "playbook", Action: "read"},
	{Code: "playbook:execute", Name: "执行Playbook", Module: "execution", Resource: "playbook", Action: "execute"},
	{Code: "task:list", Name: "查看任务列表", Module: "execution", Resource: "task", Action: "read"},
	{Code: "task:detail", Name: "查看任务详情", Module: "execution", Resource: "task", Action: "read"},
	{Code: "task:create", Name: "创建任务模板", Module: "execution", Resource: "task", Action: "create"},
	{Code: "task:update", Name: "更新任务模板", Module: "execution", Resource: "task", Action: "update"},
	{Code: "task:delete", Name: "删除任务模板", Module: "execution", Resource: "task", Action: "delete"},
	{Code: "task:cancel", Name: "取消任务", Module: "execution", Resource: "task", Action: "execute"},

	// ==================== 通知管理 ====================
	{Code: "channel:list", Name: "查看通知渠道", Module: "notification", Resource: "channel", Action: "read"},
	{Code: "channel:create", Name: "创建通知渠道", Module: "notification", Resource: "channel", Action: "create"},
	{Code: "channel:update", Name: "更新通知渠道", Module: "notification", Resource: "channel", Action: "update"},
	{Code: "channel:delete", Name: "删除通知渠道", Module: "notification", Resource: "channel", Action: "delete"},
	{Code: "template:list", Name: "查看通知模板", Module: "notification", Resource: "template", Action: "read"},
	{Code: "template:create", Name: "创建通知模板", Module: "notification", Resource: "template", Action: "create"},
	{Code: "template:update", Name: "更新通知模板", Module: "notification", Resource: "template", Action: "update"},
	{Code: "template:delete", Name: "删除通知模板", Module: "notification", Resource: "template", Action: "delete"},
	{Code: "notification:list", Name: "查看通知记录", Module: "notification", Resource: "notification", Action: "read"},
	{Code: "notification:send", Name: "发送通知", Module: "notification", Resource: "notification", Action: "execute"},

	// ==================== 自愈引擎 ====================
	{Code: "healing:flows:view", Name: "查看自愈流程", Module: "healing", Resource: "flows", Action: "read"},
	{Code: "healing:flows:create", Name: "创建自愈流程", Module: "healing", Resource: "flows", Action: "create"},
	{Code: "healing:flows:update", Name: "更新自愈流程", Module: "healing", Resource: "flows", Action: "update"},
	{Code: "healing:flows:delete", Name: "删除自愈流程", Module: "healing", Resource: "flows", Action: "delete"},
	{Code: "healing:rules:view", Name: "查看自愈规则", Module: "healing", Resource: "rules", Action: "read"},
	{Code: "healing:rules:create", Name: "创建自愈规则", Module: "healing", Resource: "rules", Action: "create"},
	{Code: "healing:rules:update", Name: "更新自愈规则", Module: "healing", Resource: "rules", Action: "update"},
	{Code: "healing:rules:delete", Name: "删除自愈规则", Module: "healing", Resource: "rules", Action: "delete"},
	{Code: "healing:instances:view", Name: "查看流程实例", Module: "healing", Resource: "instances", Action: "read"},
	{Code: "healing:approvals:view", Name: "查看审批任务", Module: "healing", Resource: "approvals", Action: "read"},
	{Code: "healing:approvals:approve", Name: "审批操作", Module: "healing", Resource: "approvals", Action: "execute"},
	{Code: "healing:trigger:view", Name: "查看待触发工单", Module: "healing", Resource: "trigger", Action: "read"},
	{Code: "healing:trigger:execute", Name: "手动触发自愈", Module: "healing", Resource: "trigger", Action: "execute"},

	// ==================== 工作流（预留） ====================
	{Code: "workflow:list", Name: "查看工作流列表", Module: "workflow", Resource: "workflow", Action: "read"},
	{Code: "workflow:detail", Name: "查看工作流详情", Module: "workflow", Resource: "workflow", Action: "read"},
	{Code: "workflow:create", Name: "创建工作流", Module: "workflow", Resource: "workflow", Action: "create"},
	{Code: "workflow:update", Name: "更新工作流", Module: "workflow", Resource: "workflow", Action: "update"},
	{Code: "workflow:delete", Name: "删除工作流", Module: "workflow", Resource: "workflow", Action: "delete"},
	{Code: "workflow:activate", Name: "激活工作流", Module: "workflow", Resource: "workflow", Action: "execute"},
	{Code: "workflow:run", Name: "手动触发执行", Module: "workflow", Resource: "workflow", Action: "execute"},

	// ==================== 系统管理 ====================
	{Code: "audit:list", Name: "查看审计日志", Module: "system", Resource: "audit", Action: "read"},
	{Code: "audit:export", Name: "导出审计日志", Module: "system", Resource: "audit", Action: "export"},
	{Code: "system:settings", Name: "系统设置", Module: "system", Resource: "settings", Action: "manage"},

	// ==================== Dashboard ====================
	{Code: "dashboard:view", Name: "查看监控面板", Module: "dashboard", Resource: "dashboard", Action: "read"},
	{Code: "dashboard:config:manage", Name: "管理面板配置", Module: "dashboard", Resource: "config", Action: "manage"},
	{Code: "dashboard:workspace:manage", Name: "管理工作区", Module: "dashboard", Resource: "workspace", Action: "manage"},

	// ==================== 站内信 ====================
	{Code: "site-message:list", Name: "查看站内信", Module: "site-message", Resource: "site-message", Action: "read"},
	{Code: "site-message:create", Name: "创建站内信", Module: "site-message", Resource: "site-message", Action: "create"},
	{Code: "site-message:settings:view", Name: "查看站内信设置", Module: "site-message", Resource: "settings", Action: "read"},
	{Code: "site-message:settings:manage", Name: "管理站内信设置", Module: "site-message", Resource: "settings", Action: "manage"},

	// ==================== 平台管理 ====================
	{Code: "platform:settings:manage", Name: "管理平台设置", Module: "platform", Resource: "settings", Action: "manage"},
	{Code: "platform:tenants:manage", Name: "管理租户", Module: "platform", Resource: "tenants", Action: "manage"},
	{Code: "platform:tenants:list", Name: "查看租户列表", Module: "platform", Resource: "tenants", Action: "read"},
	{Code: "platform:users:list", Name: "查看平台用户", Module: "platform", Resource: "users", Action: "read"},
	{Code: "platform:users:create", Name: "创建平台用户", Module: "platform", Resource: "users", Action: "create"},
	{Code: "platform:users:update", Name: "更新平台用户", Module: "platform", Resource: "users", Action: "update"},
	{Code: "platform:users:delete", Name: "删除平台用户", Module: "platform", Resource: "users", Action: "delete"},
	{Code: "platform:users:reset_password", Name: "重置平台用户密码", Module: "platform", Resource: "users", Action: "manage"},
	{Code: "platform:roles:list", Name: "查看平台角色", Module: "platform", Resource: "roles", Action: "read"},
	{Code: "platform:roles:manage", Name: "管理平台角色", Module: "platform", Resource: "roles", Action: "manage"},
	{Code: "platform:permissions:list", Name: "查看平台权限", Module: "platform", Resource: "permissions", Action: "read"},
	{Code: "platform:audit:list", Name: "查看平台审计日志", Module: "platform", Resource: "audit", Action: "read"},
	{Code: "platform:audit:export", Name: "导出平台审计日志", Module: "platform", Resource: "audit", Action: "export"},
	{Code: "platform:messages:send", Name: "发送平台站内信", Module: "platform", Resource: "messages", Action: "create"},
}

// SystemRoles 系统预置角色及其默认权限
var SystemRoles = []RoleSeed{

	{
		Name:        "platform_admin",
		DisplayName: "平台超级管理员",
		Description: "拥有平台所有权限，可管理租户、用户、角色、设置等一切平台资源",
		IsSystem:    true,
		Scope:       "platform",
		Permissions: []string{
			// 平台超级管理员拥有所有平台权限（代码层面还会赋 "*" 通配符）
			"platform:tenants:manage", "platform:tenants:list",
			"platform:settings:manage",
			"platform:users:list", "platform:users:create", "platform:users:update", "platform:users:delete", "platform:users:reset_password",
			"platform:roles:list", "platform:roles:manage",
			"platform:permissions:list",
			"platform:audit:list", "platform:audit:export",
			"platform:messages:send",
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
			"platform:users:list", // 查看用户列表（用于分配租户成员）
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
			"platform:tenants:list", // 发送消息时需要选择目标租户
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
	{
		Name:        "admin",
		DisplayName: "管理员",
		Description: "可管理用户、角色、插件、执行任务等核心资源",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 用户管理（完整）
			"user:list", "user:create", "user:update", "user:delete", "user:reset_password",
			// 角色管理（完整）
			"role:list", "role:create", "role:update", "role:delete", "role:assign",
			// 插件管理（完整）
			"plugin:list", "plugin:detail", "plugin:create", "plugin:update", "plugin:delete", "plugin:sync", "plugin:test",
			// 执行管理（完整）
			"repository:list", "repository:create", "repository:update", "repository:delete", "repository:sync",
			"playbook:list", "playbook:execute",
			"task:list", "task:detail", "task:create", "task:update", "task:delete", "task:cancel",
			// 通知管理（完整）
			"channel:list", "channel:create", "channel:update", "channel:delete",
			"template:list", "template:create", "template:update", "template:delete",
			"notification:list", "notification:send",
			// 自愈引擎（完整）
			"healing:flows:view", "healing:flows:create", "healing:flows:update", "healing:flows:delete",
			"healing:rules:view", "healing:rules:create", "healing:rules:update", "healing:rules:delete",
			"healing:instances:view",
			"healing:approvals:view", "healing:approvals:approve",
			"healing:trigger:view", "healing:trigger:execute",
			// Dashboard
			"dashboard:view", "dashboard:config:manage", "dashboard:workspace:manage",
			// 站内信
			"site-message:list", "site-message:create", "site-message:settings:view", "site-message:settings:manage",
			// 平台权限查看
			"platform:permissions:list",
			// 平台设置
			"platform:settings:manage",
			// 租户管理
			"platform:tenants:manage",
			// 审计日志
			"audit:list", "audit:export",
			// 系统设置
			"system:settings",
			// 工作流（完整）
			"workflow:list", "workflow:detail", "workflow:create", "workflow:update", "workflow:delete", "workflow:activate", "workflow:run",
		},
	},
	{
		Name:        "operator",
		DisplayName: "运维人员",
		Description: "可执行运维操作、管理自愈流程和查看系统信息",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 插件管理（操作类）
			"plugin:list", "plugin:detail", "plugin:sync", "plugin:test",
			// 执行管理（操作类）
			"repository:list", "repository:sync",
			"playbook:list", "playbook:execute",
			"task:list", "task:detail", "task:create", "task:update", "task:cancel",
			// 通知管理（查看 + 发送）
			"channel:list", "template:list", "notification:list", "notification:send",
			// 自愈引擎（完整操作）
			"healing:flows:view", "healing:flows:create", "healing:flows:update",
			"healing:rules:view", "healing:rules:create", "healing:rules:update",
			"healing:instances:view",
			"healing:approvals:view", "healing:approvals:approve",
			"healing:trigger:view", "healing:trigger:execute",
			// Dashboard（只读）
			"dashboard:view",
			// 站内信
			"site-message:list",
			// 审计日志（只读）
			"audit:list",
			// 工作流（操作类）
			"workflow:list", "workflow:detail", "workflow:update", "workflow:activate", "workflow:run",
		},
	},
	{
		Name:        "viewer",
		DisplayName: "只读用户",
		Description: "只能查看系统信息，不能进行任何修改操作",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 插件（只读）
			"plugin:list", "plugin:detail",
			// 执行（只读）
			"repository:list", "playbook:list", "task:list", "task:detail",
			// 通知（只读）
			"channel:list", "template:list", "notification:list",
			// 自愈（只读）
			"healing:flows:view", "healing:rules:view", "healing:instances:view",
			"healing:approvals:view", "healing:trigger:view",
			// Dashboard（只读）
			"dashboard:view",
			// 站内信（只读）
			"site-message:list",
			// 审计日志（只读）
			"audit:list",
			// 工作流（只读）
			"workflow:list", "workflow:detail",
		},
	},

	// ==================== 细分职能角色 ====================
	{
		Name:        "healing_engineer",
		DisplayName: "自愈工程师",
		Description: "专注自愈流程的设计和运维，可管理流程、规则、审批和触发，其他模块只读",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 自愈引擎（完整）
			"healing:flows:view", "healing:flows:create", "healing:flows:update", "healing:flows:delete",
			"healing:rules:view", "healing:rules:create", "healing:rules:update", "healing:rules:delete",
			"healing:instances:view",
			"healing:approvals:view", "healing:approvals:approve",
			"healing:trigger:view", "healing:trigger:execute",
			// 执行（只读 + 执行 Playbook）
			"task:list", "task:detail",
			"repository:list",
			"playbook:list", "playbook:execute",
			// 插件（只读 + 操作）
			"plugin:list", "plugin:detail", "plugin:sync", "plugin:test",
			// 通知（只读）
			"channel:list", "template:list", "notification:list",
			// Dashboard（只读）
			"dashboard:view",
			// 站内信（只读）
			"site-message:list",
			// 审计日志（只读）
			"audit:list",
			// 工作流（只读）
			"workflow:list", "workflow:detail",
		},
	},
	{
		Name:        "devops_engineer",
		DisplayName: "运维工程师",
		Description: "专注任务执行和自动化运维，可管理执行任务、Playbook和Git仓库，其他模块只读",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 执行管理（完整）
			"task:list", "task:detail", "task:create", "task:update", "task:delete", "task:cancel",
			"repository:list", "repository:create", "repository:update", "repository:delete", "repository:sync",
			"playbook:list", "playbook:execute",
			// 插件（只读 + 操作）
			"plugin:list", "plugin:detail", "plugin:sync", "plugin:test",
			// 自愈（只读 + 手动触发）
			"healing:flows:view", "healing:rules:view", "healing:instances:view",
			"healing:approvals:view", "healing:trigger:view", "healing:trigger:execute",
			// 通知（只读）
			"channel:list", "template:list", "notification:list",
			// Dashboard（只读）
			"dashboard:view",
			// 站内信（只读）
			"site-message:list",
			// 审计日志（只读）
			"audit:list",
			// 工作流（操作类）
			"workflow:list", "workflow:detail", "workflow:update", "workflow:activate", "workflow:run",
		},
	},
	{
		Name:        "notification_manager",
		DisplayName: "通知管理员",
		Description: "管理通知渠道、模板和站内信，可创建和发送通知，其他模块只读",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 通知管理（完整）
			"channel:list", "channel:create", "channel:update", "channel:delete",
			"template:list", "template:create", "template:update", "template:delete",
			"notification:list", "notification:send",
			// 站内信（完整）
			"site-message:list", "site-message:create", "site-message:settings:view", "site-message:settings:manage",
			// 插件（只读）
			"plugin:list", "plugin:detail",
			// 执行（只读）
			"task:list", "task:detail", "repository:list", "playbook:list",
			// 自愈（只读）
			"healing:flows:view", "healing:rules:view", "healing:instances:view",
			"healing:approvals:view", "healing:trigger:view",
			// Dashboard（只读）
			"dashboard:view",
			// 审计日志（只读）
			"audit:list",
			// 工作流（只读）
			"workflow:list", "workflow:detail",
		},
	},
	{
		Name:        "monitor_admin",
		DisplayName: "监控管理员",
		Description: "管理监控插件和Dashboard面板，可配置数据源和面板布局，其他模块只读",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 插件管理（完整）
			"plugin:list", "plugin:detail", "plugin:create", "plugin:update", "plugin:delete", "plugin:sync", "plugin:test",
			// Dashboard（完整）
			"dashboard:view", "dashboard:config:manage", "dashboard:workspace:manage",
			// 执行（只读）
			"task:list", "task:detail", "repository:list", "playbook:list",
			// 通知（只读）
			"channel:list", "template:list", "notification:list",
			// 自愈（只读）
			"healing:flows:view", "healing:rules:view", "healing:instances:view",
			"healing:approvals:view", "healing:trigger:view",
			// 站内信（只读）
			"site-message:list",
			// 审计日志（只读）
			"audit:list",
			// 工作流（只读）
			"workflow:list", "workflow:detail",
		},
	},
	{
		Name:        "auditor",
		DisplayName: "审计员",
		Description: "租户安全审计，可查看和导出审计日志，所有业务模块只读",
		IsSystem:    true,
		Scope:       "tenant",
		Permissions: []string{
			// 审计日志（完整）
			"audit:list", "audit:export",
			// 插件（只读）
			"plugin:list", "plugin:detail",
			// 执行（只读）
			"task:list", "task:detail", "repository:list", "playbook:list",
			// 通知（只读）
			"channel:list", "template:list", "notification:list",
			// 自愈（只读）
			"healing:flows:view", "healing:rules:view", "healing:instances:view",
			"healing:approvals:view", "healing:trigger:view",
			// Dashboard（只读）
			"dashboard:view",
			// 站内信（只读）
			"site-message:list",
			// 工作流（只读）
			"workflow:list", "workflow:detail",
		},
	},
}

// SyncPermissionsAndRoles 同步预置权限和角色（启动时调用）
// 使用 UPSERT 策略：存在则更新名称/描述，不存在则创建
func SyncPermissionsAndRoles() error {
	ctx := context.Background()

	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// ===== Phase 1: 同步权限 =====
		logger.Info("同步系统预置权限...")
		permCodeToID := make(map[string]string)

		for _, seed := range AllPermissions {
			perm := model.Permission{
				Code:        seed.Code,
				Name:        seed.Name,
				Description: seed.Description,
				Module:      seed.Module,
				Resource:    seed.Resource,
				Action:      seed.Action,
			}

			// Upsert: 按 code 查找，存在则更新名称等字段，不存在则创建
			result := tx.Where("code = ?", seed.Code).First(&model.Permission{})
			if result.Error == gorm.ErrRecordNotFound {
				if err := tx.Create(&perm).Error; err != nil {
					return err
				}
			} else if result.Error == nil {
				tx.Model(&model.Permission{}).Where("code = ?", seed.Code).Updates(map[string]interface{}{
					"name":        seed.Name,
					"description": seed.Description,
					"module":      seed.Module,
					"resource":    seed.Resource,
					"action":      seed.Action,
				})
			}
		}

		// 获取所有权限的 code -> ID 映射
		var allPerms []model.Permission
		if err := tx.Find(&allPerms).Error; err != nil {
			return err
		}
		for _, p := range allPerms {
			permCodeToID[p.Code] = p.ID.String()
		}

		logger.Info("权限同步完成，共 %d 个权限", len(allPerms))

		// ===== Phase 2: 同步角色 =====
		logger.Info("同步系统预置角色...")
		for _, roleSeed := range SystemRoles {
			var role model.Role
			result := tx.Where("name = ?", roleSeed.Name).First(&role)
			if result.Error == gorm.ErrRecordNotFound {
				scope := roleSeed.Scope
				if scope == "" {
					scope = "tenant"
				}
				role = model.Role{
					Name:        roleSeed.Name,
					DisplayName: roleSeed.DisplayName,
					Description: roleSeed.Description,
					IsSystem:    roleSeed.IsSystem,
					Scope:       scope,
				}
				if err := tx.Create(&role).Error; err != nil {
					return err
				}
			} else if result.Error == nil {
				// 更新描述和 scope（不覆盖 display_name，用户可能已修改）
				updateScope := roleSeed.Scope
				if updateScope == "" {
					updateScope = "tenant"
				}
				tx.Model(&role).Updates(map[string]interface{}{
					"description": roleSeed.Description,
					"is_system":   roleSeed.IsSystem,
					"scope":       updateScope,
				})
			}

			// ===== Phase 3: 同步角色权限（仅系统角色） =====
			if roleSeed.IsSystem && roleSeed.Permissions != nil {
				// super_admin 的权限通过通配符实现，不需要分配
				// 但对于 admin/operator/viewer，确保它们至少有种子文件定义的权限
				// 策略：只添加缺失的权限，不删除管理员手动添加的额外权限
				var existingRolePerms []model.RolePermission
				tx.Where("role_id = ?", role.ID).Find(&existingRolePerms)
				existingPermIDs := make(map[string]bool)
				for _, rp := range existingRolePerms {
					existingPermIDs[rp.PermissionID.String()] = true
				}

				for _, permCode := range roleSeed.Permissions {
					permID, ok := permCodeToID[permCode]
					if !ok {
						logger.Warn("权限码 %s 未找到，跳过", permCode)
						continue
					}
					if existingPermIDs[permID] {
						continue // 已有，跳过
					}
					rp := model.RolePermission{}
					rp.RoleID = role.ID
					rp.PermissionID = parseUUID(permID)
					tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rp)
				}
			}

			// 平台管理员通过显式权限列表管理，无需通配符
		}

		logger.Info("角色权限同步完成")
		return nil
	})
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
