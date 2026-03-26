package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func (r *ScheduleRepository) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()
	return r.tenantDB(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", id).Updates(updates).Error
}
