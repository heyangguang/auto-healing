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
	if plugin.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		plugin.TenantID = &tenantID
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

// Update 更新插件
func (r *PluginRepository) Update(ctx context.Context, plugin *model.Plugin) error {
	return r.db.WithContext(ctx).Save(plugin).Error
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
		if err := tx.Model(&model.Incident{}).Where("plugin_id = ?", id).Updates(map[string]interface{}{
			"source_plugin_name": plugin.Name + " (已删除)",
			"plugin_id":          nil,
		}).Error; err != nil {
			return err
		}

		// 2. 解除 CMDB 配置项与插件的关联（保留数据）
		if err := tx.Model(&model.CMDBItem{}).Where("plugin_id = ?", id).Updates(map[string]interface{}{
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
		"status": status,
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
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
	if log.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		log.TenantID = &tenantID
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
	return r.db.WithContext(ctx).Save(log).Error
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

// IncidentRepository 工单/事件仓库
type IncidentRepository struct {
	db *gorm.DB
}

// NewIncidentRepository 创建工单仓库
func NewIncidentRepository() *IncidentRepository {
	return &IncidentRepository{db: database.DB}
}

// Create 创建工单
func (r *IncidentRepository) Create(ctx context.Context, incident *model.Incident) error {
	if incident.TenantID == nil {
		tenantID := TenantIDFromContext(ctx)
		incident.TenantID = &tenantID
	}
	return r.db.WithContext(ctx).Create(incident).Error
}

// GetByID 根据 ID 获取工单
func (r *IncidentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Incident, error) {
	var incident model.Incident
	err := TenantDB(r.db, ctx).Preload("Plugin").First(&incident, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("工单不存在")
	}
	return &incident, err
}

// Update 更新工单
func (r *IncidentRepository) Update(ctx context.Context, incident *model.Incident) error {
	return r.db.WithContext(ctx).Save(incident).Error
}

// List 获取工单列表（支持查询已删除插件的工单）
// hasPlugin: nil=不筛选, true=只有关联插件, false=只无关联插件
func (r *IncidentRepository) List(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, status, healingStatus, severity string, sourcePluginName, search query.StringFilter, hasPlugin *bool, sortBy, sortOrder string, externalID query.StringFilter, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Incident, int64, error) {
	var incidents []model.Incident
	var total int64

	q := TenantDB(r.db, ctx).Model(&model.Incident{})

	if pluginID != nil {
		q = q.Where("plugin_id = ?", *pluginID)
	}
	// 筛选有/无关联插件的工单
	if hasPlugin != nil {
		if *hasPlugin {
			q = q.Where("plugin_id IS NOT NULL")
		} else {
			q = q.Where("plugin_id IS NULL")
		}
	}
	// 来源插件名称过滤
	if !sourcePluginName.IsEmpty() {
		q = query.ApplyStringFilter(q, "source_plugin_name", sourcePluginName)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if healingStatus != "" {
		q = q.Where("healing_status = ?", healingStatus)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if !search.IsEmpty() {
		q = query.ApplyMultiStringFilter(q, []string{"title", "external_id", "description"}, search)
	}
	if !externalID.IsEmpty() {
		q = query.ApplyStringFilter(q, "external_id", externalID)
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
		"title": true, "severity": true, "status": true,
		"healing_status": true, "category": true, "assignee": true,
		"external_id": true, "source_plugin_name": true,
		"created_at": true, "updated_at": true,
	}
	if sortBy != "" && allowedSortFields[sortBy] {
		sortField = sortBy
	}
	if sortOrder == "asc" || sortOrder == "ASC" {
		order = "ASC"
	}

	offset := (page - 1) * pageSize
	err := q.Preload("Plugin").Offset(offset).Limit(pageSize).Order(fmt.Sprintf("%s %s", sortField, order)).Find(&incidents).Error
	return incidents, total, err
}

// UpsertByExternalID 根据外部 ID 创建或更新工单
// 返回: (isNew, error) - isNew=true 表示新增，false 表示更新
func (r *IncidentRepository) UpsertByExternalID(ctx context.Context, incident *model.Incident) (bool, error) {
	var existing model.Incident
	err := TenantDB(r.db, ctx).Where("plugin_id = ? AND external_id = ?", incident.PluginID, incident.ExternalID).First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 不存在，创建新工单
		return true, r.Create(ctx, incident)
	}

	if err != nil {
		return false, err
	}

	// 存在，更新
	incident.ID = existing.ID
	return false, r.Update(ctx, incident)
}

// ListUnscanned 获取未扫描的工单列表（用于自愈引擎调度）
func (r *IncidentRepository) ListUnscanned(ctx context.Context, limit int) ([]model.Incident, error) {
	var incidents []model.Incident
	err := TenantDB(r.db, ctx).
		Where("scanned = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&incidents).Error
	return incidents, err
}

// ListPendingTrigger 获取待手动触发的工单列表（Manual规则匹配但未创建流程实例）
// 用于待办中心的"待触发工单"标签页
// 支持搜索和过滤：title（模糊匹配 title, external_id, affected_ci）、severity、dateFrom、dateTo
func (r *IncidentRepository) ListPendingTrigger(ctx context.Context, page, pageSize int, title, severity, dateFrom, dateTo string) ([]model.Incident, int64, error) {
	var incidents []model.Incident
	var total int64

	// 筛选条件：
	// 1. scanned = true (已扫描)
	// 2. matched_rule_id IS NOT NULL (匹配了规则)
	// 3. healing_flow_instance_id IS NULL (未创建流程实例，说明是Manual模式)
	query := TenantDB(r.db, ctx).Model(&model.Incident{}).
		Where("scanned = ?", true).
		Where("matched_rule_id IS NOT NULL").
		Where("healing_flow_instance_id IS NULL").
		Where("healing_status NOT IN ?", []string{"skipped", "dismissed"})

	// 模糊搜索：title, external_id, affected_ci
	if title != "" {
		searchPattern := "%" + title + "%"
		query = query.Where("(title ILIKE ? OR external_id ILIKE ? OR affected_ci ILIKE ?)", searchPattern, searchPattern, searchPattern)
	}

	// 严重级别过滤
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}

	// 日期范围过滤
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom+" 00:00:00")
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("Plugin").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&incidents).Error

	return incidents, total, err
}

// ListDismissedTrigger 获取已忽略的手动触发工单列表
// 用于待办中心的"已忽略"标签页
func (r *IncidentRepository) ListDismissedTrigger(ctx context.Context, page, pageSize int, title, severity, dateFrom, dateTo string) ([]model.Incident, int64, error) {
	var incidents []model.Incident
	var total int64

	// 筛选条件：
	// 1. scanned = true (已扫描)
	// 2. matched_rule_id IS NOT NULL (匹配了规则)
	// 3. healing_status = 'skipped' (已忽略)
	query := TenantDB(r.db, ctx).Model(&model.Incident{}).
		Where("scanned = ?", true).
		Where("matched_rule_id IS NOT NULL").
		Where("healing_status = ?", "dismissed")

	// 模糊搜索：title, external_id, affected_ci
	if title != "" {
		searchPattern := "%" + title + "%"
		query = query.Where("(title ILIKE ? OR external_id ILIKE ? OR affected_ci ILIKE ?)", searchPattern, searchPattern, searchPattern)
	}

	// 严重级别过滤
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}

	// 日期范围过滤
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom+" 00:00:00")
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Preload("Plugin").
		Offset(offset).
		Limit(pageSize).
		Order("updated_at DESC").
		Find(&incidents).Error

	return incidents, total, err
}

