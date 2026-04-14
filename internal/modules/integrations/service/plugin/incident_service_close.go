package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

type CloseIncidentParams struct {
	IncidentID     uuid.UUID
	Resolution     string
	WorkNotes      string
	CloseCode      string
	CloseStatus    string
	TriggerSource  string
	OperatorUserID *uuid.UUID
	OperatorName   string
	FlowInstanceID *uuid.UUID
	ExecutionRunID *uuid.UUID
}

type CloseIncidentResponse struct {
	Message        string     `json:"message"`
	LocalStatus    string     `json:"local_status"`
	SourceUpdated  bool       `json:"source_updated"`
	WritebackLogID *uuid.UUID `json:"writeback_log_id,omitempty"`
}

type incidentWritebackRequest struct {
	config     model.JSON
	closeURL   string
	method     string
	payload    map[string]any
	pluginID   *uuid.UUID
	externalID string
}

func (s *IncidentService) CloseIncident(ctx context.Context, params CloseIncidentParams) (*CloseIncidentResponse, error) {
	incident, err := s.incidentRepo.GetByID(ctx, params.IncidentID)
	if err != nil {
		return nil, fmt.Errorf("获取工单失败: %w", err)
	}

	params = normalizeCloseIncidentParams(params)
	writebackReq, buildErr := s.buildIncidentWritebackRequest(ctx, incident, params)
	logEntry, err := s.createWritebackLog(ctx, incident, params, writebackReq, buildErr)
	if err != nil {
		return nil, err
	}

	sourceUpdated, sourceErr := s.executeIncidentCloseWriteback(ctx, incident, params, writebackReq, buildErr, logEntry)
	if sourceErr != nil {
		return nil, sourceErr
	}

	finishedAt := time.Now().UTC()
	incident.Status = params.CloseStatus
	incident.HealingStatus = "healed"
	if sourceUpdated {
		incident.SourceUpdatedAt = &finishedAt
	}
	if err := s.incidentRepo.Update(ctx, incident); err != nil {
		return nil, fmt.Errorf("更新本地工单状态失败: %w", err)
	}

	return &CloseIncidentResponse{
		Message:        "工单已关闭",
		LocalStatus:    "healed",
		SourceUpdated:  sourceUpdated,
		WritebackLogID: writebackLogID(logEntry),
	}, nil
}

func normalizeCloseIncidentParams(params CloseIncidentParams) CloseIncidentParams {
	if params.CloseStatus == "" {
		params.CloseStatus = "resolved"
	}
	if params.TriggerSource == "" {
		params.TriggerSource = platformmodel.IncidentWritebackTriggerManualClose
	}
	params.OperatorName = strings.TrimSpace(params.OperatorName)
	return params
}

