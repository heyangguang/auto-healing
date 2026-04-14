package healing

import (
	"strings"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
)

const incidentPathPrefix = "incident."

func normalizeIncidentSourceField(sourceField string) string {
	sourceField = strings.TrimSpace(sourceField)
	return strings.TrimPrefix(sourceField, incidentPathPrefix)
}

func resolveFlowContextSourceValue(
	flowContext map[string]interface{},
	sourceField string,
) interface{} {
	if flowContext == nil || strings.TrimSpace(sourceField) == "" {
		return nil
	}

	if value := nestedValue(flowContext, sourceField); value != nil {
		return value
	}

	incidentPath := normalizeIncidentSourceField(sourceField)
	incidentRaw, ok := flowContext["incident"]
	if !ok || incidentPath == "" {
		return nil
	}

	switch incident := incidentRaw.(type) {
	case map[string]interface{}:
		return nestedValue(incident, incidentPath)
	case *platformmodel.Incident:
		return incidentFieldValue(incident, incidentPath)
	default:
		return nil
	}
}

func incidentFieldValue(
	incident *platformmodel.Incident,
	field string,
) interface{} {
	if incident == nil {
		return nil
	}

	switch field {
	case "affected_ci":
		return incident.AffectedCI
	case "affected_service":
		return incident.AffectedService
	case "title":
		return incident.Title
	case "description":
		return incident.Description
	case "severity":
		return incident.Severity
	case "priority":
		return incident.Priority
	case "status":
		return incident.Status
	case "category":
		return incident.Category
	default:
		if incident.RawData != nil {
			return incident.RawData[field]
		}
		return nil
	}
}
