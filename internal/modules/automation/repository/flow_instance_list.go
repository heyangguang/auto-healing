package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// List 获取流程实例列表
func (r *FlowInstanceRepository) List(ctx context.Context, page, pageSize int, flowID, ruleID *uuid.UUID, incidentID *uuid.UUID, status string, search string) ([]model.FlowInstance, int64, error) {
	var instances []model.FlowInstance
	var total int64
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, 0, err
	}
	query := r.db.WithContext(ctx).Model(&model.FlowInstance{}).Where("flow_instances.tenant_id = ?", tenantID)
	query = applyFlowInstanceSimpleFilters(query, flowID, ruleID, incidentID, status, search)
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Preload("Rule").Preload("Incident").Offset((page - 1) * pageSize).Limit(pageSize).Order("flow_instances.created_at DESC").Find(&instances).Error
	return instances, total, err
}

func applyFlowInstanceSimpleFilters(query *gorm.DB, flowID, ruleID, incidentID *uuid.UUID, status, search string) *gorm.DB {
	if flowID != nil {
		query = query.Where("flow_instances.flow_id = ?", *flowID)
	}
	if ruleID != nil {
		query = query.Where("flow_instances.rule_id = ?", *ruleID)
	}
	if incidentID != nil {
		query = query.Where("flow_instances.incident_id = ?", *incidentID)
	}
	if status != "" {
		query = query.Where("flow_instances.status = ?", status)
	}
	if search != "" {
		pattern := "%" + search + "%"
		query = query.
			Joins("LEFT JOIN healing_flows ON healing_flows.id = flow_instances.flow_id").
			Joins("LEFT JOIN healing_rules ON healing_rules.id = flow_instances.rule_id").
			Joins("LEFT JOIN incidents ON incidents.id = flow_instances.incident_id").
			Where("(flow_instances.id::text ILIKE ? OR healing_flows.name ILIKE ? OR healing_rules.name ILIKE ? OR incidents.title ILIKE ?)",
				pattern, pattern, pattern, pattern)
	}
	return query
}
