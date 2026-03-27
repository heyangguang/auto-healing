package httpapi

import (
	"fmt"

	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
)

type workbenchOverviewResponse struct {
	SystemHealth     *engagementrepo.SystemHealth           `json:"system_health,omitempty"`
	ResourceOverview *engagementrepo.ResourceOverview       `json:"resource_overview,omitempty"`
	HealingStats     *engagementrepo.HealingStats           `json:"healing_stats,omitempty"`
	IncidentStats    *engagementrepo.WorkbenchIncidentStats `json:"incident_stats,omitempty"`
	HostStats        *engagementrepo.HostStats              `json:"host_stats,omitempty"`
}

func newWorkbenchOverviewResponse(sections map[string]interface{}) (workbenchOverviewResponse, error) {
	result := workbenchOverviewResponse{}
	for section, data := range sections {
		if err := assignWorkbenchSection(&result, section, data); err != nil {
			return workbenchOverviewResponse{}, err
		}
	}
	return result, nil
}

func assignWorkbenchSection(result *workbenchOverviewResponse, section string, data interface{}) error {
	switch section {
	case "system_health":
		typed, ok := data.(*engagementrepo.SystemHealth)
		if !ok {
			return unexpectedWorkbenchSectionType(section, data)
		}
		result.SystemHealth = typed
	case "resource_overview":
		typed, ok := data.(*engagementrepo.ResourceOverview)
		if !ok {
			return unexpectedWorkbenchSectionType(section, data)
		}
		result.ResourceOverview = typed
	case "healing_stats":
		typed, ok := data.(*engagementrepo.HealingStats)
		if !ok {
			return unexpectedWorkbenchSectionType(section, data)
		}
		result.HealingStats = typed
	case "incident_stats":
		typed, ok := data.(*engagementrepo.WorkbenchIncidentStats)
		if !ok {
			return unexpectedWorkbenchSectionType(section, data)
		}
		result.IncidentStats = typed
	case "host_stats":
		typed, ok := data.(*engagementrepo.HostStats)
		if !ok {
			return unexpectedWorkbenchSectionType(section, data)
		}
		result.HostStats = typed
	default:
		return fmt.Errorf("unknown workbench section: %s", section)
	}
	return nil
}

func unexpectedWorkbenchSectionType(section string, data interface{}) error {
	return fmt.Errorf("workbench section %s returned %T", section, data)
}
