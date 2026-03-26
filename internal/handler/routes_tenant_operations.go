package handler

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerTenantOperationalRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantIncidentRoutes(tenant.Group("/incidents"), h)
	registerTenantCMDBRoutes(tenant.Group("/cmdb"), h)
	registerTenantSecretsRoutes(tenant, tenant.Group("/secrets-sources"), h)
	registerTenantGitRoutes(tenant.Group("/git-repos"), h)
	registerTenantPlaybookRoutes(tenant.Group("/playbooks"), h)
}

func registerTenantIncidentRoutes(incidents *gin.RouterGroup, h *Handlers) {
	incidents.GET("/stats", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncidentStats)
	incidents.GET("/search-schema", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncidentSearchSchema)
	incidents.GET("", middleware.RequirePermission("plugin:list"), h.Plugin.ListIncidents)
	incidents.POST("/batch-reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.BatchResetIncidentScan)
	incidents.GET("/:id", middleware.RequirePermission("plugin:list"), h.Plugin.GetIncident)
	incidents.POST("/:id/reset-scan", middleware.RequirePermission("plugin:sync"), h.Plugin.ResetIncidentScan)
	incidents.POST("/:id/close", middleware.RequirePermission("plugin:sync"), h.Plugin.CloseIncident)
	incidents.POST("/:id/trigger", middleware.RequirePermission("healing:trigger:execute"), h.Healing.TriggerIncidentManually)
	incidents.POST("/:id/dismiss", middleware.RequirePermission("healing:trigger:execute"), h.Healing.DismissIncident)
}

func registerTenantCMDBRoutes(cmdb *gin.RouterGroup, h *Handlers) {
	cmdb.GET("", middleware.RequirePermission("plugin:list"), h.CMDB.ListCMDBItems)
	cmdb.GET("/stats", middleware.RequirePermission("plugin:list"), h.CMDB.GetCMDBStats)
	cmdb.GET("/search-schema", middleware.RequirePermission("plugin:list"), h.CMDB.GetCMDBSearchSchema)
	cmdb.POST("/batch-test-connection", middleware.RequirePermission("plugin:sync"), h.CMDB.BatchTestConnection)
	cmdb.POST("/batch/maintenance", middleware.RequirePermission("plugin:update"), h.CMDB.BatchEnterMaintenance)
	cmdb.POST("/batch/resume", middleware.RequirePermission("plugin:update"), h.CMDB.BatchExitMaintenance)
	cmdb.GET("/ids", middleware.RequirePermission("plugin:list"), h.CMDB.ListCMDBItemIDs)
	cmdb.GET("/:id", middleware.RequirePermission("plugin:list"), h.CMDB.GetCMDBItem)
	cmdb.POST("/:id/test-connection", middleware.RequirePermission("plugin:sync"), h.CMDB.TestConnection)
	cmdb.POST("/:id/maintenance", middleware.RequirePermission("plugin:update"), h.CMDB.EnterMaintenance)
	cmdb.POST("/:id/resume", middleware.RequirePermission("plugin:update"), h.CMDB.ExitMaintenance)
	cmdb.GET("/:id/maintenance-logs", middleware.RequirePermission("plugin:list"), h.CMDB.GetMaintenanceLogs)
}

func registerTenantSecretsRoutes(tenant *gin.RouterGroup, sources *gin.RouterGroup, h *Handlers) {
	sources.GET("", middleware.RequirePermission("plugin:list"), h.Secrets.ListSources)
	sources.POST("", middleware.RequirePermission("plugin:create"), h.Secrets.CreateSource)
	sources.GET("/stats", middleware.RequirePermission("plugin:list"), h.Secrets.GetStats)
	sources.GET("/:id", middleware.RequirePermission("plugin:list"), h.Secrets.GetSource)
	sources.PUT("/:id", middleware.RequirePermission("plugin:update"), h.Secrets.UpdateSource)
	sources.DELETE("/:id", middleware.RequirePermission("plugin:delete"), h.Secrets.DeleteSource)
	sources.POST("/:id/test", middleware.RequirePermission("plugin:test"), h.Secrets.TestConnection)
	sources.POST("/:id/test-query", middleware.RequirePermission("plugin:test"), h.Secrets.TestQuery)
	sources.POST("/:id/enable", middleware.RequirePermission("plugin:update"), h.Secrets.Enable)
	sources.POST("/:id/disable", middleware.RequirePermission("plugin:update"), h.Secrets.Disable)

	tenant.POST("/secrets/query", middleware.RequirePermission("secrets:query"), h.Secrets.QuerySecret)
}

func registerTenantGitRoutes(repos *gin.RouterGroup, h *Handlers) {
	repos.POST("/validate", middleware.RequirePermission("repository:validate"), h.GitRepo.ValidateRepo)
	repos.GET("", middleware.RequirePermission("repository:list"), h.GitRepo.ListRepos)
	repos.POST("", middleware.RequireAllPermissions("repository:create", "repository:validate"), h.GitRepo.CreateRepo)
	repos.GET("/stats", middleware.RequirePermission("repository:list"), h.GitRepo.GetStats)
	repos.GET("/search-schema", middleware.RequirePermission("repository:list"), h.GitRepo.GetSearchSchema)
	repos.GET("/:id", middleware.RequirePermission("repository:list"), h.GitRepo.GetRepo)
	repos.PUT("/:id", middleware.RequirePermission("repository:update"), h.GitRepo.UpdateRepo)
	repos.DELETE("/:id", middleware.RequirePermission("repository:delete"), h.GitRepo.DeleteRepo)
	repos.POST("/:id/sync", middleware.RequirePermission("repository:sync"), h.GitRepo.SyncRepo)
	repos.POST("/:id/reset-status", middleware.RequirePermission("repository:update"), h.GitRepo.ResetStatus)
	repos.GET("/:id/logs", middleware.RequirePermission("repository:list"), h.GitRepo.GetSyncLogs)
	repos.GET("/:id/commits", middleware.RequirePermission("repository:list"), h.GitRepo.GetCommits)
	repos.GET("/:id/files", middleware.RequirePermission("repository:list"), h.GitRepo.GetFiles)
}

func registerTenantPlaybookRoutes(playbooks *gin.RouterGroup, h *Handlers) {
	playbooks.GET("", middleware.RequirePermission("playbook:list"), h.Playbook.List)
	playbooks.POST("", middleware.RequirePermission("playbook:create"), h.Playbook.Create)
	playbooks.GET("/stats", middleware.RequirePermission("playbook:list"), h.Playbook.GetStats)
	playbooks.GET("/:id", middleware.RequirePermission("playbook:list"), h.Playbook.Get)
	playbooks.PUT("/:id", middleware.RequirePermission("playbook:update"), h.Playbook.Update)
	playbooks.DELETE("/:id", middleware.RequirePermission("playbook:delete"), h.Playbook.Delete)
	playbooks.POST("/:id/scan", middleware.RequirePermission("playbook:update"), h.Playbook.ScanVariables)
	playbooks.PUT("/:id/variables", middleware.RequirePermission("playbook:update"), h.Playbook.UpdateVariables)
	playbooks.POST("/:id/ready", middleware.RequirePermission("playbook:update"), h.Playbook.SetReady)
	playbooks.POST("/:id/offline", middleware.RequirePermission("playbook:update"), h.Playbook.SetOffline)
	playbooks.GET("/:id/files", middleware.RequirePermission("playbook:list"), h.Playbook.GetFiles)
	playbooks.GET("/:id/scan-logs", middleware.RequirePermission("playbook:list"), h.Playbook.GetScanLogs)
}

func registerTenantAutomationRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantHealingRoutes(tenant, h)
	registerTenantDashboardRoutes(tenant.Group("/dashboard"), h)
}

func registerTenantHealingRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantHealingFlowRoutes(tenant.Group("/healing/flows"), h)
	registerTenantHealingRuleRoutes(tenant.Group("/healing/rules"), h)
	registerTenantHealingInstanceRoutes(tenant.Group("/healing/instances"), h)
	registerTenantHealingApprovalRoutes(tenant.Group("/healing/approvals"), h)
	registerTenantHealingPendingRoutes(tenant.Group("/healing/pending"), h)
}

func registerTenantHealingFlowRoutes(flows *gin.RouterGroup, h *Handlers) {
	flows.GET("/node-schema", middleware.RequirePermission("healing:flows:view"), h.Healing.GetNodeSchema)
	flows.GET("/search-schema", middleware.RequirePermission("healing:flows:view"), h.Healing.GetFlowSearchSchema)
	flows.GET("", middleware.RequirePermission("healing:flows:view"), h.Healing.ListFlows)
	flows.POST("", middleware.RequirePermission("healing:flows:create"), h.Healing.CreateFlow)
	flows.GET("/stats", middleware.RequirePermission("healing:flows:view"), h.Healing.GetFlowStats)
	flows.GET("/:id", middleware.RequirePermission("healing:flows:view"), h.Healing.GetFlow)
	flows.PUT("/:id", middleware.RequirePermission("healing:flows:update"), h.Healing.UpdateFlow)
	flows.DELETE("/:id", middleware.RequirePermission("healing:flows:delete"), h.Healing.DeleteFlow)
	flows.POST("/:id/dry-run", middleware.RequirePermission("healing:flows:update"), h.Healing.DryRunFlow)
	flows.POST("/:id/dry-run-stream", middleware.RequirePermission("healing:flows:update"), h.Healing.DryRunFlowStream)
}

