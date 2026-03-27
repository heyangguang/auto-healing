package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPluginNotFound = errors.New("插件不存在")
	ErrPluginExists   = errors.New("插件名称已存在")
)

// PluginRepository 插件数据仓库
type PluginRepository struct {
	db *gorm.DB
}

// NewPluginRepository 创建插件仓库
func NewPluginRepository() *PluginRepository {
	return &PluginRepository{db: database.DB}
}

// Create 创建插件
func (r *PluginRepository) Create(ctx context.Context, plugin *model.Plugin) error {
	if err := FillTenantID(ctx, &plugin.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(plugin).Error
}

// GetByID 根据 ID 获取插件
func (r *PluginRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Plugin, error) {
	var plugin model.Plugin
	err := TenantDB(r.db, ctx).First(&plugin, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPluginNotFound
	}
	return &plugin, err
}

// GetByName 根据名称获取插件
func (r *PluginRepository) GetByName(ctx context.Context, name string) (*model.Plugin, error) {
	var plugin model.Plugin
	err := TenantDB(r.db, ctx).First(&plugin, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPluginNotFound
	}
	return &plugin, err
}

// Delete 删除插件（工单保留并记录插件名称，同步日志级联删除）
func (r *PluginRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// 先获取插件信息
	var plugin model.Plugin
	if err := TenantDB(r.db, ctx).First(&plugin, "id = ?", id).Error; err != nil {
		return err
	}

	// 使用事务确保数据一致性
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 工单保留，设置 source_plugin_name 并解除关联
		if err := tx.Model(&platformmodel.Incident{}).Where("plugin_id = ?", id).Updates(map[string]interface{}{
			"source_plugin_name": plugin.Name + " (已删除)",
			"plugin_id":          nil,
		}).Error; err != nil {
			return err
		}

		// 2. 解除 CMDB 配置项与插件的关联（保留数据）
		if err := tx.Model(&platformmodel.CMDBItem{}).Where("plugin_id = ?", id).Updates(map[string]interface{}{
			"source_plugin_name": plugin.Name + " (已删除)",
			"plugin_id":          nil,
		}).Error; err != nil {
			return err
		}

		// 3. 级联删除同步日志
		if err := tx.Where("plugin_id = ?", id).Delete(&model.PluginSyncLog{}).Error; err != nil {
			return err
		}

		// 4. 最后删除插件本身
		return tx.Delete(&model.Plugin{}, "id = ?", id).Error
	})
}

// List 获取插件列表
func (r *PluginRepository) List(ctx context.Context, page, pageSize int, pluginType, status string, search query.StringFilter, sortBy, sortOrder string, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Plugin, int64, error) {
	var plugins []model.Plugin
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.Plugin{})

	if pluginType != "" {
		q = q.Where("type = ?", pluginType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if !search.IsEmpty() {
		q = query.ApplyMultiStringFilter(q, []string{"name", "description"}, search)
	}
	for _, scope := range scopes {
		q = scope(q)
	}

	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序（白名单校验防止 SQL 注入）
	sortField := "created_at"
	order := "DESC"
	allowedSortFields := map[string]bool{
		"name": true, "type": true, "status": true,
		"last_sync_at": true, "created_at": true, "updated_at": true,
	}
	if sortBy != "" && allowedSortFields[sortBy] {
		sortField = sortBy
	}
	if sortOrder == "asc" || sortOrder == "ASC" {
		order = "ASC"
	}

	offset := (page - 1) * pageSize
	err := q.Offset(offset).Limit(pageSize).Order(fmt.Sprintf("%s %s", sortField, order)).Find(&plugins).Error
	return plugins, total, err
}

// ExistsByName 检查插件名称是否存在
func (r *PluginRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	err := TenantDB(r.db, ctx).Model(&model.Plugin{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

// UpdateStatus 更新插件状态
func (r *PluginRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errorMsg,
	}
	return TenantDB(r.db, ctx).Model(&model.Plugin{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateSyncInfo 更新同步信息
func (r *PluginRepository) UpdateSyncInfo(ctx context.Context, id uuid.UUID, lastSyncAt, nextSyncAt *time.Time) error {
	updates := map[string]interface{}{}
	if lastSyncAt != nil {
		updates["last_sync_at"] = lastSyncAt
	}
	if nextSyncAt != nil {
		updates["next_sync_at"] = nextSyncAt
	}
	return TenantDB(r.db, ctx).Model(&model.Plugin{}).Where("id = ?", id).Updates(updates).Error
}

// PluginSyncLogRepository 插件同步日志仓库
type PluginSyncLogRepository struct {
	db *gorm.DB
}

// NewPluginSyncLogRepository 创建同步日志仓库
func NewPluginSyncLogRepository() *PluginSyncLogRepository {
	return &PluginSyncLogRepository{db: database.DB}
}

// Create 创建同步日志
func (r *PluginSyncLogRepository) Create(ctx context.Context, log *model.PluginSyncLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID 根据 ID 获取日志
func (r *PluginSyncLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PluginSyncLog, error) {
	var log model.PluginSyncLog
	err := TenantDB(r.db, ctx).Preload("Plugin").First(&log, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("同步日志不存在")
	}
	return &log, err
}

// Update 更新同步日志
func (r *PluginSyncLogRepository) Update(ctx context.Context, log *model.PluginSyncLog) error {
	return UpdateTenantScopedModel(r.db, ctx, log.ID, log)
}

// ListByPluginID 获取插件的同步日志
func (r *PluginSyncLogRepository) ListByPluginID(ctx context.Context, pluginID uuid.UUID, page, pageSize int) ([]model.PluginSyncLog, int64, error) {
	var logs []model.PluginSyncLog
	var total int64

	query := TenantDB(r.db, ctx).Model(&model.PluginSyncLog{}).Where("plugin_id = ?", pluginID)

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Preload("Plugin").Offset(offset).Limit(pageSize).Order("started_at DESC").Find(&logs).Error
	return logs, total, err
}
