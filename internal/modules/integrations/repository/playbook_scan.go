package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *PlaybookRepository) PersistScanOutcome(ctx context.Context, playbookID uuid.UUID, nextStatus string, scannedVariables model.JSONArray, build func(currentVariables model.JSONArray) (model.JSONArray, *model.PlaybookScanLog, error)) (*model.PlaybookScanLog, error) {
	now := time.Now()
	var logEntry *model.PlaybookScanLog

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.Playbook
		if err := TenantDB(tx, ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "tenant_id", "variables").
			First(&current, "id = ?", playbookID).Error; err != nil {
			return err
		}

		mergedVariables, builtLog, err := build(current.Variables)
		if err != nil {
			return err
		}
		if err := FillTenantID(ctx, &builtLog.TenantID); err != nil {
			return err
		}

		updates := map[string]any{
			"variables":         mergedVariables,
			"scanned_variables": scannedVariables,
			"last_scanned_at":   now,
			"updated_at":        now,
		}
		if nextStatus != "" {
			updates["status"] = nextStatus
		}

		if err := TenantDB(tx, ctx).
			Model(&model.Playbook{}).
			Where("id = ?", playbookID).
			Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Create(builtLog).Error; err != nil {
			return err
		}
		logEntry = builtLog
		return nil
	})
	return logEntry, err
}
