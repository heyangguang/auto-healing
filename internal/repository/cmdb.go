package repository

import (
	"context"
	"errors"
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
	if err := FillTenantID(ctx, &item.TenantID); err != nil {
		return err
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
	return UpdateTenantScopedModel(r.db, ctx, item.ID, item)
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
			// 新增时补齐 tenant_id，避免写入成功但租户查询不可见
			if err := FillTenantID(ctx, &item.TenantID); err != nil {
				return false, err
			}
			if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}

	// 更新现有记录
	if err := r.updateFromSync(ctx, existing, item); err != nil {
		return false, err
	}
	return false, nil
}

func (r *CMDBItemRepository) updateFromSync(ctx context.Context, existing model.CMDBItem, item *model.CMDBItem) error {
	return TenantDB(r.db, ctx).Model(&model.CMDBItem{}).Where("id = ?", existing.ID).Updates(buildCMDBSyncUpdates(existing, item)).Error
}

func buildCMDBSyncUpdates(existing model.CMDBItem, item *model.CMDBItem) map[string]any {
	updates := map[string]any{
		"plugin_id":          item.PluginID,
		"source_plugin_name": item.SourcePluginName,
		"name":               item.Name,
		"type":               item.Type,
		"ip_address":         item.IPAddress,
		"hostname":           item.Hostname,
		"os":                 item.OS,
		"os_version":         item.OSVersion,
		"cpu":                item.CPU,
		"memory":             item.Memory,
		"disk":               item.Disk,
		"location":           item.Location,
		"owner":              item.Owner,
		"environment":        item.Environment,
		"manufacturer":       item.Manufacturer,
		"model":              item.Model,
		"serial_number":      item.SerialNumber,
		"department":         item.Department,
		"dependencies":       item.Dependencies,
		"tags":               item.Tags,
		"raw_data":           item.RawData,
		"source_created_at":  item.SourceCreatedAt,
		"source_updated_at":  item.SourceUpdatedAt,
		"updated_at":         time.Now(),
	}
	if existing.Status != "maintenance" {
		updates["status"] = item.Status
	}
	return updates
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
