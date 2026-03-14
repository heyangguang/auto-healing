package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPluginNameExists = errors.New("插件名称已存在")
	ErrInvalidConfig    = errors.New("插件配置无效")
)

// Service 插件服务
type Service struct {
	pluginRepo  *repository.PluginRepository
	syncLogRepo *repository.PluginSyncLogRepository
	cmdbRepo    *repository.CMDBItemRepository
	httpClient  *HTTPClient
}

// NewService 创建插件服务
func NewService() *Service {
	return &Service{
		pluginRepo:  repository.NewPluginRepository(),
		syncLogRepo: repository.NewPluginSyncLogRepository(),
		cmdbRepo:    repository.NewCMDBItemRepository(),
		httpClient:  NewHTTPClient(),
	}
}

// CreatePlugin 创建插件
func (s *Service) CreatePlugin(ctx context.Context, plugin *model.Plugin) (*model.Plugin, error) {
	// 检查名称是否已存在
	exists, err := s.pluginRepo.ExistsByName(ctx, plugin.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPluginNameExists
	}

	// 验证同步间隔（最小1分钟）
	if plugin.SyncEnabled && plugin.SyncIntervalMinutes < 1 {
		return nil, errors.New("同步间隔最小为1分钟")
	}

	// 计算下次同步时间
	if plugin.SyncEnabled && plugin.SyncIntervalMinutes > 0 {
		t := time.Now().Add(time.Duration(plugin.SyncIntervalMinutes) * time.Minute)
		plugin.NextSyncAt = &t
	}

	if err := s.pluginRepo.Create(ctx, plugin); err != nil {
		return nil, err
	}

	return plugin, nil
}

// GetPlugin 获取插件
func (s *Service) GetPlugin(ctx context.Context, id uuid.UUID) (*model.Plugin, error) {
	return s.pluginRepo.GetByID(ctx, id)
}

// UpdatePlugin 更新插件
func (s *Service) UpdatePlugin(ctx context.Context, id uuid.UUID, description, version string, config, fieldMapping, syncFilter model.JSON, syncEnabled *bool, syncIntervalMinutes *int) (*model.Plugin, error) {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if description != "" {
		plugin.Description = description
	}
	if version != "" {
		plugin.Version = version
	}
	if config != nil {
		plugin.Config = config
	}
	if fieldMapping != nil {
		plugin.FieldMapping = fieldMapping
	}
	if syncFilter != nil {
		plugin.SyncFilter = syncFilter
	}
	if syncEnabled != nil {
		plugin.SyncEnabled = *syncEnabled
	}
	if syncIntervalMinutes != nil {
		plugin.SyncIntervalMinutes = *syncIntervalMinutes
	}

	// 重新计算下次同步时间
	if plugin.SyncEnabled && plugin.SyncIntervalMinutes > 0 {
		t := time.Now().Add(time.Duration(plugin.SyncIntervalMinutes) * time.Minute)
		plugin.NextSyncAt = &t
	} else {
		plugin.NextSyncAt = nil
	}

	if err := s.pluginRepo.Update(ctx, plugin); err != nil {
		return nil, err
	}

	return plugin, nil
}

// DeletePlugin 删除插件
func (s *Service) DeletePlugin(ctx context.Context, id uuid.UUID) error {
	return s.pluginRepo.Delete(ctx, id)
}

// ListPlugins 获取插件列表
func (s *Service) ListPlugins(ctx context.Context, page, pageSize int, pluginType, status string, search query.StringFilter, sortBy, sortOrder string, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Plugin, int64, error) {
	return s.pluginRepo.List(ctx, page, pageSize, pluginType, status, search, sortBy, sortOrder, scopes...)
}

// PluginStats 插件统计数据
type PluginStats struct {
	Total         int64            `json:"total"`          // 总数
	ByType        map[string]int64 `json:"by_type"`        // 按类型分布
	ByStatus      map[string]int64 `json:"by_status"`      // 按状态分布
	SyncEnabled   int64            `json:"sync_enabled"`   // 启用同步数
	SyncDisabled  int64            `json:"sync_disabled"`  // 未启用同步数
	ActiveCount   int64            `json:"active_count"`   // 激活数
	InactiveCount int64            `json:"inactive_count"` // 未激活数
	ErrorCount    int64            `json:"error_count"`    // 错误数
}

