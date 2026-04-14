package plugin

import (
	"context"
	"errors"
	"fmt"

	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrBatchResetScanScopeRequired = errors.New("批量重置必须提供 ids 或 healing_status")

// IncidentService 工单服务
type IncidentService struct {
	incidentRepo     *incidentrepo.IncidentRepository
	writebackLogRepo *incidentrepo.IncidentWritebackLogRepository
	pluginRepo       *integrationrepo.PluginRepository
	httpClient       *HTTPClient
}

type IncidentServiceDeps struct {
	IncidentRepo     *incidentrepo.IncidentRepository
	WritebackLogRepo *incidentrepo.IncidentWritebackLogRepository
	PluginRepo       *integrationrepo.PluginRepository
	HTTPClient       *HTTPClient
}

func NewIncidentServiceWithDeps(deps IncidentServiceDeps) *IncidentService {
	switch {
	case deps.IncidentRepo == nil:
		panic("integrations incident service requires incident repo")
	case deps.WritebackLogRepo == nil:
		panic("integrations incident service requires incident writeback log repo")
	case deps.PluginRepo == nil:
		panic("integrations incident service requires plugin repo")
	}
	if deps.HTTPClient == nil {
		deps.HTTPClient = NewHTTPClient()
	}
	return &IncidentService{
		incidentRepo:     deps.IncidentRepo,
		writebackLogRepo: deps.WritebackLogRepo,
		pluginRepo:       deps.PluginRepo,
		httpClient:       deps.HTTPClient,
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

func (s *IncidentService) GetStats(ctx context.Context) (*incidentrepo.IncidentStats, error) {
	return s.incidentRepo.GetStats(ctx)
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
