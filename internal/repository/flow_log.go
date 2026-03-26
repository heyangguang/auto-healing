package repository

import (
	"context"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FlowLogRepository 流程执行日志仓库
type FlowLogRepository struct {
	db *gorm.DB
}

// NewFlowLogRepository 创建流程执行日志仓库
func NewFlowLogRepository() *FlowLogRepository {
	return &FlowLogRepository{db: database.DB}
}

// Create 创建日志
func (r *FlowLogRepository) Create(ctx context.Context, log *model.FlowExecutionLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// CreateBatch 批量创建日志
func (r *FlowLogRepository) CreateBatch(ctx context.Context, logs []*model.FlowExecutionLog) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	for _, log := range logs {
		if log.TenantID == nil {
			log.TenantID = &tenantID
		}
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

// GetByInstanceID 根据流程实例ID获取所有日志
func (r *FlowLogRepository) GetByInstanceID(ctx context.Context, instanceID uuid.UUID) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ?", instanceID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByInstanceAndNode 根据流程实例ID和节点ID获取日志
func (r *FlowLogRepository) GetByInstanceAndNode(ctx context.Context, instanceID uuid.UUID, nodeID string) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ? AND node_id = ?", instanceID, nodeID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// GetByLevel 根据日志级别获取日志
func (r *FlowLogRepository) GetByLevel(ctx context.Context, instanceID uuid.UUID, level string) ([]*model.FlowExecutionLog, error) {
	var logs []*model.FlowExecutionLog
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ? AND level = ?", instanceID, level).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// DeleteByInstanceID 删除流程实例的所有日志
func (r *FlowLogRepository) DeleteByInstanceID(ctx context.Context, instanceID uuid.UUID) error {
	return TenantDB(r.db, ctx).
		Where("flow_instance_id = ?", instanceID).
		Delete(&model.FlowExecutionLog{}).Error
}

// ListPaginated 分页获取日志
func (r *FlowLogRepository) ListPaginated(ctx context.Context, instanceID uuid.UUID, page, pageSize int) ([]*model.FlowExecutionLog, int64, error) {
	var logs []*model.FlowExecutionLog
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.FlowExecutionLog{}).Where("flow_instance_id = ?", instanceID)

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at ASC").Find(&logs).Error
	return logs, total, err
}