// GetStats 获取插件统计数据
func (s *Service) GetStats(ctx context.Context) (*PluginStats, error) {
	// 获取全部插件（不分页）
	plugins, _, err := s.pluginRepo.List(ctx, 1, 10000, "", "", query.StringFilter{}, "", "")
	if err != nil {
		return nil, err
	}

	stats := &PluginStats{
		Total:    int64(len(plugins)),
		ByType:   make(map[string]int64),
		ByStatus: make(map[string]int64),
	}

	for _, p := range plugins {
		// 按类型统计
		stats.ByType[p.Type]++

		// 按状态统计
		stats.ByStatus[p.Status]++

		// 状态详细统计
		switch p.Status {
		case "active":
			stats.ActiveCount++
		case "inactive":
			stats.InactiveCount++
		case "error":
			stats.ErrorCount++
		}

		// 同步配置统计
		if p.SyncEnabled {
			stats.SyncEnabled++
		} else {
			stats.SyncDisabled++
		}
	}

	return stats, nil
}

// TestConnection 测试插件连接（只测试，不改变状态）
func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) error {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 直接发送 HTTP 请求测试连接
	return s.httpClient.TestConnection(ctx, plugin.Config)
}

// Activate 激活插件（测试成功后才激活）
func (s *Service) Activate(ctx context.Context, id uuid.UUID) error {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 先测试连接
	if err := s.httpClient.TestConnection(ctx, plugin.Config); err != nil {
		s.pluginRepo.UpdateStatus(ctx, id, "error", err.Error())
		return fmt.Errorf("连接测试失败: %w", err)
	}

	// 测试成功，激活插件
	return s.pluginRepo.UpdateStatus(ctx, id, "active", "")
}

// Deactivate 停用插件（直接停用，不需要测试）
func (s *Service) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return s.pluginRepo.UpdateStatus(ctx, id, "inactive", "")
}

// TriggerSync 触发真实同步 (异步版本，用于 API 调用)
func (s *Service) TriggerSync(ctx context.Context, id uuid.UUID) (*model.PluginSyncLog, error) {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 创建同步日志
	syncLog := &model.PluginSyncLog{
		PluginID: plugin.ID,
		SyncType: "manual",
		Status:   "running",
	}

	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		return nil, err
	}

	// 启动异步真实同步任务（使用独立 context 避免 HTTP 请求取消，并注入插件租户 ID 确保数据写入正确租户）
	asyncCtx := context.Background()
	if plugin.TenantID != nil {
		asyncCtx = repository.WithTenantID(asyncCtx, *plugin.TenantID)
	}
	go s.performSync(asyncCtx, plugin, syncLog)

	return syncLog, nil
}

// TriggerSyncSync 触发真实同步 (同步版本，用于调度器)
func (s *Service) TriggerSyncSync(ctx context.Context, id uuid.UUID) (*model.PluginSyncLog, error) {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 创建同步日志
	syncLog := &model.PluginSyncLog{
		PluginID: plugin.ID,
		SyncType: "scheduled",
		Status:   "running",
	}

	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		return nil, err
	}

	// 同步执行
	s.performSync(ctx, plugin, syncLog)

	return syncLog, nil
}

