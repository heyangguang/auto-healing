package repository

import (
	"context"
	"errors"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApprovalTaskRepository 审批任务仓库
type ApprovalTaskRepository struct {
	db *gorm.DB
}

// NewApprovalTaskRepository 创建审批任务仓库
func NewApprovalTaskRepository() *ApprovalTaskRepository {
	return &ApprovalTaskRepository{db: database.DB}
}

// Create 创建审批任务
func (r *ApprovalTaskRepository) Create(ctx context.Context, task *model.ApprovalTask) error {
	if err := FillTenantID(ctx, &task.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(task).Error
}

// CreateAndEnterWaiting 在同一事务中创建审批任务并将流程实例切换到 waiting_approval。
func (r *ApprovalTaskRepository) CreateAndEnterWaiting(ctx context.Context, task *model.ApprovalTask) error {
	if err := FillTenantID(ctx, &task.TenantID); err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.FlowInstance{}).
			Where("id = ? AND tenant_id = ? AND status = ?", task.FlowInstanceID, *task.TenantID, model.FlowInstanceStatusRunning).
			Update("status", model.FlowInstanceStatusWaitingApproval)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrFlowInstanceStateConflict
		}
		return tx.Create(task).Error
	})
}

// GetByID 根据 ID 获取审批任务
func (r *ApprovalTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ApprovalTask, error) {
	var task model.ApprovalTask
	err := TenantDB(r.db, ctx).
		Preload("FlowInstance").
		Preload("FlowInstance.Incident").
		Preload("Initiator").
		Preload("Decider").
		First(&task, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApprovalTaskNotFound
	}
	return &task, err
}

// Update 更新审批任务
func (r *ApprovalTaskRepository) Update(ctx context.Context, task *model.ApprovalTask) error {
	return UpdateTenantScopedModel(r.db, ctx, task.ID, task)
}

// Approve 批准审批任务
func (r *ApprovalTaskRepository) Approve(ctx context.Context, id uuid.UUID, decidedBy uuid.UUID, comment string) error {
	result := TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).
		Where("id = ? AND status = ?", id, model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           model.ApprovalTaskStatusApproved,
			"decided_by":       decidedBy,
			"decided_at":       gorm.Expr("NOW()"),
			"decision_comment": comment,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrApprovalTaskNotPending
	}
	return nil
}

// Reject 拒绝审批任务
func (r *ApprovalTaskRepository) Reject(ctx context.Context, id uuid.UUID, decidedBy uuid.UUID, comment string) error {
	result := TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).
		Where("id = ? AND status = ?", id, model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           model.ApprovalTaskStatusRejected,
			"decided_by":       decidedBy,
			"decided_at":       gorm.Expr("NOW()"),
			"decision_comment": comment,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrApprovalTaskNotPending
	}
	return nil
}

// CancelPendingByFlowInstance 关闭指定流程实例下所有待审批任务。
func (r *ApprovalTaskRepository) CancelPendingByFlowInstance(ctx context.Context, flowInstanceID uuid.UUID, comment string) (int64, error) {
	result := TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).
		Where("flow_instance_id = ? AND status = ?", flowInstanceID, model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           model.ApprovalTaskStatusCancelled,
			"decided_at":       time.Now(),
			"decision_comment": comment,
		})
	return result.RowsAffected, result.Error
}

// ExpireTimedOut 将超时的审批任务标记为过期（跨租户，调度器专用）
// 注意：不使用 TenantDB，调度器需要处理所有租户的超时审批
func (r *ApprovalTaskRepository) ExpireTimedOut(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.ApprovalTask{}).
		Where("status = ? AND timeout_at < NOW()", model.ApprovalTaskStatusPending).
		Updates(map[string]interface{}{
			"status":     model.ApprovalTaskStatusExpired,
			"decided_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected, result.Error
}

func (r *ApprovalTaskRepository) ListRecentlyExpired(ctx context.Context, since time.Time) ([]model.ApprovalTask, error) {
	var tasks []model.ApprovalTask
	err := r.db.WithContext(ctx).
		Where("status = ?", model.ApprovalTaskStatusExpired).
		Where("updated_at >= ?", since).
		Find(&tasks).Error
	return tasks, err
}

// ListPending 获取待审批列表
// 支持搜索和过滤：nodeName（模糊匹配 node_id, flow_instance_id）、dateFrom、dateTo
func (r *ApprovalTaskRepository) ListPending(ctx context.Context, page, pageSize int, nodeName, dateFrom, dateTo string) ([]model.ApprovalTask, int64, error) {
	var tasks []model.ApprovalTask
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.ApprovalTask{}).Where("status = ?", model.ApprovalTaskStatusPending)

	// 模糊搜索：node_id 或 flow_instance_id
	if nodeName != "" {
		searchPattern := "%" + nodeName + "%"
		query = query.Where("(node_id ILIKE ? OR flow_instance_id::text ILIKE ?)", searchPattern, searchPattern)
	}

	// 日期范围过滤
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom+" 00:00:00")
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("FlowInstance").
		Preload("FlowInstance.Incident").
		Preload("Initiator").
		Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// List 获取审批任务列表
func (r *ApprovalTaskRepository) List(ctx context.Context, page, pageSize int, flowInstanceID *uuid.UUID, status string) ([]model.ApprovalTask, int64, error) {
	var tasks []model.ApprovalTask
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.ApprovalTask{})

	if flowInstanceID != nil {
		query = query.Where("flow_instance_id = ?", *flowInstanceID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("FlowInstance").
		Preload("Initiator").
		Preload("Decider").
		Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// GetByFlowInstanceAndNode 根据流程实例ID和节点ID获取审批任务
func (r *ApprovalTaskRepository) GetByFlowInstanceAndNode(ctx context.Context, flowInstanceID uuid.UUID, nodeID string) (*model.ApprovalTask, error) {
	var task model.ApprovalTask
	err := TenantDB(r.db, ctx).
		Where("flow_instance_id = ? AND node_id = ?", flowInstanceID, nodeID).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApprovalTaskNotFound
	}
	return &task, err
}

// =============================================================================
// FlowLogRepository - 流程执行日志仓库
// =============================================================================
