package incident

import (
	"context"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IncidentWritebackLogRepository struct {
	db *gorm.DB
}

func NewIncidentWritebackLogRepositoryWithDB(db *gorm.DB) *IncidentWritebackLogRepository {
	return &IncidentWritebackLogRepository{db: db}
}

func (r *IncidentWritebackLogRepository) Create(ctx context.Context, log *platformmodel.IncidentWritebackLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	return TenantDB(r.db, ctx).Create(log).Error
}

func (r *IncidentWritebackLogRepository) Update(ctx context.Context, log *platformmodel.IncidentWritebackLog) error {
	return UpdateTenantScopedModel(r.db, ctx, log.ID, log)
}

func (r *IncidentWritebackLogRepository) ListByIncidentID(ctx context.Context, incidentID uuid.UUID) ([]platformmodel.IncidentWritebackLog, error) {
	var logs []platformmodel.IncidentWritebackLog
	err := TenantDB(r.db, ctx).
		Where("incident_id = ?", incidentID).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}
