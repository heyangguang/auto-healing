package httpapi

import (
	"context"

	"github.com/company/auto-healing/internal/modules/access/model"
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
		item, err := h.buildTenantStatsItem(ctx, tenant)
		if err != nil {
			respondInternalError(c, "TENANT", "查询租户统计失败", err)
			return
		}
		updateTenantStatsSummary(&summary, tenant, item)
		stats = append(stats, item)
	}

	response.Success(c, gin.H{
		"tenants": stats,
		"summary": summary,
	})
}

func (h *TenantHandler) buildTenantStatsItem(ctx context.Context, tenant model.Tenant) (TenantStatsItem, error) {
	item := TenantStatsItem{
		ID:     tenant.ID,
		Name:   tenant.Name,
		Code:   tenant.Code,
		Status: tenant.Status,
		Icon:   tenant.Icon,
	}
	if err := h.fillTenantStatsCore(ctx, tenant.ID, &item); err != nil {
		return TenantStatsItem{}, err
	}
	if err := h.fillTenantStatsResources(ctx, tenant.ID, &item); err != nil {
		return TenantStatsItem{}, err
	}
	if err := h.fillTenantStatsDerived(ctx, tenant.ID, &item); err != nil {
		return TenantStatsItem{}, err
	}
	return item, nil
}

func (h *TenantHandler) fillTenantStatsCore(ctx context.Context, tenantID uuid.UUID, item *TenantStatsItem) error {
	memberCount, err := h.repo.CountTenantMembers(ctx, tenantID)
	if err != nil {
		return err
	}
	ruleCount, err := h.repo.CountTenantTable(ctx, tenantID, "healing_rules")
	if err != nil {
		return err
	}
	instanceCount, err := h.repo.CountTenantTable(ctx, tenantID, "flow_instances")
	if err != nil {
		return err
	}
	templateCount, err := h.repo.CountTenantTable(ctx, tenantID, "execution_tasks")
	if err != nil {
		return err
	}
	auditLogCount, err := h.repo.CountTenantTable(ctx, tenantID, "audit_logs")
	if err != nil {
		return err
	}
	lastActivityAt, err := h.repo.GetTenantLastActivity(ctx, tenantID)
	if err != nil {
		return err
	}
	item.MemberCount = memberCount
	item.RuleCount = ruleCount
	item.InstanceCount = instanceCount
	item.TemplateCount = templateCount
	item.AuditLogCount = auditLogCount
	item.LastActivityAt = lastActivityAt
	return nil
}

func (h *TenantHandler) fillTenantStatsResources(ctx context.Context, tenantID uuid.UUID, item *TenantStatsItem) error {
	var err error
	if item.CmdbCount, err = h.repo.CountTenantTable(ctx, tenantID, "cmdb_items"); err != nil {
		return err
	}
	if item.GitCount, err = h.repo.CountTenantTable(ctx, tenantID, "git_repositories"); err != nil {
		return err
	}
	if item.PlaybookCount, err = h.repo.CountTenantTable(ctx, tenantID, "playbooks"); err != nil {
		return err
	}
	if item.SecretCount, err = h.repo.CountTenantTable(ctx, tenantID, "secrets_sources"); err != nil {
		return err
	}
	if item.PluginCount, err = h.repo.CountTenantTable(ctx, tenantID, "plugins"); err != nil {
		return err
	}
	if item.IncidentCount, err = h.repo.CountTenantTable(ctx, tenantID, "incidents"); err != nil {
		return err
	}
	if item.FlowCount, err = h.repo.CountTenantTable(ctx, tenantID, "healing_flows"); err != nil {
		return err
	}
	if item.ScheduleCount, err = h.repo.CountTenantTable(ctx, tenantID, "execution_schedules"); err != nil {
		return err
	}
	if item.NotificationChannelCount, err = h.repo.CountTenantTable(ctx, tenantID, "notification_channels"); err != nil {
		return err
	}
	if item.NotificationTemplateCount, err = h.repo.CountTenantTable(ctx, tenantID, "notification_templates"); err != nil {
		return err
	}
	return nil
}

func (h *TenantHandler) fillTenantStatsDerived(ctx context.Context, tenantID uuid.UUID, item *TenantStatsItem) error {
	var err error
	if item.HealingSuccessCount, err = h.repo.CountTenantTableWhere(ctx, tenantID, "flow_instances", "status = 'completed'"); err != nil {
		return err
	}
	if item.HealingTotalCount, err = h.repo.CountTenantTable(ctx, tenantID, "flow_instances"); err != nil {
		return err
	}
	if item.IncidentCoveredCount, err = h.repo.CountTenantTableWhere(ctx, tenantID, "incidents", "matched_rule_id IS NOT NULL"); err != nil {
		return err
	}
	return nil
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
