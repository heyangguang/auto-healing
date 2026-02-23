package handler

// ==================== 路由注册规范 ====================
//
// 【重要】固定路径必须注册在参数路由（/:id）之前！
//
// Gin 使用基数树匹配路由，如果 /:id 注册在固定路径之前，
// 固定路径（如 /stats）会被当作 :id 参数匹配，导致返回"无效的ID"。
//
// ✅ 正确顺序：
//   group.GET("/stats", handler)   // 固定路径在前
//   group.GET("/:id", handler)     // 参数路由在后
//
// ❌ 错误顺序：
//   group.GET("/:id", handler)     // 参数路由在前 → 吞掉 /stats
//   group.GET("/stats", handler)   // 永远匹配不到
//
// 同理适用于 /validate, /export, /pending 等所有固定子路径。
// =======================================================

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handlers 所有处理器集合
type Handlers struct {
	Auth             *AuthHandler
	User             *UserHandler
	TenantUser       *TenantUserHandler // 租户级用户管理
	Role             *RoleHandler
	Permission       *PermissionHandler
	Plugin           *PluginHandler
	CMDB             *CMDBHandler
	Secrets          *SecretsHandler
	GitRepo          *GitRepoHandler
	Playbook         *PlaybookHandler
	Execution        *ExecutionHandler
	Schedule         *ScheduleHandler
	Notification     *NotificationHandler
	Healing          *HealingHandler
	Dashboard        *DashboardHandler
	Preference       *PreferenceHandler
	Audit            *AuditHandler
	PlatformAudit    *PlatformAuditHandler
	Activity         *UserActivityHandler
	Search           *SearchHandler
	SiteMessage      *SiteMessageHandler
	PlatformSettings *PlatformSettingsHandler
	Tenant           *TenantHandler
	Workbench        *WorkbenchHandler
	Dictionary       *DictionaryHandler
}

// NewHandlers 创建所有处理器
func NewHandlers(cfg *config.Config) *Handlers {
	authHandler := NewAuthHandler(cfg)

	return &Handlers{
		Auth:             authHandler,
		User:             NewUserHandler(authHandler.authSvc),
		TenantUser:       NewTenantUserHandler(authHandler.authSvc),
		Role:             NewRoleHandler(),
		Permission:       NewPermissionHandler(),
		Plugin:           NewPluginHandler(),
		CMDB:             NewCMDBHandler(),
		Secrets:          NewSecretsHandler(),
		GitRepo:          NewGitRepoHandler(),
		Playbook:         NewPlaybookHandler(),
		Execution:        NewExecutionHandler(),
		Schedule:         NewScheduleHandler(),
		Notification:     NewNotificationHandler(),
		Healing:          NewHealingHandler(),
		Dashboard:        NewDashboardHandler(),
		Preference:       NewPreferenceHandler(),
		Audit:            NewAuditHandler(),
		PlatformAudit:    NewPlatformAuditHandler(),
		Activity:         NewUserActivityHandler(),
		Search:           NewSearchHandler(),
		SiteMessage:      NewSiteMessageHandler(),
		PlatformSettings: NewPlatformSettingsHandler(),
		Tenant:           NewTenantHandler(authHandler.authSvc),
		Workbench:        NewWorkbenchHandler(),
		Dictionary:       NewDictionaryHandler(),
	}
}