// performSync 执行真实的数据同步
func (s *Service) performSync(ctx context.Context, plugin *model.Plugin, syncLog *model.PluginSyncLog) {
	// panic 保护：防止 panic 导致同步日志永远停留在 running 状态
	defer func() {
		if rec := recover(); rec != nil {
			logger.Sync_("PLUGIN").Error("performSync panic: %v", rec)
			s.updateSyncLogError(ctx, syncLog, fmt.Sprintf("内部错误: %v", rec))
		}
	}()

	// 确定同步起始时间
	since := time.Now().Add(-24 * time.Hour) // 默认拉取最近24小时
	if plugin.LastSyncAt != nil {
		since = *plugin.LastSyncAt
	}

	processedCount := 0
	failedCount := 0
	newCount := 0
	updatedCount := 0
	fetchedCount := 0
	filteredCount := 0
	filteredRecords := []map[string]interface{}{} // 被筛选掉的记录详情

	// 根据插件类型拉取不同数据
	switch plugin.Type {
	case "itsm":
		// 拉取工单数据
		rawData, err := s.httpClient.FetchData(ctx, plugin.Config, since)
		if err != nil {
			s.updateSyncLogError(ctx, syncLog, fmt.Sprintf("拉取工单失败: %v", err))
			return
		}

		// 应用字段映射，转换为标准格式
		incidents := s.mapToIncidents(rawData, plugin.FieldMapping)
		fetchedCount = len(incidents)

		// 解析过滤器
		filter, _ := ParseSyncFilter(plugin.SyncFilter)

		// 处理工单数据
		for _, raw := range incidents {
			// 应用过滤器
			if filter != nil {
				matched, reason := ApplyFilterWithReason(filter, raw.RawData)
				if !matched {
					filteredCount++
					// 记录被筛选的原因（最多记录20条）
					if len(filteredRecords) < 20 {
						filteredRecords = append(filteredRecords, map[string]interface{}{
							"external_id": raw.ExternalID,
							"title":       raw.Title,
							"reason":      reason,
						})
					}
					continue
				}
			}

			isNew, err := s.saveIncident(ctx, plugin.ID, plugin.Name, raw)
			if err != nil {
				failedCount++
				continue
			}
			processedCount++
			if isNew {
				newCount++
			} else {
				updatedCount++
			}
		}

	case "cmdb":
		// 拉取 CMDB 数据
		rawData, err := s.httpClient.FetchData(ctx, plugin.Config, since)
		if err != nil {
			s.updateSyncLogError(ctx, syncLog, fmt.Sprintf("拉取CMDB数据失败: %v", err))
			return
		}

		// 应用字段映射，转换为标准格式
		cmdbItems := s.mapToCMDBItems(rawData, plugin.FieldMapping)
		fetchedCount = len(cmdbItems)

		// 解析过滤器
		filter, _ := ParseSyncFilter(plugin.SyncFilter)

		// 处理 CMDB 数据
		for _, raw := range cmdbItems {
			// 应用过滤器
			if filter != nil {
				matched, reason := ApplyFilterWithReason(filter, raw.RawData)
				if !matched {
					filteredCount++
					if len(filteredRecords) < 20 {
						filteredRecords = append(filteredRecords, map[string]interface{}{
							"external_id": raw.ExternalID,
							"name":        raw.Name,
							"reason":      reason,
						})
					}
					continue
				}
			}

			isNew, err := s.saveCMDBItem(ctx, plugin.ID, plugin.Name, raw)
			if err != nil {
				failedCount++
				continue
			}
			processedCount++
			if isNew {
				newCount++
			} else {
				updatedCount++
			}
		}

	default:
		s.updateSyncLogError(ctx, syncLog, fmt.Sprintf("不支持的插件类型: %s", plugin.Type))
		return
	}

	// 更新同步日志
	syncLog.Status = "success"
	syncLog.RecordsFetched = fetchedCount
	syncLog.RecordsFiltered = filteredCount
	syncLog.RecordsProcessed = processedCount
	syncLog.RecordsNew = newCount
	syncLog.RecordsUpdated = updatedCount
	syncLog.RecordsFailed = failedCount
	syncLog.Details = model.JSON{
		"filtered_records": filteredRecords,
	}
	now := time.Now()
	syncLog.CompletedAt = &now

	if err := s.syncLogRepo.Update(ctx, syncLog); err != nil {
		logger.Sync_("PLUGIN").Error("更新同步日志失败: %v", err)
	}

	// 输出汇总日志
	duration := now.Sub(syncLog.StartedAt)
	logger.Sync_("PLUGIN").Info("完成: %s | 获取: %d条 | 筛选: %d条 | 新增: %d条 | 更新: %d条 | 失败: %d条 | 耗时: %v",
		plugin.Name, fetchedCount, filteredCount, newCount, updatedCount, failedCount, duration)

	// 更新插件的最后同步时间
	if err := s.pluginRepo.UpdateSyncInfo(ctx, plugin.ID, &now, nil); err != nil {
		logger.Sync_("PLUGIN").Error("更新插件同步信息失败: %v", err)
	}
}

// updateSyncLogError 更新同步日志为失败状态
func (s *Service) updateSyncLogError(ctx context.Context, syncLog *model.PluginSyncLog, errMsg string) {
	syncLog.Status = "failed"
	syncLog.ErrorMessage = errMsg
	now := time.Now()
	syncLog.CompletedAt = &now
	_ = s.syncLogRepo.Update(ctx, syncLog)
}

// RawIncident 原始工单数据
type RawIncident struct {
	ExternalID      string
	Title           string
	Description     string
	Severity        string
	Priority        string
	Status          string
	Category        string
	AffectedCI      string
	AffectedService string
	Assignee        string
	Reporter        string
	SourceCreatedAt time.Time
	SourceUpdatedAt time.Time
	RawData         map[string]interface{}
}

