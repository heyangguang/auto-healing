package plugin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/query"
	sharedrepo "github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPluginNameExists = errors.New("插件名称已存在")
	ErrInvalidConfig    = errors.New("插件配置无效")
)

// Service 插件服务
type Service struct {
	pluginRepo  *integrationrepo.PluginRepository
	syncLogRepo *integrationrepo.PluginSyncLogRepository
	cmdbRepo    *sharedrepo.CMDBItemRepository
	httpClient  *HTTPClient
	lifecycle   *asyncLifecycle
}

// NewService 创建插件服务
func NewService() *Service {
	return &Service{
		pluginRepo:  integrationrepo.NewPluginRepository(),
		syncLogRepo: integrationrepo.NewPluginSyncLogRepository(),
		cmdbRepo:    sharedrepo.NewCMDBItemRepository(),
		httpClient:  NewHTTPClient(),
		lifecycle:   newAsyncLifecycle(),
	}
}

// CreatePlugin 创建插件
func (s *Service) CreatePlugin(ctx context.Context, plugin *model.Plugin) (*model.Plugin, error) {
	exists, err := s.pluginRepo.ExistsByName(ctx, plugin.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPluginNameExists
	}
	if err := validatePluginMutation(plugin.Type, plugin.SyncEnabled, plugin.SyncIntervalMinutes, plugin.MaxFailures); err != nil {
		return nil, err
	}

	plugin.NextSyncAt = calculateNextSyncAt(plugin.SyncEnabled, plugin.SyncIntervalMinutes)
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
func (s *Service) UpdatePlugin(ctx context.Context, id uuid.UUID, description, version string, config, fieldMapping, syncFilter model.JSON, syncEnabled *bool, syncIntervalMinutes, maxFailures *int) (*model.Plugin, error) {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	applyPluginUpdates(plugin, description, version, config, fieldMapping, syncFilter, syncEnabled, syncIntervalMinutes, maxFailures)
	if err := validatePluginMutation(plugin.Type, plugin.SyncEnabled, plugin.SyncIntervalMinutes, plugin.MaxFailures); err != nil {
		return nil, err
	}
	plugin.NextSyncAt = calculateNextSyncAt(plugin.SyncEnabled, plugin.SyncIntervalMinutes)

	if err := s.pluginRepo.UpdateConfig(ctx, plugin.ID, integrationrepo.PluginConfigUpdate{
		Description:         plugin.Description,
		Version:             plugin.Version,
		Config:              plugin.Config,
		FieldMapping:        plugin.FieldMapping,
		SyncFilter:          plugin.SyncFilter,
		SyncEnabled:         plugin.SyncEnabled,
		SyncIntervalMinutes: plugin.SyncIntervalMinutes,
		NextSyncAt:          plugin.NextSyncAt,
		MaxFailures:         plugin.MaxFailures,
	}); err != nil {
		return nil, err
	}
	return s.pluginRepo.GetByID(ctx, id)
}

func applyPluginUpdates(plugin *model.Plugin, description, version string, config, fieldMapping, syncFilter model.JSON, syncEnabled *bool, syncIntervalMinutes, maxFailures *int) {
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
	if maxFailures != nil {
		plugin.MaxFailures = *maxFailures
	}
}

func calculateNextSyncAt(enabled bool, intervalMinutes int) *time.Time {
	return calculateNextSyncAtFrom(time.Now(), enabled, intervalMinutes)
}

func calculateNextSyncAtFrom(base time.Time, enabled bool, intervalMinutes int) *time.Time {
	if !enabled || intervalMinutes <= 0 {
		return nil
	}
	next := base.Add(time.Duration(intervalMinutes) * time.Minute)
	return &next
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
	Total         int64            `json:"total"`
	ByType        map[string]int64 `json:"by_type"`
	ByStatus      map[string]int64 `json:"by_status"`
	SyncEnabled   int64            `json:"sync_enabled"`
	SyncDisabled  int64            `json:"sync_disabled"`
	ActiveCount   int64            `json:"active_count"`
	InactiveCount int64            `json:"inactive_count"`
	ErrorCount    int64            `json:"error_count"`
}

// GetStats 获取插件统计数据
func (s *Service) GetStats(ctx context.Context) (*PluginStats, error) {
	aggregate, err := s.pluginRepo.GetAggregateStats(ctx)
	if err != nil {
		return nil, err
	}

	stats := &PluginStats{
		Total:        aggregate.Total,
		ByType:       aggregate.ByType,
		ByStatus:     aggregate.ByStatus,
		SyncEnabled:  aggregate.SyncEnabled,
		SyncDisabled: aggregate.SyncDisabled,
	}
	stats.ActiveCount = aggregate.ByStatus["active"]
	stats.InactiveCount = aggregate.ByStatus["inactive"]
	stats.ErrorCount = aggregate.ByStatus["error"]
	return stats, nil
}

// TestConnection 测试插件连接（只测试，不改变状态）
func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) error {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.httpClient.TestConnection(ctx, plugin.Config)
}

// Activate 激活插件（测试成功后才激活）
func (s *Service) Activate(ctx context.Context, id uuid.UUID) error {
	plugin, err := s.pluginRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.httpClient.TestConnection(ctx, plugin.Config); err != nil {
		if updateErr := s.pluginRepo.UpdateStatus(ctx, id, "error", err.Error()); updateErr != nil {
			return errors.Join(fmt.Errorf("连接测试失败: %w", err), fmt.Errorf("更新插件错误状态失败: %w", updateErr))
		}
		return fmt.Errorf("连接测试失败: %w", err)
	}
	return s.pluginRepo.UpdateStatus(ctx, id, "active", "")
}

// Deactivate 停用插件（直接停用，不需要测试）
func (s *Service) Deactivate(ctx context.Context, id uuid.UUID) error {
	if _, err := s.pluginRepo.GetByID(ctx, id); err != nil {
		return err
	}
	return s.pluginRepo.UpdateStatus(ctx, id, "inactive", "")
}

// GetSyncLogs 获取同步日志
func (s *Service) GetSyncLogs(ctx context.Context, pluginID uuid.UUID, page, pageSize int) ([]model.PluginSyncLog, int64, error) {
	return s.syncLogRepo.ListByPluginID(ctx, pluginID, page, pageSize)
}