// SetupRoutes 设置所有路由
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	// 创建处理器
	h := NewHandlers(cfg)

	api := r.Group("/api/v1")

	// ==================== 公开路由 ====================
	auth := api.Group("/auth")
	{
		auth.POST("/login", h.Auth.Login)
		auth.POST("/refresh", h.Auth.RefreshToken)
	}

	// ==================== 需要认证的路由 ====================
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(h.Auth.GetJWTService()))
	protected.Use(middleware.AuditMiddleware())
	protected.Use(middleware.TenantMiddleware())
	{
		// -------------------- 全局搜索 --------------------
		protected.GET("/search", h.Search.GlobalSearch)

		// 用户认证相关
		protected.GET("/auth/me", h.Auth.GetCurrentUser)
		protected.GET("/auth/profile", h.Auth.GetProfile)
		protected.PUT("/auth/profile", h.Auth.UpdateProfile)
		protected.PUT("/auth/password", h.Auth.ChangePassword)
		protected.POST("/auth/logout", h.Auth.Logout)

		// -------------------- 租户级用户管理 --------------------
		tenantUsers := protected.Group("/tenant/users")
		{
			tenantUsers.GET("", middleware.RequirePermission("user:list"), h.TenantUser.ListTenantUsers)               // 租户级用户列表
			tenantUsers.GET("/simple", middleware.RequirePermission("user:list"), h.TenantUser.ListSimpleUsers)        // 租户级简要用户列表
			tenantUsers.POST("", middleware.RequirePermission("user:create"), h.TenantUser.CreateTenantUser)           // 租户级创建用户
			tenantUsers.GET("/:id", middleware.RequirePermission("user:list"), h.User.GetUser)                         // 租户级用户详情
			tenantUsers.PUT("/:id", middleware.RequirePermission("user:update"), h.User.UpdateUser)                    // 租户级更新用户
			tenantUsers.DELETE("/:id", middleware.RequirePermission("user:delete"), h.User.DeleteUser)                 // 租户级删除用户
			tenantUsers.POST("/:id/reset-password", middleware.RequirePermission("user:create"), h.User.ResetPassword) // 租户级重置密码
			tenantUsers.PUT("/:id/roles", middleware.RequirePermission("role:assign"), h.User.AssignUserRoles)         // 租户级分配角色
		}

		// -------------------- 租户级角色管理 --------------------
		tenantRoles := protected.Group("/tenant/roles")
		{
			tenantRoles.GET("", middleware.RequirePermission("role:list"), h.Role.ListTenantRoles)
			tenantRoles.GET("/:id", middleware.RequirePermission("role:list"), h.Role.GetRole)
			tenantRoles.GET("/:id/users", middleware.RequirePermission("role:list"), h.Role.GetTenantRoleUsers)
			tenantRoles.POST("", middleware.RequirePermission("role:create"), h.Role.CreateRole)
			tenantRoles.PUT("/:id", middleware.RequirePermission("role:update"), h.Role.UpdateRole)
			tenantRoles.DELETE("/:id", middleware.RequirePermission("role:delete"), h.Role.DeleteRole)
			tenantRoles.PUT("/:id/permissions", middleware.RequirePermission("role:assign"), h.Role.AssignRolePermissions)
		}

		// -------------------- 用户偏好设置 --------------------
		userPrefs := protected.Group("/user/preferences")
		{
			userPrefs.GET("", h.Preference.GetPreferences)
			userPrefs.PUT("", h.Preference.UpdatePreferences)
			userPrefs.PATCH("", h.Preference.PatchPreferences)
		}

		// -------------------- 用户收藏 --------------------
		userFavorites := protected.Group("/user/favorites")
		{
			userFavorites.GET("", h.Activity.ListFavorites)
			userFavorites.POST("", h.Activity.AddFavorite)
			userFavorites.DELETE("/:menu_key", h.Activity.RemoveFavorite)
		}

		// -------------------- 最近访问 --------------------
		userRecents := protected.Group("/user/recents")
		{
			userRecents.GET("", h.Activity.ListRecents)
			userRecents.POST("", h.Activity.RecordRecent)
		}

		// -------------------- 插件管理 --------------------
		plugins := protected.Group("/plugins")
		{
			plugins.GET("", middleware.RequirePermission("plugin:list"), h.Plugin.ListPlugins)
			plugins.GET("/stats", middleware.RequirePermission("plugin:list"), h.Plugin.GetPluginStats)
			plugins.POST("", middleware.RequirePermission("plugin:create"), h.Plugin.CreatePlugin)
			plugins.GET("/:id", middleware.RequirePermission("plugin:detail"), h.Plugin.GetPlugin)
			plugins.PUT("/:id", middleware.RequirePermission("plugin:update"), h.Plugin.UpdatePlugin)
			plugins.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.Plugin.DeletePlugin)
			plugins.POST("/:id/test", middleware.RequirePermission("plugin:test"), h.Plugin.TestPlugin)
			plugins.POST("/:id/activate", middleware.RequirePermission("plugin:update"), h.Plugin.ActivatePlugin)
			plugins.POST("/:id/deactivate", middleware.RequirePermission("plugin:update"), h.Plugin.DeactivatePlugin)
			plugins.POST("/:id/sync", middleware.RequirePermission("plugin:sync"), h.Plugin.SyncPlugin)
			plugins.GET("/:id/logs", middleware.RequirePermission("plugin:list"), h.Plugin.GetPluginSyncLogs)
		}

		// -------------------- 执行任务模板 --------------------
		execTasks := protected.Group("/execution-tasks")
		{
			execTasks.GET("", middleware.RequirePermission("task:list"), h.Execution.ListTasks)
			execTasks.POST("", middleware.RequirePermission("playbook:execute"), h.Execution.CreateTask)
			execTasks.GET("/stats", middleware.RequirePermission("task:list"), h.Execution.GetTaskStats)
			execTasks.POST("/batch-confirm-review", middleware.RequirePermission("task:update"), h.Execution.BatchConfirmReview)
			execTasks.GET("/:id", middleware.RequirePermission("task:detail"), h.Execution.GetTask)
			execTasks.PUT("/:id", middleware.RequirePermission("task:update"), h.Execution.UpdateTask)
			execTasks.DELETE("/:id", middleware.RequirePermission("task:delete"), h.Execution.DeleteTask)
			execTasks.POST("/:id/execute", middleware.RequirePermission("playbook:execute"), h.Execution.ExecuteTask)
			execTasks.POST("/:id/confirm-review", middleware.RequirePermission("task:update"), h.Execution.ConfirmReview)
			execTasks.GET("/:id/runs", middleware.RequirePermission("task:detail"), h.Execution.ListRuns)
		}

		// -------------------- 执行记录 --------------------
		execRuns := protected.Group("/execution-runs")
		{
			execRuns.GET("", middleware.RequirePermission("task:list"), h.Execution.ListAllRuns)
			// 统计接口（必须在 /:id 之前注册，避免路径冲突）
			execRuns.GET("/stats", middleware.RequirePermission("task:list"), h.Execution.GetRunStats)
			execRuns.GET("/trend", middleware.RequirePermission("task:list"), h.Execution.GetRunTrend)
			execRuns.GET("/trigger-distribution", middleware.RequirePermission("task:list"), h.Execution.GetTriggerDistribution)
			execRuns.GET("/top-failed", middleware.RequirePermission("task:list"), h.Execution.GetTopFailedTasks)
			execRuns.GET("/top-active", middleware.RequirePermission("task:list"), h.Execution.GetTopActiveTasks)
			// 动态路由
			execRuns.GET("/:id", middleware.RequirePermission("task:detail"), h.Execution.GetRun)
			execRuns.GET("/:id/logs", middleware.RequirePermission("task:detail"), h.Execution.GetRunLogs)
			execRuns.GET("/:id/stream", middleware.RequirePermission("task:detail"), h.Execution.StreamLogs)
			execRuns.POST("/:id/cancel", middleware.RequirePermission("task:cancel"), h.Execution.CancelRun)
		}

		// -------------------- 定时任务调度 --------------------
		schedules := protected.Group("/execution-schedules")
		{
			schedules.GET("", middleware.RequirePermission("task:list"), h.Schedule.List)
			schedules.POST("", middleware.RequirePermission("task:create"), h.Schedule.Create)
			schedules.GET("/stats", middleware.RequirePermission("task:list"), h.Schedule.GetStats)
			schedules.GET("/timeline", middleware.RequirePermission("task:list"), h.Schedule.GetTimeline) // 轻量接口：调度时间线可视化
			schedules.GET("/:id", middleware.RequirePermission("task:detail"), h.Schedule.Get)
			schedules.PUT("/:id", middleware.RequirePermission("task:update"), h.Schedule.Update)
			schedules.DELETE("/:id", middleware.RequirePermission("task:delete"), h.Schedule.Delete)
			schedules.POST("/:id/enable", middleware.RequirePermission("task:update"), h.Schedule.Enable)
			schedules.POST("/:id/disable", middleware.RequirePermission("task:update"), h.Schedule.Disable)
		}

		// -------------------- 通知渠道 --------------------
		channels := protected.Group("/channels")
		{
			channels.GET("", middleware.RequirePermission("channel:list"), h.Notification.ListChannels)
			channels.POST("", middleware.RequirePermission("channel:create"), h.Notification.CreateChannel)
			channels.GET("/:id", middleware.RequirePermission("channel:list"), h.Notification.GetChannel)
			channels.PUT("/:id", middleware.RequirePermission("channel:update"), h.Notification.UpdateChannel)
			channels.DELETE("/:id", middleware.RequirePermission("channel:delete"), h.Notification.DeleteChannel)
			channels.POST("/:id/test", middleware.RequirePermission("channel:list"), h.Notification.TestChannel)
		}

		// -------------------- 通知模板 --------------------
		templates := protected.Group("/templates")
		{
			templates.GET("", middleware.RequirePermission("template:list"), h.Notification.ListTemplates)
			templates.POST("", middleware.RequirePermission("template:create"), h.Notification.CreateTemplate)
			templates.GET("/:id", middleware.RequirePermission("template:list"), h.Notification.GetTemplate)
			templates.PUT("/:id", middleware.RequirePermission("template:update"), h.Notification.UpdateTemplate)
			templates.DELETE("/:id", middleware.RequirePermission("template:delete"), h.Notification.DeleteTemplate)
			templates.POST("/:id/preview", middleware.RequirePermission("template:list"), h.Notification.PreviewTemplate)
		}
		protected.GET("/template-variables", middleware.RequirePermission("template:list"), h.Notification.GetAvailableVariables)

		// -------------------- 通知发送 --------------------
		notifications := protected.Group("/notifications")
		{
			notifications.POST("/send", middleware.RequirePermission("notification:send"), h.Notification.SendNotification)
			notifications.GET("", middleware.RequirePermission("notification:list"), h.Notification.ListNotifications)
			notifications.GET("/stats", middleware.RequirePermission("notification:list"), h.Notification.GetStats)
			notifications.GET("/:id", middleware.RequirePermission("notification:list"), h.Notification.GetNotification)
		}

		// -------------------- 审计日志 --------------------
		auditLogs := protected.Group("/audit-logs")
		{
			auditLogs.GET("", middleware.RequirePermission("audit:list"), h.Audit.ListAuditLogs)
			auditLogs.GET("/stats", middleware.RequirePermission("audit:list"), h.Audit.GetAuditStats)
			auditLogs.GET("/user-ranking", middleware.RequirePermission("audit:list"), h.Audit.GetUserRanking)
			auditLogs.GET("/action-grouping", middleware.RequirePermission("audit:list"), h.Audit.GetActionGrouping)
			auditLogs.GET("/resource-stats", middleware.RequirePermission("audit:list"), h.Audit.GetResourceTypeStats)
			auditLogs.GET("/trend", middleware.RequirePermission("audit:list"), h.Audit.GetTrend)
			auditLogs.GET("/high-risk", middleware.RequirePermission("audit:list"), h.Audit.GetHighRiskLogs)
			auditLogs.GET("/export", middleware.RequirePermission("audit:export"), h.Audit.ExportAuditLogs)
			auditLogs.GET("/:id", middleware.RequirePermission("audit:list"), h.Audit.GetAuditLog)
		}

		// -------------------- 工单/事件 --------------------
		incidents := protected.Group("/incidents")
		{
			incidents.GET("/stats", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncidentStats)
			incidents.GET("", middleware.RequirePermission("plugin:list"), h.Plugin.ListIncidents)
			incidents.POST("/batch-reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.BatchResetIncidentScan) // 固定路径在 /:id 之前
			incidents.GET("/:id", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncident)
			incidents.POST("/:id/reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.ResetIncidentScan)
			incidents.POST("/:id/close", middleware.RequirePermission("plugin:sync"), h.Plugin.CloseIncident)
		}

		// -------------------- CMDB 配置项 --------------------
		cmdb := protected.Group("/cmdb")
		{
			cmdb.GET("", middleware.RequirePermission("plugin:list"), h.CMDB.ListCMDBItems)
			cmdb.GET("/stats", middleware.RequirePermission("plugin:list"), h.CMDB.GetCMDBStats)
			cmdb.POST("/batch-test-connection", middleware.RequirePermission("plugin:sync"), h.CMDB.BatchTestConnection)
			cmdb.POST("/batch/maintenance", middleware.RequirePermission("plugin:update"), h.CMDB.BatchEnterMaintenance)
			cmdb.POST("/batch/resume", middleware.RequirePermission("plugin:update"), h.CMDB.BatchExitMaintenance)
			cmdb.GET("/ids", middleware.RequirePermission("plugin:list"), h.CMDB.ListCMDBItemIDs) // 轻量接口：仅返回 ID 列表（全选用）
			cmdb.GET("/:id", middleware.RequirePermission("plugin:list"), h.CMDB.GetCMDBItem)
			cmdb.POST("/:id/test-connection", middleware.RequirePermission("plugin:sync"), h.CMDB.TestConnection)
			cmdb.POST("/:id/maintenance", middleware.RequirePermission("plugin:update"), h.CMDB.EnterMaintenance)
			cmdb.POST("/:id/resume", middleware.RequirePermission("plugin:update"), h.CMDB.ExitMaintenance)
			cmdb.GET("/:id/maintenance-logs", middleware.RequirePermission("plugin:list"), h.CMDB.GetMaintenanceLogs)
		}

		// -------------------- 密钥管理 --------------------
		secretsSources := protected.Group("/secrets-sources")
		{
			secretsSources.GET("", middleware.RequirePermission("plugin:list"), h.Secrets.ListSources)
			secretsSources.POST("", middleware.RequirePermission("plugin:create"), h.Secrets.CreateSource)
			secretsSources.GET("/stats", middleware.RequirePermission("plugin:list"), h.Secrets.GetStats)
			secretsSources.GET("/:id", middleware.RequirePermission("plugin:list"), h.Secrets.GetSource)
			secretsSources.PUT("/:id", middleware.RequirePermission("plugin:update"), h.Secrets.UpdateSource)
			secretsSources.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.Secrets.DeleteSource)
			secretsSources.POST("/:id/test", middleware.RequirePermission("plugin:test"), h.Secrets.TestConnection)
			secretsSources.POST("/:id/test-query", middleware.RequirePermission("plugin:test"), h.Secrets.TestQuery)
			secretsSources.POST("/:id/enable", middleware.RequirePermission("plugin:update"), h.Secrets.Enable)
			secretsSources.POST("/:id/disable", middleware.RequirePermission("plugin:update"), h.Secrets.Disable)
		}
		protected.POST("/secrets/query", middleware.RequirePermission("plugin:list"), h.Secrets.QuerySecret)

		// -------------------- Git 仓库 --------------------
		gitRepos := protected.Group("/git-repos")
		{
			gitRepos.POST("/validate", middleware.RequirePermission("plugin:list"), h.GitRepo.ValidateRepo)
			gitRepos.GET("", middleware.RequirePermission("plugin:list"), h.GitRepo.ListRepos)
			gitRepos.POST("", middleware.RequirePermission("plugin:create"), h.GitRepo.CreateRepo)
			gitRepos.GET("/stats", middleware.RequirePermission("plugin:list"), h.GitRepo.GetStats)
			gitRepos.GET("/:id", middleware.RequirePermission("plugin:list"), h.GitRepo.GetRepo)
			gitRepos.PUT("/:id", middleware.RequirePermission("plugin:update"), h.GitRepo.UpdateRepo)
			gitRepos.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.GitRepo.DeleteRepo)
			gitRepos.POST("/:id/sync", middleware.RequirePermission("plugin:sync"), h.GitRepo.SyncRepo)
			gitRepos.POST("/:id/reset-status", middleware.RequirePermission("plugin:update"), h.GitRepo.ResetStatus)
			gitRepos.GET("/:id/logs", middleware.RequirePermission("plugin:list"), h.GitRepo.GetSyncLogs)
			gitRepos.GET("/:id/commits", middleware.RequirePermission("plugin:list"), h.GitRepo.GetCommits)
			gitRepos.GET("/:id/files", middleware.RequirePermission("plugin:list"), h.GitRepo.GetFiles)
			gitRepos.GET("/:id/branches", middleware.RequirePermission("plugin:list"), h.GitRepo.GetBranches)
			gitRepos.POST("/:id/detect-branches", middleware.RequirePermission("plugin:list"), h.GitRepo.DetectBranches)
		}

		// -------------------- Playbook 模板 --------------------
		playbooks := protected.Group("/playbooks")
		{
			playbooks.GET("", middleware.RequirePermission("plugin:list"), h.Playbook.List)
			playbooks.POST("", middleware.RequirePermission("plugin:create"), h.Playbook.Create)
			playbooks.GET("/stats", middleware.RequirePermission("plugin:list"), h.Playbook.GetStats)
			playbooks.GET("/:id", middleware.RequirePermission("plugin:list"), h.Playbook.Get)
			playbooks.PUT("/:id", middleware.RequirePermission("plugin:update"), h.Playbook.Update)
			playbooks.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.Playbook.Delete)
			playbooks.POST("/:id/scan", middleware.RequirePermission("plugin:update"), h.Playbook.ScanVariables)
			playbooks.PUT("/:id/variables", middleware.RequirePermission("plugin:update"), h.Playbook.UpdateVariables)
			playbooks.POST("/:id/ready", middleware.RequirePermission("plugin:update"), h.Playbook.SetReady)
			playbooks.POST("/:id/offline", middleware.RequirePermission("plugin:update"), h.Playbook.SetOffline)
			playbooks.GET("/:id/files", middleware.RequirePermission("plugin:list"), h.Playbook.GetFiles)
			playbooks.GET("/:id/scan-logs", middleware.RequirePermission("plugin:list"), h.Playbook.GetScanLogs)
		}

		// -------------------- 自愈流程 --------------------
		healingFlows := protected.Group("/healing/flows")
		{
			healingFlows.GET("/node-schema", middleware.RequirePermission("healing:flows:view"), h.Healing.GetNodeSchema)
			healingFlows.GET("", middleware.RequirePermission("healing:flows:view"), h.Healing.ListFlows)
			healingFlows.POST("", middleware.RequirePermission("healing:flows:create"), h.Healing.CreateFlow)
			healingFlows.GET("/stats", middleware.RequirePermission("healing:flows:view"), h.Healing.GetFlowStats)
			healingFlows.GET("/:id", middleware.RequirePermission("healing:flows:view"), h.Healing.GetFlow)
			healingFlows.PUT("/:id", middleware.RequirePermission("healing:flows:update"), h.Healing.UpdateFlow)
			healingFlows.DELETE("/:id", middleware.RequirePermission("healing:flows:delete"), h.Healing.DeleteFlow)
			healingFlows.POST("/:id/dry-run", middleware.RequirePermission("healing:flows:update"), h.Healing.DryRunFlow)
			healingFlows.POST("/:id/dry-run-stream", middleware.RequirePermission("healing:flows:update"), h.Healing.DryRunFlowStream) // SSE
		}

		// -------------------- 自愈规则 --------------------
		healingRules := protected.Group("/healing/rules")
		{
			healingRules.GET("", middleware.RequirePermission("healing:rules:view"), h.Healing.ListRules)
			healingRules.POST("", middleware.RequirePermission("healing:rules:create"), h.Healing.CreateRule)
			healingRules.GET("/stats", middleware.RequirePermission("healing:rules:view"), h.Healing.GetRuleStats)
			healingRules.GET("/:id", middleware.RequirePermission("healing:rules:view"), h.Healing.GetRule)
			healingRules.PUT("/:id", middleware.RequirePermission("healing:rules:update"), h.Healing.UpdateRule)
			healingRules.DELETE("/:id", middleware.RequirePermission("healing:rules:delete"), h.Healing.DeleteRule)
			healingRules.POST("/:id/activate", middleware.RequirePermission("healing:rules:update"), h.Healing.ActivateRule)
			healingRules.POST("/:id/deactivate", middleware.RequirePermission("healing:rules:update"), h.Healing.DeactivateRule)
		}

		// -------------------- 流程实例 --------------------
		flowInstances := protected.Group("/healing/instances")
		{
			flowInstances.GET("", middleware.RequirePermission("healing:instances:view"), h.Healing.ListInstances)
			flowInstances.GET("/stats", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstanceStats)
			flowInstances.GET("/:id", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstance)
			flowInstances.POST("/:id/cancel", middleware.RequirePermission("healing:instances:view"), h.Healing.CancelInstance)
			flowInstances.POST("/:id/retry", middleware.RequirePermission("healing:instances:view"), h.Healing.RetryInstance)
			flowInstances.GET("/:id/events", middleware.RequirePermission("healing:instances:view"), h.Healing.InstanceEvents) // SSE
		}

		// -------------------- 审批任务 --------------------
		approvals := protected.Group("/healing/approvals")
		{
			approvals.GET("", middleware.RequirePermission("healing:approvals:view"), h.Healing.ListApprovals)
			approvals.GET("/pending", middleware.RequirePermission("healing:approvals:view"), h.Healing.ListPendingApprovals)
			approvals.GET("/:id", middleware.RequirePermission("healing:approvals:view"), h.Healing.GetApproval)
			approvals.POST("/:id/approve", middleware.RequirePermission("healing:approvals:approve"), h.Healing.ApproveTask)
			approvals.POST("/:id/reject", middleware.RequirePermission("healing:approvals:approve"), h.Healing.RejectTask)
		}

		// -------------------- 待触发工单 (待办中心) --------------------
		pendingCenter := protected.Group("/healing/pending")
		{
			pendingCenter.GET("/trigger", middleware.RequirePermission("healing:trigger:view"), h.Healing.ListPendingTriggerIncidents)
			pendingCenter.GET("/dismissed", middleware.RequirePermission("healing:trigger:view"), h.Healing.ListDismissedTriggerIncidents)
		}

		// 工单手动触发（放在原有 incidents 组内更合理）
		incidents.POST("/:id/trigger", middleware.RequirePermission("healing:trigger:execute"), h.Healing.TriggerIncidentManually)
		incidents.POST("/:id/dismiss", middleware.RequirePermission("healing:trigger:execute"), h.Healing.DismissIncident)

		// -------------------- Dashboard --------------------
		dashboard := protected.Group("/dashboard")
		{
			dashboard.GET("/overview", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetOverview)
			dashboard.GET("/config", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetConfig)
			dashboard.PUT("/config", middleware.RequirePermission("dashboard:config:manage"), h.Dashboard.SaveConfig)

			// 系统工作区管理（需要管理员权限）
			dashboard.POST("/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.CreateSystemWorkspace)
			dashboard.GET("/workspaces", middleware.RequirePermission("dashboard:view"), h.Dashboard.ListSystemWorkspaces)
			dashboard.PUT("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.UpdateSystemWorkspace)
			dashboard.DELETE("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.DeleteSystemWorkspace)

			// 角色-工作区关联
			dashboard.GET("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetRoleWorkspaces)
			dashboard.PUT("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.AssignRoleWorkspaces)
		}

		// -------------------- Workbench (工作台) --------------------
		workbench := protected.Group("/workbench")
		{
			workbench.GET("/overview", h.Workbench.GetOverview)
			workbench.GET("/activities", middleware.RequirePermission("audit:list"), h.Workbench.GetActivities)
			workbench.GET("/schedule-calendar", middleware.RequirePermission("task:list"), h.Workbench.GetScheduleCalendar)
			workbench.GET("/announcements", h.Workbench.GetAnnouncements)
			workbench.GET("/favorites", h.Workbench.GetFavorites)
		}

		// -------------------- 站内信 --------------------
		siteMessages := protected.Group("/site-messages")
		{
			// 固定路径必须在 /:id 之前注册
			siteMessages.GET("/unread-count", h.SiteMessage.GetUnreadCount)
			siteMessages.GET("/categories", h.SiteMessage.GetCategories)
			siteMessages.GET("/settings", middleware.RequirePermission("site-message:settings:view"), h.SiteMessage.GetSettings)
			siteMessages.GET("/events", h.SiteMessage.Events) // SSE 实时推送
			siteMessages.PUT("/settings", middleware.RequirePermission("site-message:settings:manage"), h.SiteMessage.UpdateSettings)
			siteMessages.PUT("/read", h.SiteMessage.MarkRead)
			siteMessages.PUT("/read-all", h.SiteMessage.MarkAllRead)
			siteMessages.GET("", middleware.RequirePermission("site-message:list"), h.SiteMessage.ListMessages)
		}

		// ==================== 平台级管理接口（Platform） ====================
		// 所有平台级操作统一在 /api/v1/platform/ 下
		platform := protected.Group("/platform")
		{
			// ---- 平台级用户管理 ----
			platformUsers := platform.Group("/users")
			{
				platformUsers.GET("", middleware.RequirePermission("platform:users:list"), h.User.ListUsers)
				platformUsers.POST("", middleware.RequirePermission("platform:users:create"), h.User.CreateUser)
				platformUsers.GET("/simple", middleware.RequirePermission("platform:users:list"), h.User.ListSimpleUsers)
				platformUsers.GET("/:id", middleware.RequirePermission("platform:users:list"), h.User.GetUser)
				platformUsers.PUT("/:id", middleware.RequirePermission("platform:users:update"), h.User.UpdateUser)
				platformUsers.DELETE("/:id", middleware.RequirePermission("platform:users:delete"), h.User.DeleteUser)
				platformUsers.POST("/:id/reset-password", middleware.RequirePermission("platform:users:reset_password"), h.User.ResetPassword)
				platformUsers.PUT("/:id/roles", middleware.RequirePermission("platform:roles:manage"), h.User.AssignUserRoles)
			}

			// ---- 平台级角色管理 ----
			platformRoles := platform.Group("/roles")
			{
				platformRoles.GET("", middleware.RequirePermission("platform:roles:list"), h.Role.ListRoles)
				platformRoles.POST("", middleware.RequirePermission("platform:roles:manage"), h.Role.CreateRole)
				platformRoles.GET("/:id", middleware.RequirePermission("platform:roles:list"), h.Role.GetRole)
				platformRoles.PUT("/:id", middleware.RequirePermission("platform:roles:manage"), h.Role.UpdateRole)
				platformRoles.DELETE("/:id", middleware.RequirePermission("platform:roles:manage"), h.Role.DeleteRole)
				platformRoles.PUT("/:id/permissions", middleware.RequirePermission("platform:roles:manage"), h.Role.AssignRolePermissions)
				platformRoles.GET("/:id/users", middleware.RequirePermission("platform:roles:list"), h.Role.GetRoleUsers)
			}

			// ---- 平台级权限 ----
			platform.GET("/permissions", middleware.RequirePermission("platform:permissions:list"), h.Permission.ListPermissions)
			platform.GET("/permissions/tree", middleware.RequirePermission("platform:permissions:list"), h.Permission.GetPermissionTree)

			// ---- 平台设置 ----
			platform.GET("/settings", middleware.RequirePermission("platform:settings:manage"), h.PlatformSettings.ListSettings)
			platform.PUT("/settings/:key", middleware.RequirePermission("platform:settings:manage"), h.PlatformSettings.UpdateSetting)

			// ---- 租户管理 ----
			platform.GET("/tenants", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.ListTenants)
			platform.POST("/tenants", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.CreateTenant)
			platform.GET("/tenants/:id", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.GetTenant)
			platform.PUT("/tenants/:id", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.UpdateTenant)
			platform.DELETE("/tenants/:id", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.DeleteTenant)
			platform.POST("/tenants/:id/users", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.CreateTenantUser)
			platform.GET("/tenants/:id/members", middleware.RequireAnyPermission("platform:tenants:manage", "platform:tenants:list"), h.Tenant.ListMembers)
			platform.POST("/tenants/:id/admin", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.SetTenantAdmin)
			platform.PUT("/tenants/:id/members/:userId/role", middleware.RequirePermission("platform:tenants:manage"), h.Tenant.UpdateMemberRole)

			// ---- 平台级站内信管理 ----
			platformSiteMessages := platform.Group("/site-messages")
			{
				platformSiteMessages.POST("", middleware.RequirePermission("platform:messages:send"), h.SiteMessage.CreateMessage)
			}

			// ---- 平台级审计日志 ----
			platformAudit := platform.Group("/audit-logs")
			{
				platformAudit.GET("", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.ListPlatformAuditLogs)
				platformAudit.GET("/stats", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformAuditStats)
				platformAudit.GET("/trend", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformAuditTrend)
				platformAudit.GET("/user-ranking", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformUserRanking)
				platformAudit.GET("/high-risk", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformHighRiskLogs)
				platformAudit.GET("/:id", middleware.RequirePermission("platform:audit:list"), h.PlatformAudit.GetPlatformAuditLog)
			}
		}

		// -------------------- 用户租户 --------------------
		protected.GET("/user/tenants", h.Tenant.GetUserTenants)

		// -------------------- 字典值管理 --------------------
		dictionaries := protected.Group("/dictionaries")
		{
			dictionaries.GET("", h.Dictionary.ListDictionaries) // 批量查询
			dictionaries.GET("/types", h.Dictionary.ListTypes)  // 类型列表
			dictionaries.POST("", middleware.RequirePermission("platform:settings:manage"), h.Dictionary.CreateDictionary)
			dictionaries.PUT("/:id", middleware.RequirePermission("platform:settings:manage"), h.Dictionary.UpdateDictionary)
			dictionaries.DELETE("/:id", middleware.RequirePermission("platform:settings:manage"), h.Dictionary.DeleteDictionary)
		}
	}
}
