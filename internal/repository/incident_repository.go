package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	if err := FillTenantID(ctx, &incident.TenantID); err != nil {
		return err
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
	return UpdateTenantScopedModel(r.db, ctx, incident.ID, incident)
}

// UpdateHealingStatus 仅更新工单自愈状态，避免整行覆盖并发修改。
func (r *IncidentRepository) UpdateHealingStatus(ctx context.Context, id uuid.UUID, healingStatus string) error {
	return TenantDB(r.db, ctx).
		Model(&model.Incident{}).
		Where("id = ?", id).
		Update("healing_status", healingStatus).
		Error
}

// List 获取工单列表
func (r *IncidentRepository) List(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, status, healingStatus, severity string, sourcePluginName, search query.StringFilter, hasPlugin *bool, sortBy, sortOrder string, externalID query.StringFilter, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Incident, int64, error) {
	var incidents []model.Incident
	var total int64
	q := TenantDB(r.db, ctx).Model(&model.Incident{})
	q = applyIncidentFilters(q, pluginID, status, healingStatus, severity, sourcePluginName, search, hasPlugin, externalID)
	for _, scope := range scopes {
		q = scope(q)
	}
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Preload("Plugin").Offset((page - 1) * pageSize).Limit(pageSize).Order(incidentOrderClause(sortBy, sortOrder)).Find(&incidents).Error
	return incidents, total, err
}

func applyIncidentFilters(q *gorm.DB, pluginID *uuid.UUID, status, healingStatus, severity string, sourcePluginName, search query.StringFilter, hasPlugin *bool, externalID query.StringFilter) *gorm.DB {
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
	return q
}

func incidentOrderClause(sortBy, sortOrder string) string {
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
	return fmt.Sprintf("%s %s", sortField, order)
}

// UpsertByExternalID 根据外部 ID 创建或更新工单
func (r *IncidentRepository) UpsertByExternalID(ctx context.Context, incident *model.Incident) (bool, error) {
	var existing model.Incident
	err := TenantDB(r.db, ctx).Where("plugin_id = ? AND external_id = ?", incident.PluginID, incident.ExternalID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, r.Create(ctx, incident)
	}
	if err != nil {
		return false, err
	}
	preserveIncidentManagedFields(incident, &existing)
	return false, r.Update(ctx, incident)
}

func preserveIncidentManagedFields(incoming, existing *model.Incident) {
	incoming.ID = existing.ID
	incoming.TenantID = existing.TenantID
	incoming.HealingStatus = existing.HealingStatus
	incoming.WorkflowInstanceID = existing.WorkflowInstanceID
	incoming.Scanned = existing.Scanned
	incoming.MatchedRuleID = existing.MatchedRuleID
	incoming.HealingFlowInstanceID = existing.HealingFlowInstanceID
}

// ListUnscanned 获取未扫描的工单列表（跨租户，自愈引擎调度器专用）
func (r *IncidentRepository) ListUnscanned(ctx context.Context, limit int) ([]model.Incident, error) {
	var incidents []model.Incident
	err := r.db.WithContext(ctx).Where("scanned = ?", false).Order("created_at ASC").Limit(limit).Find(&incidents).Error
	return incidents, err
}
