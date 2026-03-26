package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

const maxFilteredRecordDetails = 20

type syncSummary struct {
	processedCount int
	failedCount    int
	newCount       int
	updatedCount   int
	fetchedCount   int
	filteredCount  int
	filtered       []map[string]any
}

// TriggerSync 触发真实同步 (异步版本，用于 API 调用)
func (s *Service) TriggerSync(ctx context.Context, id uuid.UUID) (*model.PluginSyncLog, error) {
	plugin, syncLog, err := s.prepareSync(ctx, id, "manual")
	if err != nil {
		return nil, err
	}

	s.Go(func(rootCtx context.Context) {
		s.performSync(withTenantContext(rootCtx, plugin.TenantID), plugin, syncLog)
	})
	return syncLog, nil
}

// TriggerSyncSync 触发真实同步 (同步版本，用于调度器)
func (s *Service) TriggerSyncSync(ctx context.Context, id uuid.UUID) (*model.PluginSyncLog, error) {
	plugin, syncLog, err := s.prepareSync(ctx, id, "scheduled")
	if err != nil {
		return nil, err
	}
	s.performSync(ctx, plugin, syncLog)
	return syncLog, nil
}

func (s *Service) prepareSync(ctx context.Context, id uuid.UUID, syncType string) (*model.Plugin, *model.PluginSyncLog, error) {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	syncLog := &model.PluginSyncLog{
		PluginID: plugin.ID,
		SyncType: syncType,
		Status:   "running",
	}
	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		return nil, nil, err
	}
	return plugin, syncLog, nil
}

// performSync 执行真实的数据同步
func (s *Service) performSync(ctx context.Context, plugin *model.Plugin, syncLog *model.PluginSyncLog) {
	defer s.recoverSyncPanic(ctx, syncLog)

	summary, err := s.runSync(ctx, plugin)
	if err != nil {
		if updateErr := s.updateSyncLogError(ctx, syncLog, err.Error()); updateErr != nil {
			logger.Sync_("PLUGIN").Error("更新失败同步日志失败: %v", updateErr)
		}
		return
	}

	s.completeSync(ctx, plugin, syncLog, summary)
}

func (s *Service) recoverSyncPanic(ctx context.Context, syncLog *model.PluginSyncLog) {
	if rec := recover(); rec != nil {
		logger.Sync_("PLUGIN").Error("performSync panic: %v", rec)
		if err := s.updateSyncLogError(ctx, syncLog, fmt.Sprintf("内部错误: %v", rec)); err != nil {
			logger.Sync_("PLUGIN").Error("更新 panic 同步日志失败: %v", err)
		}
	}
}

func (s *Service) runSync(ctx context.Context, plugin *model.Plugin) (*syncSummary, error) {
	since := syncSince(plugin)

	switch plugin.Type {
	case "itsm":
		return s.syncIncidents(ctx, plugin, since)
	case "cmdb":
		return s.syncCMDB(ctx, plugin, since)
	default:
		return nil, fmt.Errorf("不支持的插件类型: %s", plugin.Type)
	}
}

func syncSince(plugin *model.Plugin) time.Time {
	if plugin.LastSyncAt != nil {
		return *plugin.LastSyncAt
	}
	return time.Now().Add(-24 * time.Hour)
}

func (s *Service) syncIncidents(ctx context.Context, plugin *model.Plugin, since time.Time) (*syncSummary, error) {
	rawData, err := s.httpClient.FetchData(ctx, plugin.Config, since)
	if err != nil {
		return nil, fmt.Errorf("拉取工单失败: %v", err)
	}

	incidents := s.mapToIncidents(rawData, plugin.FieldMapping)
	filter, err := ParseSyncFilter(plugin.SyncFilter)
	if err != nil {
		return nil, fmt.Errorf("解析同步过滤器失败: %w", err)
	}
	summary := &syncSummary{fetchedCount: len(incidents)}
	for _, incident := range incidents {
		if summary.skipFilteredIncident(filter, incident) {
			continue
		}
		isNew, err := s.saveIncident(ctx, plugin.ID, plugin.Name, incident)
		summary.recordSaveResult(isNew, err)
	}
	return summary, nil
}

func (s *Service) syncCMDB(ctx context.Context, plugin *model.Plugin, since time.Time) (*syncSummary, error) {
	rawData, err := s.httpClient.FetchData(ctx, plugin.Config, since)
	if err != nil {
		return nil, fmt.Errorf("拉取CMDB数据失败: %v", err)
	}

	items := s.mapToCMDBItems(rawData, plugin.FieldMapping)
	filter, err := ParseSyncFilter(plugin.SyncFilter)
	if err != nil {
		return nil, fmt.Errorf("解析同步过滤器失败: %w", err)
	}
	summary := &syncSummary{fetchedCount: len(items)}
	for _, item := range items {
		if summary.skipFilteredCMDB(filter, item) {
			continue
		}
		isNew, err := s.saveCMDBItem(ctx, plugin.ID, plugin.Name, item)
		summary.recordSaveResult(isNew, err)
	}
	return summary, nil
}

