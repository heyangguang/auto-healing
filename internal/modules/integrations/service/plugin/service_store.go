package plugin

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

// saveIncident 保存工单到数据库
// 返回: (isNew, error) - isNew=true 表示新增
func (s *Service) saveIncident(ctx context.Context, pluginID uuid.UUID, pluginName string, raw RawIncident) (bool, error) {
	incident := &platformmodel.Incident{
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
	return s.incidentRepo.UpsertByExternalID(ctx, incident)
}

// saveCMDBItem 保存 CMDB 配置项到数据库
// 返回: (isNew, error) - isNew=true 表示新增
func (s *Service) saveCMDBItem(ctx context.Context, pluginID uuid.UUID, pluginName string, raw RawCMDBItem) (bool, error) {
	item := &platformmodel.CMDBItem{
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
		Dependencies:     toJSONArray(raw.Dependencies),
		Tags:             toJSON(raw.Tags),
		RawData:          raw.RawData,
		SourceCreatedAt:  optionalTime(raw.SourceCreatedAt),
		SourceUpdatedAt:  optionalTime(raw.SourceUpdatedAt),
	}
	return s.cmdbRepo.UpsertByExternalID(ctx, item)
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func toJSONArray(values []string) model.JSONArray {
	result := make(model.JSONArray, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func toJSON(values map[string]string) model.JSON {
	result := model.JSON{}
	for key, value := range values {
		result[key] = value
	}
	return result
}
