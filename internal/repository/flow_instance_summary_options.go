package repository

import (
	"context"

	"github.com/company/auto-healing/internal/pkg/query"
	"gorm.io/gorm"
)

// ListSummaryWithOptions 增强版列表查询，支持排序/过滤/时间范围/数量范围
func (r *FlowInstanceRepository) ListSummaryWithOptions(ctx context.Context, opts FlowInstanceListOptions) ([]FlowInstanceSummary, int64, error) {
	baseQuery, err := flowInstanceSummaryBaseQuery(r, ctx)
	if err != nil {
		return nil, 0, err
	}
	baseQuery = applyFlowInstanceSummaryFilters(baseQuery, opts)
	baseQuery = applyFlowInstanceSummaryCountFilters(baseQuery, opts)

	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var results []FlowInstanceSummary
	dataQuery, err := flowInstanceSummaryBaseQuery(r, ctx)
	if err != nil {
		return nil, 0, err
	}
	dataQuery = dataQuery.
		Select(`
				flow_instances.id,
			flow_instances.status,
			flow_instances.flow_id,
			COALESCE(flow_instances.flow_name, healing_flows.name, '') AS flow_name,
			flow_instances.rule_id,
			healing_rules.name AS rule_name,
			flow_instances.incident_id,
			incidents.title AS incident_title,
			flow_instances.current_node_id,
			flow_instances.error_message,
			COALESCE(jsonb_array_length(healing_flows.nodes), 0) AS node_count,
			(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) AS failed_node_count,
			(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'rejected') AS rejected_node_count,
			flow_instances.started_at,
			flow_instances.completed_at,
			flow_instances.created_at
		`)
	dataQuery = applyFlowInstanceSummaryFilters(dataQuery, opts)
	dataQuery = applyFlowInstanceSummaryCountFilters(dataQuery, opts)
	err = dataQuery.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize).Order(flowInstanceSummaryOrder(opts)).Scan(&results).Error
	return results, total, err
}

func applyFlowInstanceSummaryFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	q = applyFlowInstanceSummaryEntityFilters(q, opts)
	q = applyFlowInstanceSummaryTextFilters(q, opts)
	q = applyFlowInstanceSummaryErrorFilters(q, opts)
	q = applyFlowInstanceSummaryApprovalFilters(q, opts)
	q = applyFlowInstanceSummaryTimeFilters(q, opts)
	return q
}

func applyFlowInstanceSummaryEntityFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	if opts.Status != "" {
		q = q.Where("flow_instances.status = ?", opts.Status)
	}
	if opts.FlowID != nil {
		q = q.Where("flow_instances.flow_id = ?", *opts.FlowID)
	}
	if opts.RuleID != nil {
		q = q.Where("flow_instances.rule_id = ?", *opts.RuleID)
	}
	if opts.IncidentID != nil {
		q = q.Where("flow_instances.incident_id = ?", *opts.IncidentID)
	}
	if opts.CurrentNodeID != "" {
		q = q.Where("flow_instances.current_node_id = ?", opts.CurrentNodeID)
	}
	return q
}

func applyFlowInstanceSummaryTextFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	if !opts.FlowName.IsEmpty() {
		q = query.ApplyStringFilter(q, "COALESCE(flow_instances.flow_name, healing_flows.name, '')", opts.FlowName)
	}
	if !opts.RuleName.IsEmpty() {
		q = query.ApplyStringFilter(q, "healing_rules.name", opts.RuleName)
	}
	if !opts.IncidentTitle.IsEmpty() {
		q = query.ApplyStringFilter(q, "incidents.title", opts.IncidentTitle)
	}
	if !opts.ErrorMessage.IsEmpty() {
		q = query.ApplyStringFilter(q, "flow_instances.error_message", opts.ErrorMessage)
	}
	if !opts.Search.IsEmpty() {
		return query.ApplyMultiStringFilter(q, []string{"flow_instances.id::text", "COALESCE(flow_instances.flow_name, healing_flows.name, '')", "healing_rules.name", "incidents.title"}, opts.Search)
	}
	return q
}

func applyFlowInstanceSummaryErrorFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	if opts.HasError == nil {
		return q
	}
	if *opts.HasError {
		return q.Where("flow_instances.error_message IS NOT NULL AND flow_instances.error_message != ''")
	}
	return q.Where("(flow_instances.error_message IS NULL OR flow_instances.error_message = '')")
}

func applyFlowInstanceSummaryApprovalFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	switch opts.ApprovalStatus {
	case "approved":
		return q.Where("EXISTS (SELECT 1 FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'approved')")
	case "rejected":
		return q.Where("EXISTS (SELECT 1 FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' = 'rejected')")
	default:
		return q
	}
}

func applyFlowInstanceSummaryTimeFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	if opts.CreatedFrom != nil {
		q = q.Where("flow_instances.created_at >= ?", *opts.CreatedFrom)
	}
	if opts.CreatedTo != nil {
		q = q.Where("flow_instances.created_at <= ?", *opts.CreatedTo)
	}
	if opts.StartedFrom != nil {
		q = q.Where("flow_instances.started_at >= ?", *opts.StartedFrom)
	}
	if opts.StartedTo != nil {
		q = q.Where("flow_instances.started_at <= ?", *opts.StartedTo)
	}
	if opts.CompletedFrom != nil {
		q = q.Where("flow_instances.completed_at >= ?", *opts.CompletedFrom)
	}
	if opts.CompletedTo != nil {
		q = q.Where("flow_instances.completed_at <= ?", *opts.CompletedTo)
	}
	return q
}

func applyFlowInstanceSummaryCountFilters(q *gorm.DB, opts FlowInstanceListOptions) *gorm.DB {
	if opts.MinNodes != nil {
		q = q.Where("COALESCE(jsonb_array_length(healing_flows.nodes), 0) >= ?", *opts.MinNodes)
	}
	if opts.MaxNodes != nil {
		q = q.Where("COALESCE(jsonb_array_length(healing_flows.nodes), 0) <= ?", *opts.MaxNodes)
	}
	if opts.MinFailedNodes != nil {
		q = q.Where("(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) >= ?", *opts.MinFailedNodes)
	}
	if opts.MaxFailedNodes != nil {
		q = q.Where("(SELECT COUNT(*) FROM jsonb_each(flow_instances.node_states) AS ns(key, value) WHERE ns.value->>'status' IN ('failed', 'error')) <= ?", *opts.MaxFailedNodes)
	}
	return q
}

func flowInstanceSummaryOrder(opts FlowInstanceListOptions) string {
	orderClause := "flow_instances.created_at DESC"
	if opts.SortBy == "" {
		return orderClause
	}
	sortColumnMap := map[string]string{
		"created_at":   "flow_instances.created_at",
		"started_at":   "flow_instances.started_at",
		"completed_at": "flow_instances.completed_at",
		"status":       "flow_instances.status",
		"flow_name":    "flow_name",
		"rule_name":    "rule_name",
	}
	col, ok := sortColumnMap[opts.SortBy]
	if !ok {
		return orderClause
	}
	direction := "DESC"
	if opts.SortOrder == "asc" {
		direction = "ASC"
	}
	return col + " " + direction
}
