package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FlowInstanceRepository 流程实例仓库
type FlowInstanceRepository struct {
	db *gorm.DB
}

type IncidentSyncOptions struct {
	IncidentID        uuid.UUID
	HealingStatus     string
	MatchedRuleID     *uuid.UUID
	FlowInstanceID    *uuid.UUID
	Scanned           *bool
	ResetFlowInstance bool
}

// NewFlowInstanceRepository 创建流程实例仓库
func NewFlowInstanceRepository() *FlowInstanceRepository {
	return &FlowInstanceRepository{db: database.DB}
}

// Create 创建流程实例
func (r *FlowInstanceRepository) Create(ctx context.Context, instance *model.FlowInstance) error {
	if err := FillTenantID(ctx, &instance.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(instance).Error
}

func (r *FlowInstanceRepository) CreateWithIncidentSync(ctx context.Context, instance *model.FlowInstance, opts IncidentSyncOptions) error {
	if err := FillTenantID(ctx, &instance.TenantID); err != nil {
		return err
	}
	return TenantDB(r.db, ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Create(instance).Error; err != nil {
			return err
		}
		return updateIncidentSyncTx(tx, ctx, opts)
	})
}

func updateIncidentSyncTx(tx *gorm.DB, ctx context.Context, opts IncidentSyncOptions) error {
	result := TenantDB(tx, ctx).
		Model(&model.Incident{}).
		Where("id = ?", opts.IncidentID).
		Updates(incidentSyncUpdates(opts))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("工单不存在: %s", opts.IncidentID)
	}
	return nil
}

func incidentSyncUpdates(opts IncidentSyncOptions) map[string]interface{} {
	updates := map[string]interface{}{}
	if opts.HealingStatus != "" {
		updates["healing_status"] = opts.HealingStatus
	}
	if opts.MatchedRuleID != nil {
		updates["matched_rule_id"] = *opts.MatchedRuleID
	}
	if opts.FlowInstanceID != nil {
		updates["healing_flow_instance_id"] = *opts.FlowInstanceID
	}
	if opts.Scanned != nil {
		updates["scanned"] = *opts.Scanned
	}
	if opts.ResetFlowInstance {
		updates["healing_flow_instance_id"] = nil
	}
	return updates
}

// GetByID 根据 ID 获取流程实例
func (r *FlowInstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.FlowInstance, error) {
	var instance model.FlowInstance
	err := TenantDB(r.db, ctx).Preload("Rule").Preload("Incident").First(&instance, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFlowInstanceNotFound
	}
	if err != nil {
		return nil, err
	}
	backfillFlowSnapshot(ctx, r.db, &instance)
	return &instance, nil
}

func backfillFlowSnapshot(ctx context.Context, db *gorm.DB, instance *model.FlowInstance) {
	if len(instance.FlowNodes) != 0 && len(instance.FlowEdges) != 0 && instance.FlowName != "" {
		return
	}

	var flow model.HealingFlow
	if err := db.WithContext(ctx).First(&flow, "id = ?", instance.FlowID).Error; err != nil {
		return
	}
	if len(instance.FlowNodes) == 0 {
		instance.FlowNodes = flow.Nodes
	}
	if len(instance.FlowEdges) == 0 {
		instance.FlowEdges = flow.Edges
	}
	if instance.FlowName == "" {
		instance.FlowName = flow.Name
	}
}

// Update 更新流程实例
func (r *FlowInstanceRepository) Update(ctx context.Context, instance *model.FlowInstance) error {
	return TenantDB(r.db, ctx).
		Model(&model.FlowInstance{}).
		Where("id = ?", instance.ID).
		Select("flow_name", "flow_nodes", "flow_edges", "rule_id", "incident_id", "context", "current_node_id", "started_at", "updated_at").
		Updates(instance).Error
}
