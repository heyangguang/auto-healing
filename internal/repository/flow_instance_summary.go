package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FlowInstanceListOptions 实例列表查询选项
type FlowInstanceListOptions struct {
	Page           int
	PageSize       int
	Search         query.StringFilter
	SortBy         string
	SortOrder      string
	Status         string
	FlowID         *uuid.UUID
	FlowName       query.StringFilter
	RuleID         *uuid.UUID
	RuleName       query.StringFilter
	IncidentID     *uuid.UUID
	IncidentTitle  query.StringFilter
	CurrentNodeID  string
	ErrorMessage   query.StringFilter
	HasError       *bool
	ApprovalStatus string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	StartedFrom    *time.Time
	StartedTo      *time.Time
	CompletedFrom  *time.Time
	CompletedTo    *time.Time
	MinNodes       *int
	MaxNodes       *int
	MinFailedNodes *int
	MaxFailedNodes *int
}

// FlowInstanceSummary 列表接口的精简 DTO
type FlowInstanceSummary struct {
	ID                uuid.UUID  `json:"id"`
	Status            string     `json:"status"`
	FlowID            uuid.UUID  `json:"flow_id"`
	FlowName          string     `json:"flow_name"`
	RuleID            *uuid.UUID `json:"rule_id,omitempty"`
	RuleName          *string    `json:"rule_name,omitempty"`
	IncidentID        *uuid.UUID `json:"incident_id,omitempty"`
	IncidentTitle     *string    `json:"incident_title,omitempty"`
	CurrentNodeID     string     `json:"current_node_id,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	NodeCount         int        `json:"node_count"`
	FailedNodeCount   int        `json:"failed_node_count"`
	RejectedNodeCount int        `json:"rejected_node_count"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// ListSummary 获取流程实例精简列表（瘦身版）
func (r *FlowInstanceRepository) ListSummary(ctx context.Context, page, pageSize int, flowID, ruleID *uuid.UUID, incidentID *uuid.UUID, status string, search string) ([]FlowInstanceSummary, int64, error) {
	opts := FlowInstanceListOptions{
		Page:       page,
		PageSize:   pageSize,
		Status:     status,
		FlowID:     flowID,
		RuleID:     ruleID,
		IncidentID: incidentID,
	}
	if search != "" {
		opts.Search = query.StringFilter{Value: search}
	}
	return r.ListSummaryWithOptions(ctx, opts)
}

func flowInstanceSummaryBaseQuery(r *FlowInstanceRepository, ctx context.Context) (*gorm.DB, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return r.db.WithContext(ctx).Where("flow_instances.tenant_id = ?", tenantID).
		Table("flow_instances").
		Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
		Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
		Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id"), nil
}