func registerTenantHealingRuleRoutes(rules *gin.RouterGroup, h *Handlers) {
	rules.GET("/search-schema", middleware.RequirePermission("healing:rules:view"), h.Healing.GetRuleSearchSchema)
	rules.GET("", middleware.RequirePermission("healing:rules:view"), h.Healing.ListRules)
	rules.POST("", middleware.RequirePermission("healing:rules:create"), h.Healing.CreateRule)
	rules.GET("/stats", middleware.RequirePermission("healing:rules:view"), h.Healing.GetRuleStats)
	rules.GET("/:id", middleware.RequirePermission("healing:rules:view"), h.Healing.GetRule)
	rules.PUT("/:id", middleware.RequirePermission("healing:rules:update"), h.Healing.UpdateRule)
	rules.DELETE("/:id", middleware.RequirePermission("healing:rules:delete"), h.Healing.DeleteRule)
	rules.POST("/:id/activate", middleware.RequirePermission("healing:rules:update"), h.Healing.ActivateRule)
	rules.POST("/:id/deactivate", middleware.RequirePermission("healing:rules:update"), h.Healing.DeactivateRule)
}

func registerTenantHealingInstanceRoutes(instances *gin.RouterGroup, h *Handlers) {
	instances.GET("/search-schema", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstanceSearchSchema)
	instances.GET("", middleware.RequirePermission("healing:instances:view"), h.Healing.ListInstances)
	instances.GET("/stats", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstanceStats)
	instances.GET("/:id", middleware.RequirePermission("healing:instances:view"), h.Healing.GetInstance)
	instances.POST("/:id/cancel", middleware.RequirePermission("healing:flows:update"), h.Healing.CancelInstance)
	instances.POST("/:id/retry", middleware.RequirePermission("healing:flows:update"), h.Healing.RetryInstance)
	instances.GET("/:id/events", middleware.RequirePermission("healing:instances:view"), h.Healing.InstanceEvents)
}

func registerTenantHealingApprovalRoutes(approvals *gin.RouterGroup, h *Handlers) {
	approvals.GET("", middleware.RequirePermission("healing:approvals:view"), h.Healing.ListApprovals)
	approvals.GET("/pending", middleware.RequirePermission("healing:approvals:view"), h.Healing.ListPendingApprovals)
	approvals.GET("/:id", middleware.RequirePermission("healing:approvals:view"), h.Healing.GetApproval)
	approvals.POST("/:id/approve", middleware.RequirePermission("healing:approvals:approve"), h.Healing.ApproveTask)
	approvals.POST("/:id/reject", middleware.RequirePermission("healing:approvals:approve"), h.Healing.RejectTask)
}

func registerTenantHealingPendingRoutes(pending *gin.RouterGroup, h *Handlers) {
	pending.GET("/trigger", middleware.RequirePermission("healing:trigger:view"), h.Healing.ListPendingTriggerIncidents)
	pending.GET("/dismissed", middleware.RequirePermission("healing:trigger:view"), h.Healing.ListDismissedTriggerIncidents)
}

func registerTenantDashboardRoutes(dashboard *gin.RouterGroup, h *Handlers) {
	dashboard.GET("/overview", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetOverview)
	dashboard.GET("/config", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetConfig)
	dashboard.PUT("/config", middleware.RequirePermission("dashboard:config:manage"), h.Dashboard.SaveConfig)
	dashboard.POST("/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.CreateSystemWorkspace)
	dashboard.GET("/workspaces", middleware.RequirePermission("dashboard:view"), h.Dashboard.ListSystemWorkspaces)
	dashboard.PUT("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.UpdateSystemWorkspace)
	dashboard.DELETE("/workspaces/:id", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.DeleteSystemWorkspace)
	dashboard.GET("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:view"), h.Dashboard.GetRoleWorkspaces)
	dashboard.PUT("/roles/:roleId/workspaces", middleware.RequirePermission("dashboard:workspace:manage"), h.Dashboard.AssignRoleWorkspaces)
}

func registerTenantExperienceRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantSiteMessageRoutes(tenant.Group("/site-messages"), h)
	registerTenantImpersonationApprovalRoutes(tenant.Group("/impersonation"), h)
	registerTenantSettingsRoutes(tenant.Group("/settings"), h)
}

