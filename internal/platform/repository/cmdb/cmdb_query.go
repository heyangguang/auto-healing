package cmdb

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

	orderField := "updated_at"
	orderDir := "DESC"
	if sortBy != "" && cmdbAllowedSortFields[sortBy] {
		orderField = sortBy
	}
	if sortOrder == "asc" || sortOrder == "ASC" {
		orderDir = "ASC"
	}
	err := q.Order(fmt.Sprintf("%s %s", orderField, orderDir)).Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

// CMDBItemBasic CMDB 配置项轻量信息（仅用于 ListIDs 返回）
type CMDBItemBasic struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Hostname  string    `json:"hostname"`
	IPAddress string    `json:"ip_address" gorm:"column:ip_address"`
	Status    string    `json:"status"`
}

// ListIDs 获取符合筛选条件的配置项 ID 列表
func (r *CMDBItemRepository) ListIDs(ctx context.Context, pluginID *uuid.UUID, itemType, status, environment, sourcePluginName string, hasPlugin *bool) ([]CMDBItemBasic, int64, error) {
	var items []CMDBItemBasic
	var total int64
	q := TenantDB(r.db, ctx).Model(&model.CMDBItem{})
	q = applyCMDBFilters(q, pluginID, itemType, status, environment, sourcePluginName, query.StringFilter{}, hasPlugin)
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Select("id, name, hostname, ip_address, status").Order("updated_at DESC").Find(&items).Error
	return items, total, err
}

// GetStats 获取统计信息
func (r *CMDBItemRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	var total int64
	if err := newDB().Model(&model.CMDBItem{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	type typeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var typeCounts []typeCount
	if err := newDB().Model(&model.CMDBItem{}).Select("type, count(*) as count").Group("type").Scan(&typeCounts).Error; err != nil {
		return nil, err
	}
	stats["by_type"] = typeCounts

	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []statusCount
	if err := newDB().Model(&model.CMDBItem{}).Select("status, count(*) as count").Group("status").Scan(&statusCounts).Error; err != nil {
		return nil, err
	}
	stats["by_status"] = statusCounts

	type envCount struct {
		Environment string `json:"environment"`
		Count       int64  `json:"count"`
	}
	var envCounts []envCount
	if err := newDB().Model(&model.CMDBItem{}).Select("environment, count(*) as count").Group("environment").Scan(&envCounts).Error; err != nil {
		return nil, err
	}
	stats["by_environment"] = envCounts

	var maintenanceCount int64
	if err := newDB().Model(&model.CMDBItem{}).Where("status = ?", "maintenance").Count(&maintenanceCount).Error; err != nil {
		return nil, err
	}
	stats["maintenance_count"] = maintenanceCount
	return stats, nil
}
