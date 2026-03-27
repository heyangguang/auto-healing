package plugin

import (
	"context"
	"errors"
	"fmt"

	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrBatchResetScanScopeRequired = errors.New("批量重置必须提供 ids 或 healing_status")

// IncidentService 工单服务
type IncidentService struct {
	incidentRepo *incidentrepo.IncidentRepository
	pluginRepo   *integrationrepo.PluginRepository
	httpClient   *HTTPClient
}

// NewIncidentService 创建工单服务
func NewIncidentService() *IncidentService {
	return &IncidentService{
		incidentRepo: incidentrepo.NewIncidentRepository(),
		pluginRepo:   integrationrepo.NewPluginRepository(),
		httpClient:   NewHTTPClient(),
	}
}

// GetIncident 获取工单
func (s *IncidentService) GetIncident(ctx context.Context, id uuid.UUID) (*platformmodel.Incident, error) {
	return s.incidentRepo.GetByID(ctx, id)
}

// ListIncidents 获取工单列表（支持查询已删除插件的工单）
func (s *IncidentService) ListIncidents(ctx context.Context, page, pageSize int, pluginID *uuid.UUID, status, healingStatus, severity string, sourcePluginName, search query.StringFilter, hasPlugin *bool, sortBy, sortOrder string, externalID query.StringFilter, scopes ...func(*gorm.DB) *gorm.DB) ([]platformmodel.Incident, int64, error) {
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
	incident, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取工单失败: %w", err)
	}
	if closeStatus == "" {
		closeStatus = "resolved"
	}

	sourceUpdated, err := s.writeBackIncidentClose(ctx, id, incident, resolution, workNotes, closeCode, closeStatus)
	if err != nil {
		return nil, err
	}
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

func (s *IncidentService) writeBackIncidentClose(ctx context.Context, id uuid.UUID, incident *platformmodel.Incident, resolution, workNotes, closeCode, closeStatus string) (bool, error) {
	if incident.PluginID == nil {
		return false, nil
	}
	plugin, err := s.pluginRepo.GetByID(ctx, *incident.PluginID)
	if err != nil {
		if errors.Is(err, integrationrepo.ErrPluginNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("获取来源插件失败: %w", err)
	}
	if plugin == nil {
		return false, nil
	}

	closeURL, ok := plugin.Config["close_incident_url"].(string)
	if !ok || closeURL == "" {
		return false, nil
	}

	req := map[string]any{
		"external_id":  incident.ExternalID,
		"resolution":   resolution,
		"work_notes":   workNotes,
		"close_code":   closeCode,
		"close_status": closeStatus,
	}
	if err := s.httpClient.CloseIncident(ctx, plugin.Config, req); err != nil {
		logger.Sync_("PLUGIN").Warn("回写工单到源系统失败: incident_id=%s, external_id=%s, error=%s", id, incident.ExternalID, err.Error())
		return false, fmt.Errorf("回写工单到源系统失败: %w", err)
	}

	logger.Sync_("PLUGIN").Info("成功回写工单到源系统: incident_id=%s, external_id=%s", id, incident.ExternalID)
	return true, nil
}

// ResetScan 重置工单扫描状态
func (s *IncidentService) ResetScan(ctx context.Context, id uuid.UUID) error {
	if _, err := s.incidentRepo.GetByID(ctx, id); err != nil {
		return fmt.Errorf("获取工单失败: %w", err)
	}
	return s.incidentRepo.ResetScan(ctx, id)
}

// BatchResetScanResponse 批量重置响应
type BatchResetScanResponse struct {
	AffectedCount int64  `json:"affected_count"`
	Message       string `json:"message"`
}

// BatchResetScanRequest 批量重置请求
type BatchResetScanRequest struct {
	IDs           []uuid.UUID `json:"ids"`
	HealingStatus string      `json:"healing_status"`
}

// BatchResetScan 批量重置工单扫描状态
func (s *IncidentService) BatchResetScan(ctx context.Context, ids []uuid.UUID, healingStatus string) (*BatchResetScanResponse, error) {
	if len(ids) == 0 && healingStatus == "" {
		return nil, ErrBatchResetScanScopeRequired
	}
	count, err := s.incidentRepo.BatchResetScan(ctx, ids, healingStatus)
	if err != nil {
		return nil, fmt.Errorf("批量重置失败: %w", err)
	}
	return &BatchResetScanResponse{
		AffectedCount: count,
		Message:       fmt.Sprintf("成功重置 %d 条工单的扫描状态", count),
	}, nil
}
