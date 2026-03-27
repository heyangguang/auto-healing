package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HealingSection struct {
	FlowsTotal          int64          `json:"flows_total"`
	FlowsActive         int64          `json:"flows_active"`
	RulesTotal          int64          `json:"rules_total"`
	RulesActive         int64          `json:"rules_active"`
	InstancesTotal      int64          `json:"instances_total"`
	InstancesRunning    int64          `json:"instances_running"`
	PendingApprovals    int64          `json:"pending_approvals"`
	PendingTriggers     int64          `json:"pending_triggers"`
	InstancesByStatus   []StatusCount  `json:"instances_by_status"`
	InstanceTrend7d     []TrendPoint   `json:"instance_trend_7d"`
	ApprovalsByStatus   []StatusCount  `json:"approvals_by_status"`
	RulesByTriggerMode  []StatusCount  `json:"rules_by_trigger_mode"`
	FlowTop10           []RankItem     `json:"flow_top10"`
	RecentInstances     []InstanceItem `json:"recent_instances"`
	PendingApprovalList []ApprovalItem `json:"pending_approval_list"`
	PendingTriggerList  []TriggerItem  `json:"pending_trigger_list"`
}

type InstanceItem struct {
	ID        uuid.UUID `json:"id"`
	FlowName  string    `json:"flow_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ApprovalItem struct {
	ID             uuid.UUID `json:"id"`
	FlowInstanceID uuid.UUID `json:"flow_instance_id"`
	NodeID         string    `json:"node_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type TriggerItem struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	AffectedCI string    `json:"affected_ci"`
	CreatedAt  time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetHealingSection(ctx context.Context, permissions []string) (*HealingSection, error) {
	section := &HealingSection{}
	db := r.tenantDB(ctx)
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if repoHasPermission(permissions, "healing:flows:view") {
		if err := countModel(db, &model.HealingFlow{}, &section.FlowsTotal); err != nil {
			return nil, err
		}
		if err := countModel(db.Where("is_active = ?", true), &model.HealingFlow{}, &section.FlowsActive); err != nil {
			return nil, err
		}
		flowTop, err := listTopHealingFlows(r.db.WithContext(ctx), tenantID)
		if err != nil {
			return nil, err
		}
		section.FlowTop10 = flowTop
	}
	if repoHasPermission(permissions, "healing:rules:view") {
		if err := countModel(db, &model.HealingRule{}, &section.RulesTotal); err != nil {
			return nil, err
		}
		if err := countModel(db.Where("is_active = ?", true), &model.HealingRule{}, &section.RulesActive); err != nil {
			return nil, err
		}
		if err := scanStatusCounts(db, &model.HealingRule{}, "trigger_mode", &section.RulesByTriggerMode); err != nil {
			return nil, err
		}
	}
	if repoHasPermission(permissions, "healing:instances:view") {
		if err := countModel(db, &model.FlowInstance{}, &section.InstancesTotal); err != nil {
			return nil, err
		}
		if err := countModel(db.Where("status = ?", "running"), &model.FlowInstance{}, &section.InstancesRunning); err != nil {
			return nil, err
		}
		if err := scanStatusCounts(db, &model.FlowInstance{}, "status", &section.InstancesByStatus); err != nil {
			return nil, err
		}
		if err := scanTrendPoints(db, &model.FlowInstance{}, "created_at", time.Now().AddDate(0, 0, -7), &section.InstanceTrend7d); err != nil {
			return nil, err
		}
		recent, err := listRecentInstances(db.Order("created_at DESC").Limit(10))
		if err != nil {
			return nil, err
		}
		section.RecentInstances = recent
	}
	if repoHasPermission(permissions, "healing:approvals:view") {
		if err := countModel(db.Where("status = ?", "pending"), &model.ApprovalTask{}, &section.PendingApprovals); err != nil {
			return nil, err
		}
		if err := scanStatusCounts(db, &model.ApprovalTask{}, "status", &section.ApprovalsByStatus); err != nil {
			return nil, err
		}
		approvals, err := listPendingApprovals(db.Where("status = ?", "pending").Order("created_at DESC").Limit(10))
		if err != nil {
			return nil, err
		}
		section.PendingApprovalList = approvals
	}
	if repoHasPermission(permissions, "healing:trigger:view") {
		if err := countModel(pendingTriggerQuery(db), &model.Incident{}, &section.PendingTriggers); err != nil {
			return nil, err
		}
		triggers, err := listPendingTriggers(pendingTriggerQuery(db).Order("created_at DESC").Limit(10))
		if err != nil {
			return nil, err
		}
		section.PendingTriggerList = triggers
	}
	return section, nil
}

func pendingTriggerQuery(db *gorm.DB) *gorm.DB {
	return db.Where("scanned = ? AND matched_rule_id IS NOT NULL AND healing_flow_instance_id IS NULL", true)
}

func listTopHealingFlows(db *gorm.DB, tenantID uuid.UUID) ([]RankItem, error) {
	var items []RankItem
	if err := db.Where("fi.tenant_id = ?", tenantID).
		Table("flow_instances fi").
		Select("hf.name as name, count(*) as count").
		Joins("JOIN healing_flows hf ON fi.flow_id = hf.id AND hf.tenant_id = ?", tenantID).
		Group("hf.name").
		Order("count DESC").
		Limit(10).
		Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func listRecentInstances(query *gorm.DB) ([]InstanceItem, error) {
	var instances []model.FlowInstance
	if err := query.Find(&instances).Error; err != nil {
		return nil, err
	}
	items := make([]InstanceItem, 0, len(instances))
	for _, instance := range instances {
		items = append(items, InstanceItem{
			ID:        instance.ID,
			FlowName:  instance.FlowName,
			Status:    instance.Status,
			CreatedAt: instance.CreatedAt,
		})
	}
	return items, nil
}

func listPendingApprovals(query *gorm.DB) ([]ApprovalItem, error) {
	var approvals []model.ApprovalTask
	if err := query.Find(&approvals).Error; err != nil {
		return nil, err
	}
	items := make([]ApprovalItem, 0, len(approvals))
	for _, approval := range approvals {
		items = append(items, ApprovalItem{
			ID:             approval.ID,
			FlowInstanceID: approval.FlowInstanceID,
			NodeID:         approval.NodeID,
			Status:         approval.Status,
			CreatedAt:      approval.CreatedAt,
		})
	}
	return items, nil
}

func listPendingTriggers(query *gorm.DB) ([]TriggerItem, error) {
	var incidents []model.Incident
	if err := query.Find(&incidents).Error; err != nil {
		return nil, err
	}
	items := make([]TriggerItem, 0, len(incidents))
	for _, incident := range incidents {
		items = append(items, TriggerItem{
			ID:         incident.ID,
			Title:      incident.Title,
			Severity:   incident.Severity,
			AffectedCI: incident.AffectedCI,
			CreatedAt:  incident.CreatedAt,
		})
	}
	return items, nil
}