func (s *IncidentService) buildIncidentWritebackRequest(
	ctx context.Context,
	incident *platformmodel.Incident,
	params CloseIncidentParams,
) (*incidentWritebackRequest, error) {
	if incident.PluginID == nil {
		return nil, nil
	}

	plugin, err := s.pluginRepo.GetByID(ctx, *incident.PluginID)
	if err != nil {
		if errors.Is(err, integrationrepo.ErrPluginNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("获取来源插件失败: %w", err)
	}
	if plugin == nil {
		return nil, nil
	}

	closeURL, _ := plugin.Config["close_incident_url"].(string)
	method := "POST"
	if configuredMethod, ok := plugin.Config["close_incident_method"].(string); ok && configuredMethod != "" {
		method = strings.ToUpper(configuredMethod)
	}

	return &incidentWritebackRequest{
		config:   plugin.Config,
		closeURL: strings.ReplaceAll(closeURL, "{external_id}", incident.ExternalID),
		method:   method,
		payload: map[string]any{
			"external_id":  incident.ExternalID,
			"resolution":   params.Resolution,
			"work_notes":   params.WorkNotes,
			"close_code":   params.CloseCode,
			"close_status": params.CloseStatus,
		},
		pluginID:   incident.PluginID,
		externalID: incident.ExternalID,
	}, nil
}

func (s *IncidentService) createWritebackLog(
	ctx context.Context,
	incident *platformmodel.Incident,
	params CloseIncidentParams,
	req *incidentWritebackRequest,
	buildErr error,
) (*platformmodel.IncidentWritebackLog, error) {
	if incident.PluginID == nil {
		return nil, nil
	}

	logEntry := &platformmodel.IncidentWritebackLog{
		IncidentID:     incident.ID,
		PluginID:       incident.PluginID,
		ExternalID:     incident.ExternalID,
		Action:         platformmodel.IncidentWritebackActionClose,
		TriggerSource:  params.TriggerSource,
		Status:         platformmodel.IncidentWritebackStatusPending,
		RequestPayload: modeltypes.JSON{},
		OperatorUserID: params.OperatorUserID,
		OperatorName:   params.OperatorName,
		FlowInstanceID: params.FlowInstanceID,
		ExecutionRunID: params.ExecutionRunID,
		StartedAt:      time.Now().UTC(),
	}
	if req != nil {
		logEntry.RequestMethod = req.method
		logEntry.RequestURL = req.closeURL
		logEntry.RequestPayload = modeltypes.JSON(req.payload)
	}
	if buildErr != nil {
		logEntry.Status = platformmodel.IncidentWritebackStatusFailed
		logEntry.ErrorMessage = buildErr.Error()
		finishedAt := logEntry.StartedAt
		logEntry.FinishedAt = &finishedAt
	}
	if req == nil && buildErr == nil {
		logEntry.Status = platformmodel.IncidentWritebackStatusSkipped
		logEntry.ErrorMessage = "未配置关闭工单回写接口"
		finishedAt := logEntry.StartedAt
		logEntry.FinishedAt = &finishedAt
	}
	if err := s.writebackLogRepo.Create(ctx, logEntry); err != nil {
		return nil, fmt.Errorf("创建工单回写日志失败: %w", err)
	}
	return logEntry, nil
}

func (s *IncidentService) executeIncidentCloseWriteback(
	ctx context.Context,
	incident *platformmodel.Incident,
	params CloseIncidentParams,
	req *incidentWritebackRequest,
	buildErr error,
	logEntry *platformmodel.IncidentWritebackLog,
) (bool, error) {
	if buildErr != nil {
		logger.Sync_("PLUGIN").Warn("构建工单回写请求失败: incident_id=%s, external_id=%s, error=%s", incident.ID, incident.ExternalID, buildErr.Error())
		return false, buildErr
	}
	if req == nil {
		return false, nil
	}

	result, err := s.httpClient.CloseIncident(ctx, req.config, req.closeURL, req.method, req.payload)
	if err != nil {
		if logEntry != nil {
			finishedAt := time.Now().UTC()
			logEntry.Status = platformmodel.IncidentWritebackStatusFailed
			logEntry.ErrorMessage = err.Error()
			logEntry.ResponseStatusCode = intPointer(result.StatusCode)
			logEntry.ResponseBody = result.ResponseBody
			logEntry.FinishedAt = &finishedAt
			if updateErr := s.writebackLogRepo.Update(ctx, logEntry); updateErr != nil {
				logger.Sync_("PLUGIN").Warn("更新工单回写失败日志失败: log_id=%s, error=%s", logEntry.ID, updateErr.Error())
			}
		}
		logger.Sync_("PLUGIN").Warn("回写工单到源系统失败: incident_id=%s, external_id=%s, error=%s", incident.ID, incident.ExternalID, err.Error())
		return false, fmt.Errorf("回写工单到源系统失败: %w", err)
	}

	if logEntry != nil {
		finishedAt := time.Now().UTC()
		logEntry.Status = platformmodel.IncidentWritebackStatusSuccess
		logEntry.ResponseStatusCode = intPointer(result.StatusCode)
		logEntry.ResponseBody = result.ResponseBody
		logEntry.FinishedAt = &finishedAt
		if updateErr := s.writebackLogRepo.Update(ctx, logEntry); updateErr != nil {
			logger.Sync_("PLUGIN").Warn("更新工单回写成功日志失败: log_id=%s, error=%s", logEntry.ID, updateErr.Error())
		}
	}

	logger.Sync_("PLUGIN").Info("成功回写工单到源系统: incident_id=%s, external_id=%s", incident.ID, incident.ExternalID)
	return true, nil
}

func writebackLogID(logEntry *platformmodel.IncidentWritebackLog) *uuid.UUID {
	if logEntry == nil {
		return nil
	}
	return &logEntry.ID
}

func intPointer(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}
