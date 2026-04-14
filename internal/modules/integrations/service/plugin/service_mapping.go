package plugin

import (
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

var supportedTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02",
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
	incidentMapping := extractFieldMapping(fieldMapping, "incident_mapping")
	incidents := make([]RawIncident, 0, len(rawData))

	for _, data := range rawData {
		incidents = append(incidents, RawIncident{
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
			SourceCreatedAt: getTimeField(data, incidentMapping, "source_created_at", "source_created_at"),
			SourceUpdatedAt: getTimeField(data, incidentMapping, "source_updated_at", "source_updated_at"),
			RawData:         data,
		})
	}
	return incidents
}

// mapToCMDBItems 将原始数据按字段映射转换为 CMDB 格式
func (s *Service) mapToCMDBItems(rawData []map[string]interface{}, fieldMapping model.JSON) []RawCMDBItem {
	cmdbMapping := extractFieldMapping(fieldMapping, "cmdb_mapping")
	items := make([]RawCMDBItem, 0, len(rawData))

	for _, data := range rawData {
		items = append(items, RawCMDBItem{
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
			SourceCreatedAt: getTimeField(
				data,
				cmdbMapping,
				"source_created_at",
				"source_created_at",
			),
			SourceUpdatedAt: getTimeField(
				data,
				cmdbMapping,
				"source_updated_at",
				"source_updated_at",
			),
			RawData:      data,
		})
	}
	return items
}

func extractFieldMapping(fieldMapping model.JSON, key string) map[string]string {
	mapping := make(map[string]string)
	rawMapping, ok := fieldMapping[key].(map[string]interface{})
	if !ok {
		return mapping
	}
	for field, value := range rawMapping {
		if text, ok := value.(string); ok {
			mapping[field] = text
		}
	}
	return mapping
}

// getStringField 从数据中获取字段值（支持字段映射）
func getStringField(data map[string]interface{}, mapping map[string]string, fieldName, defaultField string) string {
	actualField := defaultField
	if mapped, ok := mapping[fieldName]; ok && mapped != "" {
		actualField = mapped
	}
	if value, ok := data[actualField]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func getTimeField(data map[string]interface{}, mapping map[string]string, fieldName, defaultField string) time.Time {
	return parseTimeValue(getStringField(data, mapping, fieldName, defaultField))
}

func parseTimeValue(raw string) time.Time {
	for _, layout := range supportedTimeLayouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
