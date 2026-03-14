package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
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
	if item.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		item.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(item).Error
}

// GetByID 根据 ID 获取配置项
func (r *CMDBItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CMDBItem, error) {
	var item model.CMDBItem
	err := TenantDB(r.db, ctx).First(&item, "id = ?", id).Error
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
	err := TenantDB(r.db, ctx).
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
func (r *CMDBItemRepository) List(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, search query.StringFilter, hasPlugin *bool, sortBy, sortOrder string, scopes ...func(*gorm.DB) *gorm.DB) ([]model.CMDBItem, int64, error) {
	var items []model.CMDBItem
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.CMDBItem{})
	q = applyCMDBFilters(q, pluginID, itemType, status, environment, sourcePluginName, search, hasPlugin)
	for _, scope := range scopes {
		q = scope(q)
	}

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
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
	if err := q.Order(fmt.Sprintf("%s %s", orderField, orderDir)).Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CMDBItemBasic CMDB 配置项轻量信息（仅用于 ListIDs 返回）
type CMDBItemBasic struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Hostname  string    `json:"hostname"`
	IPAddress string    `json:"ip_address" gorm:"column:ip_address"`
	Status    string    `json:"status"`
}

// ListIDs 获取符合筛选条件的配置项 ID 列表（轻量接口，用于全选）
func (r *CMDBItemRepository) ListIDs(ctx context.Context, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, hasPlugin *bool) ([]CMDBItemBasic, int64, error) {
	var items []CMDBItemBasic
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.CMDBItem{})
	q = applyCMDBFilters(q, pluginID, itemType, status, environment, sourcePluginName, query.StringFilter{}, hasPlugin)

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := q.Select("id, name, hostname, ip_address, status").Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// applyCMDBFilters 通用 CMDB 筛选条件（List 和 ListIDs 共用）
func applyCMDBFilters(q *gorm.DB, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, search query.StringFilter, hasPlugin *bool) *gorm.DB {
	if pluginID != nil {
		q = q.Where("plugin_id = ?", *pluginID)
	}
	if hasPlugin != nil {
		if *hasPlugin {
			q = q.Where("plugin_id IS NOT NULL")
		} else {
			q = q.Where("plugin_id IS NULL")
		}
	}
	if itemType != "" {
		q = q.Where("type = ?", itemType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if environment != "" {
		q = q.Where("environment = ?", environment)
	}
	if sourcePluginName != "" {
		q = q.Where("LOWER(source_plugin_name) LIKE LOWER(?)", "%"+sourcePluginName+"%")
	}
	if !search.IsEmpty() {
		q = query.ApplyMultiStringFilter(q, []string{"name", "hostname", "ip_address"}, search)
	}
	return q
}

// GetStats 获取统计信息
func (r *CMDBItemRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	// 总数
	var total int64
	if err := newDB().Model(&model.CMDBItem{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 按类型统计
	type TypeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var typeCounts []TypeCount
	newDB().Model(&model.CMDBItem{}).
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
	newDB().Model(&model.CMDBItem{}).
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
	newDB().Model(&model.CMDBItem{}).
		Select("environment, count(*) as count").
		Group("environment").
		Scan(&envCounts)
	stats["by_environment"] = envCounts

	// 统计维护中数量
	var maintenanceCount int64
	newDB().Model(&model.CMDBItem{}).
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
	return TenantDB(r.db, ctx).Model(&model.CMDBItem{}).Where("id = ?", id).Updates(updates).Error
}

// ExitMaintenance 退出维护模式
func (r *CMDBItemRepository) ExitMaintenance(ctx context.Context, id uuid.UUID) error {
	updates := map[string]interface{}{
		"status":               "active",
		"maintenance_reason":   "",
		"maintenance_start_at": nil,
		"maintenance_end_at":   nil,
	}
	return TenantDB(r.db, ctx).Model(&model.CMDBItem{}).Where("id = ?", id).Updates(updates).Error
}

// GetExpiredMaintenanceItems 获取维护到期的配置项（跨租户，调度器专用）
// 注意：不使用 TenantDB，调度器需要处理所有租户的维护到期项
func (r *CMDBItemRepository) GetExpiredMaintenanceItems(ctx context.Context) ([]model.CMDBItem, error) {
	var items []model.CMDBItem
	err := r.db.WithContext(ctx).
		Where("status = ? AND maintenance_end_at IS NOT NULL AND maintenance_end_at <= ?", "maintenance", time.Now()).
		Find(&items).Error
	return items, err
}

// CreateMaintenanceLog 创建维护日志
func (r *CMDBItemRepository) CreateMaintenanceLog(ctx context.Context, log *model.CMDBMaintenanceLog) error {
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// ListMaintenanceLogs 获取维护日志
func (r *CMDBItemRepository) ListMaintenanceLogs(ctx context.Context, cmdbItemID uuid.UUID, page, pageSize int) ([]model.CMDBMaintenanceLog, int64, error) {
	var logs []model.CMDBMaintenanceLog
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.CMDBMaintenanceLog{}).Where("cmdb_item_id = ?", cmdbItemID)
	query.Session(&gorm.Session{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// FindByNameOrIP 根据名称、主机名或 IP 地址查找配置项
// 用于 CMDB 验证节点
func (r *CMDBItemRepository) FindByNameOrIP(ctx context.Context, identifier string) (*model.CMDBItem, error) {
	var item model.CMDBItem
	err := TenantDB(r.db, ctx).
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
	err := TenantDB(r.db, ctx).
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
