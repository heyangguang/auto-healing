package repository

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FlowInstanceRepository 流程实例仓库
type FlowInstanceRepository struct {
	db *gorm.DB
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
