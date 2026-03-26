package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// Start 将 pending 实例原子切换为 running，并记录开始时间。
func (r *FlowInstanceRepository) Start(ctx context.Context, id uuid.UUID) (bool, error) {
	return r.updateLifecycleStatus(ctx, id, []string{model.FlowInstanceStatusPending}, model.FlowInstanceStatusRunning, "")
}

// RetryStart 将 failed 实例原子切换为 running，并重置开始/结束信息。
func (r *FlowInstanceRepository) RetryStart(ctx context.Context, id uuid.UUID) (bool, error) {
	return r.updateLifecycleStatus(ctx, id, []string{model.FlowInstanceStatusFailed}, model.FlowInstanceStatusRunning, "")
}

// EnterWaitingApproval 将 running 实例原子切换为 waiting_approval。
func (r *FlowInstanceRepository) EnterWaitingApproval(ctx context.Context, id uuid.UUID) (bool, error) {
	result := TenantDB(r.db, ctx).
		Model(&model.FlowInstance{}).
		Where("id = ? AND status = ?", id, model.FlowInstanceStatusRunning).
		Update("status", model.FlowInstanceStatusWaitingApproval)
	return result.RowsAffected > 0, result.Error
}

// UpdateStatus 更新流程实例状态
func (r *FlowInstanceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMsg string) error {
	_, err := r.updateLifecycleStatus(ctx, id, nil, status, errorMsg)
	return err
}

// UpdateStatusIfCurrent 仅当当前状态匹配时更新流程实例状态。
func (r *FlowInstanceRepository) UpdateStatusIfCurrent(ctx context.Context, id uuid.UUID, currentStatuses []string, status string, errorMsg string) (bool, error) {
	return r.updateLifecycleStatus(ctx, id, currentStatuses, status, errorMsg)
}

func (r *FlowInstanceRepository) updateLifecycleStatus(ctx context.Context, id uuid.UUID, currentStatuses []string, status string, errorMsg string) (bool, error) {
	updates := flowInstanceStatusUpdates(status, errorMsg)
	query := TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("id = ?", id)
	if len(currentStatuses) > 0 {
		query = query.Where("status IN ?", currentStatuses)
	}
	result := query.Updates(updates)
	return result.RowsAffected > 0, result.Error
}

func flowInstanceStatusUpdates(status, errorMsg string) map[string]interface{} {
	updates := map[string]interface{}{"status": status}
	if errorMsg != "" || status == model.FlowInstanceStatusRunning {
		updates["error_message"] = errorMsg
	}
	if status == model.FlowInstanceStatusRunning {
		updates["started_at"] = time.Now()
		updates["completed_at"] = nil
	}
	if status == model.FlowInstanceStatusCompleted || status == model.FlowInstanceStatusFailed || status == model.FlowInstanceStatusCancelled {
		updates["completed_at"] = time.Now()
	}
	return updates
}

// UpdateNodeStates 更新节点状态
func (r *FlowInstanceRepository) UpdateNodeStates(ctx context.Context, id uuid.UUID, nodeStates model.JSON) error {
	return TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Update("node_states", nodeStates).Error
}

// ListStaleRunning 查询所有停滞的 running/pending 实例（跨租户）
func (r *FlowInstanceRepository) ListStaleRunning(ctx context.Context, staleThreshold time.Duration) ([]model.FlowInstance, error) {
	var instances []model.FlowInstance
	cutoff := time.Now().Add(-staleThreshold)
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{model.FlowInstanceStatusRunning, model.FlowInstanceStatusPending}).
		Where("updated_at < ?", cutoff).
		Find(&instances).Error
	return instances, err
}

// UpdateCurrentNodeAndStates 更新当前节点和节点状态
func (r *FlowInstanceRepository) UpdateCurrentNodeAndStates(ctx context.Context, id uuid.UUID, currentNodeID string, nodeStates model.JSON) error {
	return TenantDB(r.db, ctx).Model(&model.FlowInstance{}).Where("id = ?", id).Updates(map[string]interface{}{
		"current_node_id": currentNodeID,
		"node_states":     nodeStates,
	}).Error
}