// RawCMDBItem 原始 CMDB 配置项数据
type RawCMDBItem struct {
	ExternalID      string
	Name            string
	Type            string
	Status          string
	IPAddress       string
	Hostname        string
	OS              string
	OSVersion       string
	CPU             string
	Memory          string
	Disk            string
	Location        string
	Owner           string
	Environment     string
	Manufacturer    string
	Model           string
	SerialNumber    string
	Department      string
	SourceCreatedAt time.Time
	SourceUpdatedAt time.Time
	Dependencies    []string
	Tags            map[string]string
	RawData         map[string]interface{}
}

// mapToIncidents 将原始数据按字段映射转换为工单格式
func (s *Service) mapToIncidents(rawData []map[string]interface{}, fieldMapping model.JSON) []RawIncident {
	var incidents []RawIncident

	// 获取工单字段映射
	incidentMapping := make(map[string]string)
	if mapping, ok := fieldMapping["incident_mapping"].(map[string]interface{}); ok {
		for k, v := range mapping {
			if vs, ok := v.(string); ok {
				incidentMapping[k] = vs
			}
		}
	}

	for _, data := range rawData {
		incident := RawIncident{
			ExternalID:      getStringField(data, incidentMapping, "external_id", "id"),
			Title:           getStringField(data, incidentMapping, "title", "title"),
			Description:     getStringField(data, incidentMapping, "description", "description"),
			Severity:        getStringField(data, incidentMapping, "severity", "severity"),
			Priority:        getStringField(data, incidentMapping, "priority", "priority"),
			Status:          getStringField(data, incidentMapping, "status", "status"),
			Category:        getStringField(data, incidentMapping, "category", "category"),
			AffectedCI:      getStringField(data, incidentMapping, "affected_ci", "affected_ci"),
			AffectedService: getStringField(data, incidentMapping, "affected_service", "affected_service"),
			Assignee:        getStringField(data, incidentMapping, "assignee", "assignee"),
			Reporter:        getStringField(data, incidentMapping, "reporter", "reporter"),
			RawData:         data,
		}
		incidents = append(incidents, incident)
	}

	return incidents
}

// mapToCMDBItems 将原始数据按字段映射转换为 CMDB 格式
func (s *Service) mapToCMDBItems(rawData []map[string]interface{}, fieldMapping model.JSON) []RawCMDBItem {
	var items []RawCMDBItem

	// 获取 CMDB 字段映射
	cmdbMapping := make(map[string]string)
	if mapping, ok := fieldMapping["cmdb_mapping"].(map[string]interface{}); ok {
		for k, v := range mapping {
			if vs, ok := v.(string); ok {
				cmdbMapping[k] = vs
			}
		}
	}

	for _, data := range rawData {
		item := RawCMDBItem{
			ExternalID:   getStringField(data, cmdbMapping, "external_id", "id"),
			Name:         getStringField(data, cmdbMapping, "name", "name"),
			Type:         getStringField(data, cmdbMapping, "type", "type"),
			Status:       getStringField(data, cmdbMapping, "status", "status"),
			IPAddress:    getStringField(data, cmdbMapping, "ip_address", "ip_address"),
			Hostname:     getStringField(data, cmdbMapping, "hostname", "hostname"),
			OS:           getStringField(data, cmdbMapping, "os", "os"),
			OSVersion:    getStringField(data, cmdbMapping, "os_version", "os_version"),
			CPU:          getStringField(data, cmdbMapping, "cpu", "cpu"),
			Memory:       getStringField(data, cmdbMapping, "memory", "memory"),
			Disk:         getStringField(data, cmdbMapping, "disk", "disk"),
			Location:     getStringField(data, cmdbMapping, "location", "location"),
			Owner:        getStringField(data, cmdbMapping, "owner", "owner"),
			Environment:  getStringField(data, cmdbMapping, "environment", "environment"),
			Manufacturer: getStringField(data, cmdbMapping, "manufacturer", "manufacturer"),
			Model:        getStringField(data, cmdbMapping, "model", "model"),
			SerialNumber: getStringField(data, cmdbMapping, "serial_number", "serial_number"),
			Department:   getStringField(data, cmdbMapping, "department", "department"),
			RawData:      data,
		}
		items = append(items, item)
	}

	return items
}

