package handler

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handlers 所有处理器集合
type Handlers struct {
	Auth         *AuthHandler
	User         *UserHandler
	Role         *RoleHandler
	Permission   *PermissionHandler
	Plugin       *PluginHandler
	CMDB         *CMDBHandler
	Secrets      *SecretsHandler
	GitRepo      *GitRepoHandler
	Playbook     *PlaybookHandler
	Execution    *ExecutionHandler
	Schedule     *ScheduleHandler
	Notification *NotificationHandler
	Healing      *HealingHandler
	Dashboard    *DashboardHandler
}

// NewHandlers 创建所有处理器
func NewHandlers(cfg *config.Config) *Handlers {
	authHandler := NewAuthHandler(cfg)

	return &Handlers{
		Auth:         authHandler,
		User:         NewUserHandler(authHandler.authSvc),
		Role:         NewRoleHandler(),
		Permission:   NewPermissionHandler(),
		Plugin:       NewPluginHandler(),
		CMDB:         NewCMDBHandler(),
		Secrets:      NewSecretsHandler(),
		GitRepo:      NewGitRepoHandler(),
		Playbook:     NewPlaybookHandler(),
		Execution:    NewExecutionHandler(),
		Schedule:     NewScheduleHandler(),
		Notification: NewNotificationHandler(),
		Healing:      NewHealingHandler(),
		Dashboard:    NewDashboardHandler(),
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
	{
		// 用户认证相关
		protected.GET("/auth/me", h.Auth.GetCurrentUser)
		protected.GET("/auth/profile", h.Auth.GetProfile)
		protected.PUT("/auth/profile", h.Auth.UpdateProfile)
		protected.PUT("/auth/password", h.Auth.ChangePassword)
		protected.POST("/auth/logout", h.Auth.Logout)

		// -------------------- 用户管理 --------------------
		users := protected.Group("/users")
		{
			users.GET("", middleware.RequirePermission("user:list"), h.User.ListUsers)
			users.POST("", middleware.RequirePermission("user:create"), h.User.CreateUser)
			users.GET("/:id", middleware.RequirePermission("user:list"), h.User.GetUser)
			users.PUT("/:id", middleware.RequirePermission("user:update"), h.User.UpdateUser)
			users.DELETE("/:id", middleware.RequirePermission("user:delete"), h.User.DeleteUser)
			users.POST("/:id/reset-password", middleware.RequirePermission("user:reset_password"), h.User.ResetPassword)
			users.PUT("/:id/roles", middleware.RequirePermission("role:assign"), h.User.AssignUserRoles)
		}

		// -------------------- 角色管理 --------------------
		roles := protected.Group("/roles")
		{
			roles.GET("", middleware.RequirePermission("role:list"), h.Role.ListRoles)
			roles.POST("", middleware.RequirePermission("role:create"), h.Role.CreateRole)
			roles.GET("/:id", middleware.RequirePermission("role:list"), h.Role.GetRole)
			roles.PUT("/:id", middleware.RequirePermission("role:update"), h.Role.UpdateRole)
			roles.DELETE("/:id", middleware.RequirePermission("role:delete"), h.Role.DeleteRole)
			roles.PUT("/:id/permissions", middleware.RequirePermission("role:assign"), h.Role.AssignRolePermissions)
		}

		// -------------------- 权限 --------------------
		protected.GET("/permissions", h.Permission.ListPermissions)
		protected.GET("/permissions/tree", h.Permission.GetPermissionTree)

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

		// -------------------- 工作流管理 (待实现) --------------------
		// workflows := protected.Group("/workflows")
		// {
		// 	workflows.GET("", middleware.RequirePermission("workflow:list"), ListWorkflows)
		// 	workflows.POST("", middleware.RequirePermission("workflow:create"), CreateWorkflow)
		// 	workflows.GET("/:id", middleware.RequirePermission("workflow:detail"), GetWorkflow)
		// 	workflows.PUT("/:id", middleware.RequirePermission("workflow:update"), UpdateWorkflow)
		// 	workflows.DELETE("/:id", middleware.RequirePermission("workflow:delete"), DeleteWorkflow)
		// 	workflows.POST("/:id/activate", middleware.RequirePermission("workflow:activate"), ActivateWorkflow)
		// 	workflows.POST("/:id/deactivate", middleware.RequirePermission("workflow:activate"), DeactivateWorkflow)
		// 	workflows.POST("/:id/clone", middleware.RequirePermission("workflow:create"), CloneWorkflow)
		// 	workflows.POST("/:id/run", middleware.RequirePermission("workflow:run"), RunWorkflow)
		// 	workflows.POST("/:id/nodes", middleware.RequirePermission("workflow:update"), CreateNode)
		// 	workflows.POST("/:id/edges", middleware.RequirePermission("workflow:update"), CreateEdge)
		// }

		// protected.PUT("/nodes/:id", middleware.RequirePermission("workflow:update"), UpdateNode)
		// protected.DELETE("/nodes/:id", middleware.RequirePermission("workflow:update"), DeleteNode)
		// protected.DELETE("/edges/:id", middleware.RequirePermission("workflow:update"), DeleteEdge)

		// -------------------- 工作流实例 (待实现) --------------------
		// instances := protected.Group("/instances")
		// {
		// 	instances.GET("", middleware.RequirePermission("workflow:list"), ListInstances)
		// 	instances.GET("/:id", middleware.RequirePermission("workflow:list"), GetInstance)
		// 	instances.POST("/:id/pause", middleware.RequirePermission("workflow:run"), PauseInstance)
		// 	instances.POST("/:id/resume", middleware.RequirePermission("workflow:run"), ResumeInstance)
		// 	instances.POST("/:id/cancel", middleware.RequirePermission("workflow:run"), CancelInstance)
		// 	instances.POST("/:id/retry", middleware.RequirePermission("workflow:run"), RetryInstance)
		// 	instances.GET("/:id/logs", middleware.RequirePermission("workflow:list"), GetInstanceLogs)
		// 	instances.GET("/:id/logs/stream", middleware.RequirePermission("workflow:list"), StreamInstanceLogs)
		// }

		// -------------------- Git 仓库管理 (待实现, 使用 git-repos 替代) --------------------
		// repositories := protected.Group("/repositories")
		// {
		// 	repositories.GET("", middleware.RequirePermission("repository:list"), ListRepositories)
		// 	repositories.POST("", middleware.RequirePermission("repository:create"), CreateRepository)
		// 	repositories.GET("/:id", middleware.RequirePermission("repository:list"), GetRepository)
		// 	repositories.PUT("/:id", middleware.RequirePermission("repository:update"), UpdateRepository)
		// 	repositories.DELETE("/:id", middleware.RequirePermission("repository:delete"), DeleteRepository)
		// 	repositories.POST("/:id/sync", middleware.RequirePermission("repository:sync"), SyncRepository)
		// 	repositories.POST("/:id/scan", middleware.RequirePermission("repository:sync"), ScanPlaybooks)
		// }

		// -------------------- Playbook 管理 (待实现) --------------------
		// playbooks := protected.Group("/playbooks")
		// {
		// 	playbooks.GET("", middleware.RequirePermission("playbook:list"), ListPlaybooks)
		// 	playbooks.GET("/:id", middleware.RequirePermission("playbook:list"), GetPlaybook)
		// 	playbooks.PUT("/:id", middleware.RequirePermission("playbook:list"), UpdatePlaybook)
		// 	playbooks.GET("/:id/content", middleware.RequirePermission("playbook:list"), GetPlaybookContent)
		// }

		// -------------------- 执行任务模板 --------------------
		execTasks := protected.Group("/execution-tasks")
		{
			execTasks.GET("", middleware.RequirePermission("task:list"), h.Execution.ListTasks)
			execTasks.POST("", middleware.RequirePermission("playbook:execute"), h.Execution.CreateTask)
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
		protected.GET("/template-variables", h.Notification.GetAvailableVariables)

		// -------------------- 通知发送 --------------------
		notifications := protected.Group("/notifications")
		{
			notifications.POST("/send", middleware.RequirePermission("notification:send"), h.Notification.SendNotification)
			notifications.GET("", middleware.RequirePermission("notification:list"), h.Notification.ListNotifications)
			notifications.GET("/:id", middleware.RequirePermission("notification:list"), h.Notification.GetNotification)
		}

		// -------------------- 审计日志 (待实现) --------------------
		// auditLogs := protected.Group("/audit-logs")
		// {
		// 	auditLogs.GET("", middleware.RequirePermission("audit:list"), ListAuditLogs)
		// 	auditLogs.GET("/:id", middleware.RequirePermission("audit:list"), GetAuditLog)
		// 	auditLogs.GET("/export", middleware.RequirePermission("audit:export"), ExportAuditLogs)
		// }

		// -------------------- 工单/事件 --------------------
		incidents := protected.Group("/incidents")
		{
			incidents.GET("/stats", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncidentStats)
			incidents.GET("", middleware.RequirePermission("plugin:list"), h.Plugin.ListIncidents)
			incidents.GET("/:id", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncident)
			incidents.POST("/:id/reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.ResetIncidentScan)
			incidents.POST("/batch-reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.BatchResetIncidentScan)
		}

		// -------------------- CMDB 配置项 --------------------
		cmdb := protected.Group("/cmdb")
		{
			cmdb.GET("", middleware.RequirePermission("plugin:list"), h.CMDB.ListCMDBItems)
			cmdb.GET("/stats", middleware.RequirePermission("plugin:list"), h.CMDB.GetCMDBStats)
			cmdb.POST("/batch-test-connection", middleware.RequirePermission("plugin:sync"), h.CMDB.BatchTestConnection)
			cmdb.POST("/batch/maintenance", middleware.RequirePermission("plugin:update"), h.CMDB.BatchEnterMaintenance)
			cmdb.POST("/batch/resume", middleware.RequirePermission("plugin:update"), h.CMDB.BatchExitMaintenance)
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
			gitRepos.GET("/:id", middleware.RequirePermission("plugin:list"), h.GitRepo.GetRepo)
			gitRepos.PUT("/:id", middleware.RequirePermission("plugin:update"), h.GitRepo.UpdateRepo)
			gitRepos.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.GitRepo.DeleteRepo)
			gitRepos.POST("/:id/sync", middleware.RequirePermission("plugin:sync"), h.GitRepo.SyncRepo)
			gitRepos.POST("/:id/reset-status", middleware.RequirePermission("plugin:update"), h.GitRepo.ResetStatus)
			gitRepos.GET("/:id/logs", middleware.RequirePermission("plugin:list"), h.GitRepo.GetSyncLogs)
			gitRepos.GET("/:id/commits", middleware.RequirePermission("plugin:list"), h.GitRepo.GetCommits)
			gitRepos.GET("/:id/files", middleware.RequirePermission("plugin:list"), h.GitRepo.GetFiles)
		}

		// -------------------- Playbook 模板 --------------------
		playbooks := protected.Group("/playbooks")
		{
			playbooks.GET("", middleware.RequirePermission("plugin:list"), h.Playbook.List)
			playbooks.POST("", middleware.RequirePermission("plugin:create"), h.Playbook.Create)
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
		}

		// 工单手动触发（放在原有 incidents 组内更合理）
		incidents.POST("/:id/trigger", middleware.RequirePermission("healing:trigger:execute"), h.Healing.TriggerIncidentManually)

		// -------------------- Dashboard --------------------
		dashboard := protected.Group("/dashboard")
		{
			dashboard.GET("/overview", h.Dashboard.GetOverview)
			dashboard.GET("/config", h.Dashboard.GetConfig)
			dashboard.PUT("/config", h.Dashboard.SaveConfig)

			// 系统工作区管理（需要管理员权限）
			dashboard.POST("/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.CreateSystemWorkspace)
			dashboard.GET("/workspaces", h.Dashboard.ListSystemWorkspaces)
			dashboard.PUT("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.UpdateSystemWorkspace)
			dashboard.DELETE("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.DeleteSystemWorkspace)

			// 角色-工作区关联
			dashboard.GET("/roles/:roleId/workspaces", h.Dashboard.GetRoleWorkspaces)
			dashboard.PUT("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.AssignRoleWorkspaces)
		}
	}
}
