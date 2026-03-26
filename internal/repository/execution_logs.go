package repository

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// AppendLog 追加执行日志
func (r *ExecutionRepository) AppendLog(ctx context.Context, log *model.ExecutionLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// GetLogsByRunID 获取执行记录的日志
func (r *ExecutionRepository) GetLogsByRunID(ctx context.Context, runID uuid.UUID) ([]model.ExecutionLog, error) {
	var logs []model.ExecutionLog
	err := r.tenantDB(ctx).
		Where("run_id = ?", runID).
		Order("sequence ASC").
		Find(&logs).Error
	return logs, err
}

// GetNextLogSequence 获取下一个日志序号
func (r *ExecutionRepository) GetNextLogSequence(ctx context.Context, runID uuid.UUID) (int, error) {
	var maxSeq int
	err := r.tenantDB(ctx).
		Model(&model.ExecutionLog{}).
		Where("run_id = ?", runID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSeq).Error
	return maxSeq + 1, err
}
