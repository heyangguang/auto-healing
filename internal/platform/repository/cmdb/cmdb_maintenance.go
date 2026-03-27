package cmdb

import (
	"context"
	"errors"
	"time"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnterMaintenance 进入维护模式
func (r *CMDBItemRepository) EnterMaintenance(ctx context.Context, id uuid.UUID, reason string, endAt *time.Time) error {
	now := time.Now()
	return TenantDB(r.db, ctx).Model(&platformmodel.CMDBItem{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":               "maintenance",
		"maintenance_reason":   reason,
		"maintenance_start_at": &now,
		"maintenance_end_at":   endAt,
	}).Error
}

// ExitMaintenance 退出维护模式
func (r *CMDBItemRepository) ExitMaintenance(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&platformmodel.CMDBItem{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":               "active",
		"maintenance_reason":   "",
		"maintenance_start_at": nil,
		"maintenance_end_at":   nil,
	}).Error
}

// GetExpiredMaintenanceItems 获取维护到期的配置项（跨租户，调度器专用）
func (r *CMDBItemRepository) GetExpiredMaintenanceItems(ctx context.Context) ([]platformmodel.CMDBItem, error) {
	var items []platformmodel.CMDBItem
	err := r.db.WithContext(ctx).
		Where("status = ? AND maintenance_end_at IS NOT NULL AND maintenance_end_at <= ?", "maintenance", time.Now()).
		Find(&items).Error
	return items, err
}

// CreateMaintenanceLog 创建维护日志
func (r *CMDBItemRepository) CreateMaintenanceLog(ctx context.Context, log *platformmodel.CMDBMaintenanceLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// ListMaintenanceLogs 获取维护日志
func (r *CMDBItemRepository) ListMaintenanceLogs(ctx context.Context, cmdbItemID uuid.UUID, page, pageSize int) ([]platformmodel.CMDBMaintenanceLog, int64, error) {
	var logs []platformmodel.CMDBMaintenanceLog
	var total int64
	query := TenantDB(r.db, ctx).Model(&platformmodel.CMDBMaintenanceLog{}).Where("cmdb_item_id = ?", cmdbItemID)
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// FindByNameOrIP 根据名称、主机名或 IP 地址查找配置项
func (r *CMDBItemRepository) FindByNameOrIP(ctx context.Context, identifier string) (*platformmodel.CMDBItem, error) {
	return r.findByNameOrIP(ctx, identifier, "")
}

// FindActiveByNameOrIP 根据名称、主机名或 IP 地址查找活跃的配置项
func (r *CMDBItemRepository) FindActiveByNameOrIP(ctx context.Context, identifier string) (*platformmodel.CMDBItem, error) {
	return r.findByNameOrIP(ctx, identifier, "active")
}

func (r *CMDBItemRepository) findByNameOrIP(ctx context.Context, identifier, status string) (*platformmodel.CMDBItem, error) {
	var item platformmodel.CMDBItem
	query := TenantDB(r.db, ctx).Where("name = ? OR hostname = ? OR ip_address = ?", identifier, identifier, identifier)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCMDBItemNotFound
		}
		return nil, err
	}
	return &item, nil
}
