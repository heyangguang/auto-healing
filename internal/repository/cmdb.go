package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrCMDBItemNotFound = errors.New("配置项不存在")
)

// CMDBItemRepository CMDB 配置项仓库
type CMDBItemRepository struct {
	db *gorm.DB
}

// NewCMDBItemRepository 创建 CMDB 仓库
func NewCMDBItemRepository() *CMDBItemRepository {
	return &CMDBItemRepository{
		db: database.DB,
	}
}

// Create 创建配置项
func (r *CMDBItemRepository) Create(ctx context.Context, item *model.CMDBItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// GetByID 根据 ID 获取配置项
func (r *CMDBItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CMDBItem, error) {
	var item model.CMDBItem
	err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCMDBItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

// Update 更新配置项
func (r *CMDBItemRepository) Update(ctx context.Context, item *model.CMDBItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// UpsertByExternalID 根据外部 ID 创建或更新配置项
// 返回: (isNew, error) - isNew=true 表示新增，false 表示更新
func (r *CMDBItemRepository) UpsertByExternalID(ctx context.Context, item *model.CMDBItem) (bool, error) {
	var existing model.CMDBItem
	err := r.db.WithContext(ctx).
		Where("plugin_id = ? AND external_id = ?", item.PluginID, item.ExternalID).
		First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 新增
			if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}

	// 更新现有记录
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return false, err
	}
	return false, nil
}

// cmdbAllowedSortFields CMDB 可排序字段白名单
var cmdbAllowedSortFields = map[string]bool{
	"name":               true,
	"type":               true,
	"status":             true,
	"environment":        true,
	"os":                 true,
	"owner":              true,
	"department":         true,
	"source_plugin_name": true,
	"updated_at":         true,
	"created_at":         true,
}

// List 获取配置项列表（支持过滤和排序）
func (r *CMDBItemRepository) List(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, hasPlugin *bool, sortBy, sortOrder string) ([]model.CMDBItem, int64, error) {
	var items []model.CMDBItem
	var total int64

	query := r.db.WithContext(ctx).Model(&model.CMDBItem{})

	if pluginID != nil {
		query = query.Where("plugin_id = ?", *pluginID)
	}
	// 筛选有/无关联插件的配置项
	if hasPlugin != nil {
		if *hasPlugin {
			query = query.Where("plugin_id IS NOT NULL")
		} else {
			query = query.Where("plugin_id IS NULL")
		}
	}
	if itemType != "" {
		query = query.Where("type = ?", itemType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if environment != "" {
		query = query.Where("environment = ?", environment)
	}
	if sourcePluginName != "" {
		query = query.Where("LOWER(source_plugin_name) LIKE LOWER(?)", "%"+sourcePluginName+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序：白名单校验防注入，默认 updated_at DESC
	orderField := "updated_at"
	orderDir := "DESC"
	if sortBy != "" && cmdbAllowedSortFields[sortBy] {
		orderField = sortBy
	}
	if sortOrder == "asc" || sortOrder == "ASC" {
		orderDir = "ASC"
	}

	offset := (page - 1) * pageSize
	if err := query.Order(fmt.Sprintf("%s %s", orderField, orderDir)).Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetStats 获取统计信息
func (r *CMDBItemRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.CMDBItem{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按类型统计
	type TypeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var typeCounts []TypeCount
	r.db.WithContext(ctx).Model(&model.CMDBItem{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&typeCounts)
	stats["by_type"] = typeCounts

	// 按状态统计
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	r.db.WithContext(ctx).Model(&model.CMDBItem{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	// 按环境统计
	type EnvCount struct {
		Environment string `json:"environment"`
		Count       int64  `json:"count"`
	}
	var envCounts []EnvCount
	r.db.WithContext(ctx).Model(&model.CMDBItem{}).
		Select("environment, count(*) as count").
		Group("environment").
		Scan(&envCounts)
	stats["by_environment"] = envCounts

	// 统计维护中数量
	var maintenanceCount int64
	r.db.WithContext(ctx).Model(&model.CMDBItem{}).
		Where("status = ?", "maintenance").
		Count(&maintenanceCount)
	stats["maintenance_count"] = maintenanceCount

	return stats, nil
}

// EnterMaintenance 进入维护模式
func (r *CMDBItemRepository) EnterMaintenance(ctx context.Context, id uuid.UUID, reason string, endAt *time.Time) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":               "maintenance",
		"maintenance_reason":   reason,
		"maintenance_start_at": &now,
		"maintenance_end_at":   endAt,
	}
	return r.db.WithContext(ctx).Model(&model.CMDBItem{}).Where("id = ?", id).Updates(updates).Error
}

// ExitMaintenance 退出维护模式
func (r *CMDBItemRepository) ExitMaintenance(ctx context.Context, id uuid.UUID) error {
	updates := map[string]interface{}{
		"status":               "active",
		"maintenance_reason":   "",
		"maintenance_start_at": nil,
		"maintenance_end_at":   nil,
	}
	return r.db.WithContext(ctx).Model(&model.CMDBItem{}).Where("id = ?", id).Updates(updates).Error
}

// GetExpiredMaintenanceItems 获取维护到期的配置项
func (r *CMDBItemRepository) GetExpiredMaintenanceItems(ctx context.Context) ([]model.CMDBItem, error) {
	var items []model.CMDBItem
	err := r.db.WithContext(ctx).
		Where("status = ? AND maintenance_end_at IS NOT NULL AND maintenance_end_at <= ?", "maintenance", time.Now()).
		Find(&items).Error
	return items, err
}

// CreateMaintenanceLog 创建维护日志
func (r *CMDBItemRepository) CreateMaintenanceLog(ctx context.Context, log *model.CMDBMaintenanceLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListMaintenanceLogs 获取维护日志
func (r *CMDBItemRepository) ListMaintenanceLogs(ctx context.Context, cmdbItemID uuid.UUID, page, pageSize int) ([]model.CMDBMaintenanceLog, int64, error) {
	var logs []model.CMDBMaintenanceLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.CMDBMaintenanceLog{}).Where("cmdb_item_id = ?", cmdbItemID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// FindByNameOrIP 根据名称、主机名或 IP 地址查找配置项
// 用于 CMDB 验证节点
func (r *CMDBItemRepository) FindByNameOrIP(ctx context.Context, identifier string) (*model.CMDBItem, error) {
	var item model.CMDBItem
	err := r.db.WithContext(ctx).
		Where("name = ? OR hostname = ? OR ip_address = ?", identifier, identifier, identifier).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCMDBItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

// FindActiveByNameOrIP 根据名称、主机名或 IP 地址查找活跃的配置项
func (r *CMDBItemRepository) FindActiveByNameOrIP(ctx context.Context, identifier string) (*model.CMDBItem, error) {
	var item model.CMDBItem
	err := r.db.WithContext(ctx).
		Where("(name = ? OR hostname = ? OR ip_address = ?) AND status = ?",
			identifier, identifier, identifier, "active").
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCMDBItemNotFound
		}
		return nil, err
	}
	return &item, nil
}