// getStringField 从数据中获取字段值（支持字段映射）
func getStringField(data map[string]interface{}, mapping map[string]string, fieldName, defaultField string) string {
	// 优先使用映射的字段名
	actualField := defaultField
	if mapped, ok := mapping[fieldName]; ok && mapped != "" {
		actualField = mapped
	}

	if val, ok := data[actualField]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// saveIncident 保存工单到数据库
// 返回: (isNew, error) - isNew=true 表示新增
func (s *Service) saveIncident(ctx context.Context, pluginID uuid.UUID, pluginName string, raw RawIncident) (bool, error) {
	incidentRepo := repository.NewIncidentRepository()

	incident := &model.Incident{
		PluginID:         &pluginID,
		SourcePluginName: pluginName,
		ExternalID:       raw.ExternalID,
		Title:            raw.Title,
		Description:      raw.Description,
		Severity:         raw.Severity,
		Priority:         raw.Priority,
		Status:           raw.Status,
		Category:         raw.Category,
		AffectedCI:       raw.AffectedCI,
		AffectedService:  raw.AffectedService,
		Assignee:         raw.Assignee,
		Reporter:         raw.Reporter,
		HealingStatus:    "pending",
		RawData:          raw.RawData,
	}

	// 使用 UpsertByExternalID 避免重复
	return incidentRepo.UpsertByExternalID(ctx, incident)
}

// saveCMDBItem 保存 CMDB 配置项到数据库
// 返回: (isNew, error) - isNew=true 表示新增
func (s *Service) saveCMDBItem(ctx context.Context, pluginID uuid.UUID, pluginName string, raw RawCMDBItem) (bool, error) {
	// 处理时间
	var srcCreated, srcUpdated *time.Time
	if !raw.SourceCreatedAt.IsZero() {
		srcCreated = &raw.SourceCreatedAt
	}
	if !raw.SourceUpdatedAt.IsZero() {
		srcUpdated = &raw.SourceUpdatedAt
	}

	// 转换标签
	tags := model.JSON{}
	for k, v := range raw.Tags {
		tags[k] = v
	}

	// 转换依赖（存储为 JSON 数组）
	deps := make(model.JSONArray, len(raw.Dependencies))
	for i, d := range raw.Dependencies {
		deps[i] = d
	}

	item := &model.CMDBItem{
		PluginID:         &pluginID,
		SourcePluginName: pluginName,
		ExternalID:       raw.ExternalID,
		Name:             raw.Name,
		Type:             raw.Type,
		Status:           raw.Status,
		IPAddress:        raw.IPAddress,
		Hostname:         raw.Hostname,
		OS:               raw.OS,
		OSVersion:        raw.OSVersion,
		CPU:              raw.CPU,
		Memory:           raw.Memory,
		Disk:             raw.Disk,
		Location:         raw.Location,
		Owner:            raw.Owner,
		Environment:      raw.Environment,
		Manufacturer:     raw.Manufacturer,
		Model:            raw.Model,
		SerialNumber:     raw.SerialNumber,
		Department:       raw.Department,
		Dependencies:     deps,
		Tags:             tags,
		RawData:          raw.RawData,
		SourceCreatedAt:  srcCreated,
		SourceUpdatedAt:  srcUpdated,
	}

	return s.cmdbRepo.UpsertByExternalID(ctx, item)
}

// GetSyncLogs 获取同步日志
func (s *Service) GetSyncLogs(ctx context.Context, pluginID uuid.UUID, page, pageSize int) ([]model.PluginSyncLog, int64, error) {
	return s.syncLogRepo.ListByPluginID(ctx, pluginID, page, pageSize)
}

// IncidentService 工单服务
type IncidentService struct {
	incidentRepo *repository.IncidentRepository
	pluginRepo   *repository.PluginRepository
	httpClient   *HTTPClient
}

// NewIncidentService 创建工单服务
func NewIncidentService() *IncidentService {
	return &IncidentService{
		incidentRepo: repository.NewIncidentRepository(),
		pluginRepo:   repository.NewPluginRepository(),
		httpClient:   NewHTTPClient(),
	}
}

// GetIncident 获取工单
func (s *IncidentService) GetIncident(ctx context.Context, id uuid.UUID) (*model.Incident, error) {
	return s.incidentRepo.GetByID(ctx, id)
}

// ListIncidents 获取工单列表（支持查询已删除插件的工单）
// hasPlugin: nil=不筛选, true=只有关联插件, false=只无关联插件
func (s *IncidentService) ListIncidents(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, status, healingStatus, severity string, sourcePluginName, search query.StringFilter, hasPlugin *bool, sortBy, sortOrder string, externalID query.StringFilter, scopes ...func(*gorm.DB) *gorm.DB) ([]model.Incident, int64, error) {
	return s.incidentRepo.List(ctx, page, pageSize, pluginID, status, healingStatus, severity, sourcePluginName, search, hasPlugin, sortBy, sortOrder, externalID, scopes...)
}

// CloseIncidentResponse 关闭工单响应
type CloseIncidentResponse struct {
	Message       string `json:"message"`
	LocalStatus   string `json:"local_status"`
	SourceUpdated bool   `json:"source_updated"`
}

// CloseIncident 关闭工单
func (s *IncidentService) CloseIncident(ctx context.Context, id uuid.UUID, resolution, workNotes, closeCode, closeStatus string) (*CloseIncidentResponse, error) {
	// 1. 获取工单信息
	incident, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取工单失败: %w", err)
	}

	// 默认状态
	if closeStatus == "" {
		closeStatus = "resolved"
	}

	// 2. 尝试回写到源系统（如果有关联插件且配置了回写接口）
	sourceUpdated := false
	if incident.PluginID != nil {
		plugin, err := s.pluginRepo.GetByID(ctx, *incident.PluginID)
		if err == nil && plugin != nil {
			// 检查是否配置了关闭工单的接口
			if closeURL, ok := plugin.Config["close_incident_url"].(string); ok && closeURL != "" {
				closeReq := map[string]interface{}{
					"external_id":  incident.ExternalID,
					"resolution":   resolution,
					"work_notes":   workNotes,
					"close_code":   closeCode,
					"close_status": closeStatus,
				}
				if err := s.httpClient.CloseIncident(ctx, plugin.Config, closeReq); err != nil {
					// 回写失败只记录日志，不阻断流程
					logger.Sync_("PLUGIN").Warn("回写工单到源系统失败: incident_id=%s, external_id=%s, error=%s",
						id, incident.ExternalID, err.Error(),
					)
				} else {
					sourceUpdated = true
					logger.Sync_("PLUGIN").Info("成功回写工单到源系统: incident_id=%s, external_id=%s",
						id, incident.ExternalID,
					)
				}
			}
		}
	}

	// 3. 更新本地工单状态
	incident.Status = closeStatus
	incident.HealingStatus = "healed"
	if err := s.incidentRepo.Update(ctx, incident); err != nil {
		return nil, fmt.Errorf("更新本地工单状态失败: %w", err)
	}

	return &CloseIncidentResponse{
		Message:       "工单已关闭",
		LocalStatus:   "healed",
		SourceUpdated: sourceUpdated,
	}, nil
}