// MarkScanned 标记工单为已扫描
func (r *IncidentRepository) MarkScanned(ctx context.Context, id uuid.UUID, matchedRuleID *uuid.UUID, flowInstanceID *uuid.UUID) error {
	updates := map[string]interface{}{
		"scanned": true,
	}
	if matchedRuleID != nil {
		updates["matched_rule_id"] = *matchedRuleID
	}
	if flowInstanceID != nil {
		updates["healing_flow_instance_id"] = *flowInstanceID
	}
	return TenantDB(r.db, ctx).Model(&model.Incident{}).Where("id = ?", id).Updates(updates).Error
}

// ResetScan 重置工单扫描状态
func (r *IncidentRepository) ResetScan(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&model.Incident{}).Where("id = ?", id).Updates(map[string]interface{}{
		"scanned":                  false,
		"matched_rule_id":          nil,
		"healing_flow_instance_id": nil,
		"healing_status":           "pending",
	}).Error
}

// BatchResetScan 批量重置工单扫描状态
// ids 为空时表示重置所有符合条件的工单
func (r *IncidentRepository) BatchResetScan(ctx context.Context, ids []uuid.UUID, healingStatus string) (int64, error) {
	query := TenantDB(r.db, ctx).Model(&model.Incident{})

	// 如果指定了 ID 列表
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}

	// 如果指定了自愈状态筛选
	if healingStatus != "" {
		query = query.Where("healing_status = ?", healingStatus)
	}

	result := query.Updates(map[string]interface{}{
		"scanned":                  false,
		"matched_rule_id":          nil,
		"healing_flow_instance_id": nil,
		"healing_status":           "pending",
	})

	return result.RowsAffected, result.Error
}

// IncidentStats 工单统计数据
type IncidentStats struct {
	Total      int64 `json:"total"`
	Scanned    int64 `json:"scanned"`
	Unscanned  int64 `json:"unscanned"`
	Matched    int64 `json:"matched"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Healed     int64 `json:"healed"`
	Failed     int64 `json:"failed"`
	Skipped    int64 `json:"skipped"`
	Dismissed  int64 `json:"dismissed"`
}

// GetStats 获取工单统计数据
func (r *IncidentRepository) GetStats(ctx context.Context) (*IncidentStats, error) {
	stats := &IncidentStats{}

	// 每次查询使用新的 TenantDB 实例，避免 GORM session WHERE 条件累积
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	// 总数
	if err := newDB().Model(&model.Incident{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// 已扫描
	if err := newDB().Model(&model.Incident{}).Where("scanned = ?", true).Count(&stats.Scanned).Error; err != nil {
		return nil, err
	}

	// 待扫描
	stats.Unscanned = stats.Total - stats.Scanned

	// 已匹配规则
	if err := newDB().Model(&model.Incident{}).Where("matched_rule_id IS NOT NULL").Count(&stats.Matched).Error; err != nil {
		return nil, err
	}

	// 按 healing_status 统计
	if err := newDB().Model(&model.Incident{}).Where("healing_status = ?", "pending").Count(&stats.Pending).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.Incident{}).Where("healing_status = ?", "processing").Count(&stats.Processing).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.Incident{}).Where("healing_status = ?", "healed").Count(&stats.Healed).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.Incident{}).Where("healing_status = ?", "failed").Count(&stats.Failed).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.Incident{}).Where("healing_status = ?", "skipped").Count(&stats.Skipped).Error; err != nil {
		return nil, err
	}
	if err := newDB().Model(&model.Incident{}).Where("healing_status = ?", "dismissed").Count(&stats.Dismissed).Error; err != nil {
		return nil, err
	}

	return stats, nil
}
