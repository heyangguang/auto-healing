package incident

import (
	"context"
	"fmt"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListPendingTrigger 获取待手动触发的工单列表
func (r *IncidentRepository) ListPendingTrigger(ctx context.Context, page, pageSize int, title, severity, dateFrom, dateTo string) ([]platformmodel.Incident, int64, error) {
	var incidents []platformmodel.Incident
	var total int64
	query := TenantDB(r.db, ctx).
		Model(&platformmodel.Incident{}).
		Where("scanned = ?", true).
		Where("matched_rule_id IS NOT NULL").
		Where("healing_flow_instance_id IS NULL").
		Where("healing_status NOT IN ?", []string{"skipped", "dismissed"})

	query = applyIncidentTriggerFilters(query, title, severity, dateFrom, dateTo)
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Plugin").Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&incidents).Error
	return incidents, total, err
}

// ListTriggerRecords 获取已经处理过的手动触发工单记录。
// 记录包含两类：人工忽略的工单，以及已经确认触发并生成自愈流程实例的工单。
func (r *IncidentRepository) ListTriggerRecords(ctx context.Context, page, pageSize int, title, severity, dateFrom, dateTo string) ([]platformmodel.Incident, int64, error) {
	var incidents []platformmodel.Incident
	var total int64
	query := TenantDB(r.db, ctx).
		Model(&platformmodel.Incident{}).
		Where("scanned = ?", true).
		Where("matched_rule_id IS NOT NULL").
		Where("(healing_status = ? OR healing_flow_instance_id IS NOT NULL)", "dismissed")

	query = applyIncidentTriggerFilters(query, title, severity, dateFrom, dateTo)
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Plugin").Offset((page - 1) * pageSize).Limit(pageSize).Order("updated_at DESC").Find(&incidents).Error
	return incidents, total, err
}

func applyIncidentTriggerFilters(query *gorm.DB, title, severity, dateFrom, dateTo string) *gorm.DB {
	if title != "" {
		pattern := "%" + title + "%"
		query = query.Where("(title ILIKE ? OR external_id ILIKE ? OR affected_ci ILIKE ?)", pattern, pattern, pattern)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom+" 00:00:00")
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}
	return query
}

// MarkScanned 标记工单为已扫描
func (r *IncidentRepository) MarkScanned(ctx context.Context, id uuid.UUID, matchedRuleID *uuid.UUID, flowInstanceID *uuid.UUID) error {
	updates := map[string]interface{}{"scanned": true}
	if matchedRuleID != nil {
		updates["matched_rule_id"] = *matchedRuleID
	}
	if flowInstanceID != nil {
		updates["healing_flow_instance_id"] = *flowInstanceID
	}
	return TenantDB(r.db, ctx).Model(&platformmodel.Incident{}).Where("id = ?", id).Updates(updates).Error
}

func (r *IncidentRepository) SyncState(ctx context.Context, opts IncidentSyncOptions) error {
	result := TenantDB(r.db, ctx).
		Model(&platformmodel.Incident{}).
		Where("id = ?", opts.IncidentID).
		Updates(incidentSyncUpdates(opts))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("工单不存在: %s", opts.IncidentID)
	}
	return nil
}

// ResetScan 重置工单扫描状态
func (r *IncidentRepository) ResetScan(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).Model(&platformmodel.Incident{}).Where("id = ?", id).Updates(resetIncidentScanUpdates()).Error
}

// RestorePendingTrigger 将已忽略的手动触发工单恢复到待触发池。
func (r *IncidentRepository) RestorePendingTrigger(ctx context.Context, id uuid.UUID) error {
	return TenantDB(r.db, ctx).
		Model(&platformmodel.Incident{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"healing_status":           "pending",
			"scanned":                  true,
			"healing_flow_instance_id": nil,
		}).Error
}

// BatchResetScan 批量重置工单扫描状态
func (r *IncidentRepository) BatchResetScan(ctx context.Context, ids []uuid.UUID, healingStatus string) (int64, error) {
	query := TenantDB(r.db, ctx).Model(&platformmodel.Incident{})
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}
	if healingStatus != "" {
		query = query.Where("healing_status = ?", healingStatus)
	}
	result := query.Updates(resetIncidentScanUpdates())
	return result.RowsAffected, result.Error
}

func resetIncidentScanUpdates() map[string]interface{} {
	return map[string]interface{}{
		"scanned":                  false,
		"matched_rule_id":          nil,
		"healing_flow_instance_id": nil,
		"healing_status":           "pending",
	}
}
