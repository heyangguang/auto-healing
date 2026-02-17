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
	{Code: "dashboard:workspace:manage", Name: "管理工作区", Module: "dashboard", Resource: "workspace", Action: "manage"},

	// ==================== 站内信 ====================
	{Code: "site-message:create", Name: "创建站内信", Module: "site-message", Resource: "site-message", Action: "create"},
}

// SystemRoles 系统预置角色及其默认权限
var SystemRoles = []RoleSeed{
	{
		Name:        "super_admin",
		DisplayName: "超级管理员",
		Description: "拥有系统所有权限，可管理所有资源",
		IsSystem:    true,
		Permissions: nil, // super_admin 通过 * 通配符获得所有权限，无需逐个分配
	},
	{
		Name:        "admin",
		DisplayName: "管理员",
		Description: "可管理用户、角色、插件、执行任务等核心资源",
		IsSystem:    true,
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
			"dashboard:workspace:manage",
			// 站内信
			"site-message:create",
			// 审计日志（只读）
			"audit:list",
		},
	},
	{
		Name:        "operator",
		DisplayName: "运维人员",
		Description: "可执行运维操作、管理自愈流程和查看系统信息",
		IsSystem:    true,
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
			// 审计日志（只读）
			"audit:list",
		},
	},
	{
		Name:        "viewer",
		DisplayName: "只读用户",
		Description: "只能查看系统信息，不能进行任何修改操作",
		IsSystem:    true,
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
			// 审计日志（只读）
			"audit:list",
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
				role = model.Role{
					Name:        roleSeed.Name,
					DisplayName: roleSeed.DisplayName,
					Description: roleSeed.Description,
					IsSystem:    roleSeed.IsSystem,
				}
				if err := tx.Create(&role).Error; err != nil {
					return err
				}
			} else if result.Error == nil {
				// 更新描述（不覆盖 display_name，用户可能已修改）
				tx.Model(&role).Updates(map[string]interface{}{
					"description": roleSeed.Description,
					"is_system":   roleSeed.IsSystem,
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

			// super_admin: 分配所有权限
			if roleSeed.Name == "super_admin" {
				var existingCount int64
				tx.Model(&model.RolePermission{}).Where("role_id = ?", role.ID).Count(&existingCount)
				if int(existingCount) < len(allPerms) {
					for _, perm := range allPerms {
						rp := model.RolePermission{
							RoleID:       role.ID,
							PermissionID: perm.ID,
						}
						tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rp)
					}
				}
			}
		}

		logger.Info("角色权限同步完成")
		return nil
	})
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
