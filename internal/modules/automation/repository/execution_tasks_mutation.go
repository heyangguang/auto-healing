package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BatchConfirmReviewByIDs 按任务 ID 列表批量确认审核
func (r *ExecutionRepository) BatchConfirmReviewByIDs(ctx context.Context, taskIDs []uuid.UUID) (int64, error) {
	return r.batchConfirmReview(ctx, "id IN ?", taskIDs)
}

// BatchConfirmReviewByPlaybookID 按 Playbook ID 批量确认审核
func (r *ExecutionRepository) BatchConfirmReviewByPlaybookID(ctx context.Context, playbookID uuid.UUID) (int64, error) {
	return r.batchConfirmReview(ctx, "playbook_id = ?", playbookID)
}

func (r *ExecutionRepository) batchConfirmReview(ctx context.Context, predicate string, value any) (int64, error) {
	result := r.tenantDB(ctx).
		Model(&model.ExecutionTask{}).
		Where(predicate+" AND needs_review = ?", value, true).
		Updates(map[string]any{
			"needs_review":      false,
			"changed_variables": "[]",
		})
	return result.RowsAffected, result.Error
}

// ListTasksByPlaybookIDAndReview 查询指定 Playbook 下需要审核的任务
func (r *ExecutionRepository) ListTasksByPlaybookIDAndReview(ctx context.Context, playbookID uuid.UUID) ([]model.ExecutionTask, error) {
	return r.listReviewedTasks(ctx, "playbook_id = ?", playbookID)
}

// ListTasksByIDsAndReview 查询指定 ID 列表中需要审核的任务
func (r *ExecutionRepository) ListTasksByIDsAndReview(ctx context.Context, taskIDs []uuid.UUID) ([]model.ExecutionTask, error) {
	return r.listReviewedTasks(ctx, "id IN ?", taskIDs)
}

func (r *ExecutionRepository) listReviewedTasks(ctx context.Context, predicate string, value any) ([]model.ExecutionTask, error) {
	var tasks []model.ExecutionTask
	err := r.tenantDB(ctx).
		Where(predicate+" AND needs_review = ?", value, true).
		Find(&tasks).Error
	return tasks, err
}

// UpdateTask 更新任务模板
func (r *ExecutionRepository) UpdateTask(ctx context.Context, task *model.ExecutionTask) error {
	return r.tenantDB(ctx).
		Model(task).
		Select("name", "playbook_id", "target_hosts", "extra_vars", "executor_type", "description", "secrets_source_ids", "notification_config", "playbook_variables_snapshot", "needs_review", "changed_variables", "updated_at").
		Updates(task).Error
}

// DeleteTask 删除任务模板（级联删除 runs、logs、notification_logs）
func (r *ExecutionRepository) DeleteTask(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		runIDs, err := r.listRunIDsForDelete(tx, id)
		if err != nil {
			return err
		}
		if err := r.deleteTaskDependencies(tx, id, runIDs); err != nil {
			return err
		}
		return tx.Delete(&model.ExecutionTask{}, "id = ?", id).Error
	})
}

func (r *ExecutionRepository) listRunIDsForDelete(tx *gorm.DB, taskID uuid.UUID) ([]uuid.UUID, error) {
	var runIDs []uuid.UUID
	err := tx.Model(&model.ExecutionRun{}).Where("task_id = ?", taskID).Pluck("id", &runIDs).Error
	return runIDs, err
}

func (r *ExecutionRepository) deleteTaskDependencies(tx *gorm.DB, taskID uuid.UUID, runIDs []uuid.UUID) error {
	if len(runIDs) == 0 {
		return nil
	}
	if err := tx.Where("execution_run_id IN ?", runIDs).Delete(&model.NotificationLog{}).Error; err != nil {
		return err
	}
	if err := tx.Where("run_id IN ?", runIDs).Delete(&model.ExecutionLog{}).Error; err != nil {
		return err
	}
	return tx.Where("task_id = ?", taskID).Delete(&model.ExecutionRun{}).Error
}

// UpdateTaskReviewStatus 更新任务模板的 review 状态
func (r *ExecutionRepository) UpdateTaskReviewStatus(ctx context.Context, taskID uuid.UUID, needsReview bool, changedVariables model.JSONArray) error {
	return r.tenantDB(ctx).
		Model(&model.ExecutionTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{
			"needs_review":      needsReview,
			"changed_variables": changedVariables,
			"updated_at":        time.Now(),
		}).Error
}