// ResetScan 重置工单扫描状态
// 将工单的 scanned 设为 false，清除 matched_rule_id 和 healing_flow_instance_id
// 这样工单会被自愈调度器重新扫描
func (s *IncidentService) ResetScan(ctx context.Context, id uuid.UUID) error {
	// 先检查工单是否存在
	_, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("工单不存在: %w", err)
	}

	// 重置扫描状态
	return s.incidentRepo.ResetScan(ctx, id)
}

// BatchResetScanRequest 批量重置请求
type BatchResetScanRequest struct {
	IDs           []uuid.UUID `json:"ids"`            // 指定工单 ID 列表（为空时按条件筛选）
	HealingStatus string      `json:"healing_status"` // 按自愈状态筛选（如 failed）
}

// BatchResetScanResponse 批量重置响应
type BatchResetScanResponse struct {
	AffectedCount int64  `json:"affected_count"`
	Message       string `json:"message"`
}

// BatchResetScan 批量重置工单扫描状态
func (s *IncidentService) BatchResetScan(ctx context.Context, ids []uuid.UUID, healingStatus string) (*BatchResetScanResponse, error) {
	count, err := s.incidentRepo.BatchResetScan(ctx, ids, healingStatus)
	if err != nil {
		return nil, fmt.Errorf("批量重置失败: %w", err)
	}

	return &BatchResetScanResponse{
		AffectedCount: count,
		Message:       fmt.Sprintf("成功重置 %d 条工单的扫描状态", count),
	}, nil
}
