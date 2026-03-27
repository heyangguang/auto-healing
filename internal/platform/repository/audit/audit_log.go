package audit

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if log.TenantID == nil {
		tenantID, ok := TenantIDFromContextOK(ctx)
		if ok {
			log.TenantID = &tenantID
		}
	}
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *AuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AuditLog, error) {
	var log model.AuditLog
	err := tenantDB(r.db, ctx).Preload("User").First(&log, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}
