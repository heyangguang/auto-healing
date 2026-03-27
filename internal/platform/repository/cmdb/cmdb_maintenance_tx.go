package cmdb

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *CMDBItemRepository) EnterMaintenanceWithLog(ctx context.Context, id uuid.UUID, reason string, endAt *time.Time, log *model.CMDBMaintenanceLog) error {
	return r.withMaintenanceLog(ctx, log, func(tx *gorm.DB) error {
		return txEnterMaintenance(tx, ctx, id, reason, endAt)
	})
}

func (r *CMDBItemRepository) ExitMaintenanceWithLog(ctx context.Context, id uuid.UUID, log *model.CMDBMaintenanceLog) error {
	return r.withMaintenanceLog(ctx, log, func(tx *gorm.DB) error {
		return txExitMaintenance(tx, ctx, id)
	})
}

func (r *CMDBItemRepository) withMaintenanceLog(ctx context.Context, log *model.CMDBMaintenanceLog, mutate func(tx *gorm.DB) error) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := mutate(tx); err != nil {
			return err
		}
		return tx.Create(log).Error
	})
}

func txEnterMaintenance(tx *gorm.DB, ctx context.Context, id uuid.UUID, reason string, endAt *time.Time) error {
	now := time.Now()
	return TenantDB(tx, ctx).Model(&model.CMDBItem{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":               "maintenance",
		"maintenance_reason":   reason,
		"maintenance_start_at": &now,
		"maintenance_end_at":   endAt,
	}).Error
}

func txExitMaintenance(tx *gorm.DB, ctx context.Context, id uuid.UUID) error {
	return TenantDB(tx, ctx).Model(&model.CMDBItem{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":               "active",
		"maintenance_reason":   "",
		"maintenance_start_at": nil,
		"maintenance_end_at":   nil,
	}).Error
}
