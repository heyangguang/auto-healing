package repository

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FlowRecoveryAttemptRepository struct {
	db *gorm.DB
}

func NewFlowRecoveryAttemptRepositoryWithDB(db *gorm.DB) *FlowRecoveryAttemptRepository {
	return &FlowRecoveryAttemptRepository{db: db}
}

func (r *FlowRecoveryAttemptRepository) Create(ctx context.Context, attempt *model.FlowRecoveryAttempt) error {
	if err := platformrepo.FillTenantID(ctx, &attempt.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *FlowRecoveryAttemptRepository) Update(ctx context.Context, attempt *model.FlowRecoveryAttempt) error {
	return platformrepo.UpdateTenantScopedModel(r.db, ctx, attempt.ID, attempt)
}

func (r *FlowRecoveryAttemptRepository) ListByFlowInstanceID(ctx context.Context, flowInstanceID uuid.UUID) ([]model.FlowRecoveryAttempt, error) {
	var attempts []model.FlowRecoveryAttempt
	err := platformrepo.TenantDB(r.db, ctx).
		Where("flow_instance_id = ?", flowInstanceID).
		Order("created_at DESC").
		Find(&attempts).Error
	return attempts, err
}