func (s *Service) completeSync(ctx context.Context, plugin *model.Plugin, syncLog *model.PluginSyncLog, summary *syncSummary) {
	now := time.Now()
	syncLog.Status = summary.status()
	syncLog.ErrorMessage = summary.errorMessage()
	syncLog.RecordsFetched = summary.fetchedCount
	syncLog.RecordsFiltered = summary.filteredCount
	syncLog.RecordsProcessed = summary.processedCount
	syncLog.RecordsNew = summary.newCount
	syncLog.RecordsUpdated = summary.updatedCount
	syncLog.RecordsFailed = summary.failedCount
	syncLog.Details = model.JSON{
		"filtered_records": summary.filtered,
		"new_count":        summary.newCount,
		"updated_count":    summary.updatedCount,
	}
	syncLog.CompletedAt = &now

	if err := s.syncLogRepo.Update(ctx, syncLog); err != nil {
		logger.Sync_("PLUGIN").Error("更新同步日志失败: %v", err)
	}

	duration := now.Sub(syncLog.StartedAt)
	logger.Sync_("PLUGIN").Info("完成: %s | 获取: %d条 | 筛选: %d条 | 新增: %d条 | 更新: %d条 | 失败: %d条 | 耗时: %v",
		plugin.Name, summary.fetchedCount, summary.filteredCount, summary.newCount, summary.updatedCount, summary.failedCount, duration)

	if summary.failedCount == 0 {
		s.advancePluginSyncCursor(ctx, plugin.ID, now)
		return
	}
	logger.Sync_("PLUGIN").Warn("同步存在失败记录，跳过推进最后同步时间: %s | 失败: %d条", plugin.Name, summary.failedCount)
}

func (s *Service) advancePluginSyncCursor(ctx context.Context, pluginID uuid.UUID, syncedAt time.Time) {
	latestPlugin, err := s.pluginRepo.GetByID(ctx, pluginID)
	if err != nil {
		logger.Sync_("PLUGIN").Error("读取插件最新同步配置失败: %v", err)
		return
	}
	nextSyncAt := calculateNextSyncAtFrom(syncedAt, latestPlugin.SyncEnabled, latestPlugin.SyncIntervalMinutes)
	if err := s.pluginRepo.UpdateSyncInfo(ctx, pluginID, &syncedAt, nextSyncAt); err != nil {
		logger.Sync_("PLUGIN").Error("更新插件同步信息失败: %v", err)
	}
}

func (summary *syncSummary) status() string {
	if summary.failedCount > 0 {
		return "failed"
	}
	return "success"
}

func (summary *syncSummary) errorMessage() string {
	if summary.failedCount == 0 {
		return ""
	}
	return fmt.Sprintf("同步处理存在 %d 条失败记录", summary.failedCount)
}

func (summary *syncSummary) recordSaveResult(isNew bool, err error) {
	if err != nil {
		summary.failedCount++
		return
	}
	summary.processedCount++
	if isNew {
		summary.newCount++
		return
	}
	summary.updatedCount++
}

func (summary *syncSummary) skipFilteredIncident(filter *FilterCondition, raw RawIncident) bool {
	if filter == nil {
		return false
	}
	return summary.recordFilterResult(filter, raw.RawData, map[string]any{
		"external_id": raw.ExternalID,
		"title":       raw.Title,
	})
}

func (summary *syncSummary) skipFilteredCMDB(filter *FilterCondition, raw RawCMDBItem) bool {
	if filter == nil {
		return false
	}
	return summary.recordFilterResult(filter, raw.RawData, map[string]any{
		"external_id": raw.ExternalID,
		"name":        raw.Name,
	})
}

func (summary *syncSummary) recordFilterResult(filter *FilterCondition, data map[string]any, detail map[string]any) bool {
	matched, reason := ApplyFilterWithReason(filter, data)
	if matched {
		return false
	}

	summary.filteredCount++
	if len(summary.filtered) < maxFilteredRecordDetails {
		detail["reason"] = reason
		summary.filtered = append(summary.filtered, detail)
	}
	return true
}

// updateSyncLogError 更新同步日志为失败状态
func (s *Service) updateSyncLogError(ctx context.Context, syncLog *model.PluginSyncLog, errMsg string) error {
	syncLog.Status = "failed"
	syncLog.ErrorMessage = errMsg
	now := time.Now()
	syncLog.CompletedAt = &now
	return s.syncLogRepo.Update(ctx, syncLog)
}
