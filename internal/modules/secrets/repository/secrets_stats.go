package repository

import (
	"context"
	"encoding/json"
	"errors"

	automationmodel "github.com/company/auto-healing/internal/modules/automation/model"
	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *SecretsSourceRepository) GetDefault(ctx context.Context) (*secretsmodel.SecretsSource, error) {
	var source secretsmodel.SecretsSource
	err := TenantDB(r.db, ctx).Where("is_default = ? AND status = ?", true, "active").First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = TenantDB(r.db, ctx).Where("status = ?", "active").Order("priority ASC, created_at ASC").First(&source).Error
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *SecretsSourceRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.setDefaultWithDB(ctx, tx, id)
	})
}

func (r *SecretsSourceRepository) setDefaultWithDB(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		Update("is_default", false).Error; err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("is_default", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SecretsSourceRepository) EnsureActiveDefault(ctx context.Context) error {
	return r.ensureActiveDefaultWithDB(ctx, r.db)
}

func (r *SecretsSourceRepository) ensureActiveDefaultWithDB(ctx context.Context, db *gorm.DB) error {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return err
	}

	var count int64
	if err := db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
		Where("tenant_id = ? AND is_default = ? AND status = ?", tenantID, true, "active").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var candidate secretsmodel.SecretsSource
	err = db.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, "active").
		Order("priority ASC, created_at ASC").
		First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.WithContext(ctx).Model(&secretsmodel.SecretsSource{}).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).
			Update("is_default", false).Error
	}
	if err != nil {
		return err
	}
	return r.setDefaultWithDB(ctx, db, candidate.ID)
}

func (r *SecretsSourceRepository) UpdateTestResult(ctx context.Context, id uuid.UUID, success bool) error {
	result := TenantDB(r.db, ctx).Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_test_at":     gorm.Expr("NOW()"),
			"last_test_result": success,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SecretsSourceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	result := TenantDB(r.db, ctx).Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SecretsSourceRepository) UpdateTestTime(ctx context.Context, id uuid.UUID) error {
	result := TenantDB(r.db, ctx).Model(&secretsmodel.SecretsSource{}).
		Where("id = ?", id).
		Update("last_test_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *SecretsSourceRepository) CountTasksUsingSource(ctx context.Context, sourceID string) (int64, error) {
	var count int64
	err := countSecretsSourceUsage(TenantDB(r.db, ctx).Model(&automationmodel.ExecutionTask{}), r.db.Dialector.Name(), sourceID, &count)
	return count, err
}

func (r *SecretsSourceRepository) CountSchedulesUsingSource(ctx context.Context, sourceID string) (int64, error) {
	var count int64
	err := countSecretsSourceUsage(TenantDB(r.db, ctx).Model(&automationmodel.ExecutionSchedule{}), r.db.Dialector.Name(), sourceID, &count)
	return count, err
}

func (r *SecretsSourceRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	var total int64
	if err := newDB().Model(&secretsmodel.SecretsSource{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	newDB().Model(&secretsmodel.SecretsSource{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	type TypeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var typeCounts []TypeCount
	newDB().Model(&secretsmodel.SecretsSource{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&typeCounts)
	stats["by_type"] = typeCounts

	return stats, nil
}

func marshalSecretsSourceReference(sourceID string) ([]byte, error) {
	return json.Marshal([]string{sourceID})
}

func countSecretsSourceUsage(query *gorm.DB, dialectName, sourceID string, count *int64) error {
	switch dialectName {
	case "sqlite":
		return query.
			Where("EXISTS (SELECT 1 FROM json_each(secrets_source_ids) WHERE json_each.value = ?)", sourceID).
			Count(count).Error
	default:
		payload, err := marshalSecretsSourceReference(sourceID)
		if err != nil {
			return err
		}
		return query.Where("secrets_source_ids @> ?", payload).Count(count).Error
	}
}
