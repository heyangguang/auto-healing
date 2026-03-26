package handler

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantStatsItem 单个租户的统计数据
type TenantStatsItem struct {
	ID                        uuid.UUID `json:"id"`
	Name                      string    `json:"name"`
	Code                      string    `json:"code"`
	Status                    string    `json:"status"`
	Icon                      string    `json:"icon"`
	MemberCount               int64     `json:"member_count"`
	RuleCount                 int64     `json:"rule_count"`
	InstanceCount             int64     `json:"instance_count"`
	TemplateCount             int64     `json:"template_count"`
	AuditLogCount             int64     `json:"audit_log_count"`
	LastActivityAt            *string   `json:"last_activity_at"`
	CmdbCount                 int64     `json:"cmdb_count"`
	GitCount                  int64     `json:"git_count"`
	PlaybookCount             int64     `json:"playbook_count"`
	SecretCount               int64     `json:"secret_count"`
	PluginCount               int64     `json:"plugin_count"`
	IncidentCount             int64     `json:"incident_count"`
	FlowCount                 int64     `json:"flow_count"`
	ScheduleCount             int64     `json:"schedule_count"`
	NotificationChannelCount  int64     `json:"notification_channel_count"`
	NotificationTemplateCount int64     `json:"notification_template_count"`
	HealingSuccessCount       int64     `json:"healing_success_count"`
	HealingTotalCount         int64     `json:"healing_total_count"`
	IncidentCoveredCount      int64     `json:"incident_covered_count"`
}

// TenantStatsSummary 聚合统计总览
type TenantStatsSummary struct {
	TotalTenants    int64 `json:"total_tenants"`
	ActiveTenants   int64 `json:"active_tenants"`
	DisabledTenants int64 `json:"disabled_tenants"`
	TotalUsers      int64 `json:"total_users"`
	TotalRules      int64 `json:"total_rules"`
	TotalInstances  int64 `json:"total_instances"`
	TotalTemplates  int64 `json:"total_templates"`
}

// GetTenantStats 获取租户运营总览统计
func (h *TenantHandler) GetTenantStats(c *gin.Context) {
	ctx := c.Request.Context()
	tenants, _, err := h.repo.List(ctx, "", query.StringFilter{}, query.StringFilter{}, "", 1, 1000)
	if err != nil {
		response.InternalError(c, "查询租户列表失败")
		return
	}

	stats := make([]TenantStatsItem, 0, len(tenants))
	summary := TenantStatsSummary{TotalTenants: int64(len(tenants))}
	for _, tenant := range tenants {
		item := h.buildTenantStatsItem(ctx, tenant)
		updateTenantStatsSummary(&summary, tenant, item)
		stats = append(stats, item)
	}

	response.Success(c, gin.H{
		"tenants": stats,
		"summary": summary,
	})
}

func (h *TenantHandler) buildTenantStatsItem(ctx context.Context, tenant model.Tenant) TenantStatsItem {
	item := TenantStatsItem{
		ID:     tenant.ID,
		Name:   tenant.Name,
		Code:   tenant.Code,
		Status: tenant.Status,
		Icon:   tenant.Icon,
	}
	item.MemberCount = h.repo.CountTenantMembers(ctx, tenant.ID)
	item.RuleCount = h.repo.CountTenantTable(ctx, tenant.ID, "healing_rules")
	item.InstanceCount = h.repo.CountTenantTable(ctx, tenant.ID, "flow_instances")
	item.TemplateCount = h.repo.CountTenantTable(ctx, tenant.ID, "execution_tasks")
	item.AuditLogCount = h.repo.CountTenantTable(ctx, tenant.ID, "audit_logs")
	item.LastActivityAt = h.repo.GetTenantLastActivity(ctx, tenant.ID)
	item.CmdbCount = h.repo.CountTenantTable(ctx, tenant.ID, "cmdb_items")
	item.GitCount = h.repo.CountTenantTable(ctx, tenant.ID, "git_repositories")
	item.PlaybookCount = h.repo.CountTenantTable(ctx, tenant.ID, "playbooks")
	item.SecretCount = h.repo.CountTenantTable(ctx, tenant.ID, "secrets_sources")
	item.PluginCount = h.repo.CountTenantTable(ctx, tenant.ID, "plugins")
	item.IncidentCount = h.repo.CountTenantTable(ctx, tenant.ID, "incidents")
	item.FlowCount = h.repo.CountTenantTable(ctx, tenant.ID, "healing_flows")
	item.ScheduleCount = h.repo.CountTenantTable(ctx, tenant.ID, "execution_schedules")
	item.NotificationChannelCount = h.repo.CountTenantTable(ctx, tenant.ID, "notification_channels")
	item.NotificationTemplateCount = h.repo.CountTenantTable(ctx, tenant.ID, "notification_templates")
	item.HealingSuccessCount = h.repo.CountTenantTableWhere(ctx, tenant.ID, "flow_instances", "status = 'completed'")
	item.HealingTotalCount = h.repo.CountTenantTable(ctx, tenant.ID, "flow_instances")
	item.IncidentCoveredCount = h.repo.CountTenantTableWhere(ctx, tenant.ID, "incidents", "matched_rule_id IS NOT NULL")
	return item
}

func updateTenantStatsSummary(summary *TenantStatsSummary, tenant model.Tenant, item TenantStatsItem) {
	if tenant.Status == model.TenantStatusActive {
		summary.ActiveTenants++
	} else {
		summary.DisabledTenants++
	}
	summary.TotalUsers += item.MemberCount
	summary.TotalRules += item.RuleCount
	summary.TotalInstances += item.InstanceCount
	summary.TotalTemplates += item.TemplateCount
}

// GetTenantTrends 获取平台运营趋势数据
func (h *TenantHandler) GetTenantTrends(c *gin.Context) {
	ctx := c.Request.Context()
	days := parsePositiveIntQuery(c, "days", 7, 90)

	opDates, opCounts, err := h.repo.GetTrendByDay(ctx, "audit_logs", days)
	if err != nil {
		response.InternalError(c, "查询操作趋势失败")
		return
	}
	_, auditCounts, err := h.repo.GetTrendByDayWhere(ctx, "audit_logs", days,
		"action IN ('login','logout','impersonation_enter','impersonation_exit','impersonation_terminate','approve')")
	if err != nil {
		response.InternalError(c, "查询审计趋势失败")
		return
	}
	_, taskCounts, err := h.repo.GetTrendByDay(ctx, "execution_runs", days)
	if err != nil {
		response.InternalError(c, "查询任务执行趋势失败")
		return
	}

	response.Success(c, gin.H{
		"dates":           opDates,
		"operations":      opCounts,
		"audit_logs":      auditCounts,
		"task_executions": taskCounts,
	})
}