func registerTenantSiteMessageRoutes(siteMessages *gin.RouterGroup, h *Handlers) {
	siteMessages.GET("/unread-count", h.SiteMessage.GetUnreadCount)
	siteMessages.GET("/events", h.SiteMessage.Events)
	siteMessages.PUT("/read", h.SiteMessage.MarkRead)
	siteMessages.PUT("/read-all", h.SiteMessage.MarkAllRead)
	siteMessages.GET("", middleware.RequirePermission("site-message:list"), h.SiteMessage.ListMessages)
}

func registerTenantImpersonationApprovalRoutes(impersonation *gin.RouterGroup, h *Handlers) {
	impersonation.GET("/pending", middleware.RequirePermission("tenant:impersonation:view"), h.Impersonation.ListPending)
	impersonation.GET("/history", middleware.RequirePermission("tenant:impersonation:view"), h.Impersonation.ListHistory)
	impersonation.POST("/:id/approve", middleware.RequirePermission("tenant:impersonation:approve"), h.Impersonation.Approve)
	impersonation.POST("/:id/reject", middleware.RequirePermission("tenant:impersonation:approve"), h.Impersonation.Reject)
}

func registerTenantSettingsRoutes(settings *gin.RouterGroup, h *Handlers) {
	settings.GET("/impersonation-approvers", middleware.RequirePermission("tenant:impersonation:approve"), h.Impersonation.GetApprovers)
	settings.PUT("/impersonation-approvers", middleware.RequirePermission("tenant:impersonation:approve"), h.Impersonation.SetApprovers)
}

func registerTenantSecurityRoutes(tenant *gin.RouterGroup, h *Handlers) {
	registerTenantBlacklistRoutes(tenant.Group("/command-blacklist"), h)
	registerTenantExemptionRoutes(tenant.Group("/blacklist-exemptions"), h)
}

func registerTenantBlacklistRoutes(blacklist *gin.RouterGroup, h *Handlers) {
	blacklist.GET("", middleware.RequirePermission("security:blacklist:view"), h.CommandBlacklist.List)
	blacklist.GET("/search-schema", middleware.RequirePermission("security:blacklist:view"), h.CommandBlacklist.GetSearchSchema)
	blacklist.POST("", middleware.RequirePermission("security:blacklist:create"), h.CommandBlacklist.Create)
	blacklist.POST("/batch-toggle", middleware.RequirePermission("security:blacklist:update"), h.CommandBlacklist.BatchToggle)
	blacklist.POST("/simulate", middleware.RequirePermission("security:blacklist:view"), h.CommandBlacklist.Simulate)
	blacklist.GET("/:id", middleware.RequirePermission("security:blacklist:view"), h.CommandBlacklist.Get)
	blacklist.PUT("/:id", middleware.RequirePermission("security:blacklist:update"), h.CommandBlacklist.Update)
	blacklist.DELETE("/:id", middleware.RequirePermission("security:blacklist:delete"), h.CommandBlacklist.Delete)
	blacklist.POST("/:id/toggle", middleware.RequirePermission("security:blacklist:update"), h.CommandBlacklist.ToggleActive)
}

func registerTenantExemptionRoutes(exemptions *gin.RouterGroup, h *Handlers) {
	exemptions.GET("", middleware.RequirePermission("security:exemption:view"), h.BlacklistExemption.List)
	exemptions.GET("/search-schema", middleware.RequirePermission("security:exemption:view"), h.BlacklistExemption.GetSearchSchema)
	exemptions.GET("/pending", middleware.RequirePermission("security:exemption:approve"), h.BlacklistExemption.GetPending)
	exemptions.POST("", middleware.RequirePermission("security:exemption:create"), h.BlacklistExemption.Create)
	exemptions.GET("/:id", middleware.RequirePermission("security:exemption:view"), h.BlacklistExemption.Get)
	exemptions.POST("/:id/approve", middleware.RequirePermission("security:exemption:approve"), h.BlacklistExemption.Approve)
	exemptions.POST("/:id/reject", middleware.RequirePermission("security:exemption:approve"), h.BlacklistExemption.Reject)
}
