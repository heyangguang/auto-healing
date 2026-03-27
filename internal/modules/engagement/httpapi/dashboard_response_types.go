package httpapi

import (
	"fmt"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/google/uuid"
)

type dashboardOverviewResponse struct {
	Incidents     *engagementrepo.IncidentSection     `json:"incidents,omitempty"`
	CMDB          *engagementrepo.CMDBSection         `json:"cmdb,omitempty"`
	Healing       *engagementrepo.HealingSection      `json:"healing,omitempty"`
	Execution     *engagementrepo.ExecutionSection    `json:"execution,omitempty"`
	Plugins       *engagementrepo.PluginSection       `json:"plugins,omitempty"`
	Notifications *engagementrepo.NotificationSection `json:"notifications,omitempty"`
	Git           *engagementrepo.GitSection          `json:"git,omitempty"`
	Playbooks     *engagementrepo.PlaybookSection     `json:"playbooks,omitempty"`
	Secrets       *engagementrepo.SecretsSection      `json:"secrets,omitempty"`
	Users         *engagementrepo.UsersSection        `json:"users,omitempty"`
}

type dashboardConfigResponse struct {
	Config           model.JSON                         `json:"config"`
	SystemWorkspaces []dashboardSystemWorkspaceResponse `json:"system_workspaces"`
}

type dashboardSystemWorkspaceResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Config      model.JSON `json:"config"`
	IsSystem    bool       `json:"is_system"`
	IsReadonly  bool       `json:"is_readonly"`
	IsDefault   bool       `json:"is_default"`
}

func newDashboardOverviewResponse(sections map[string]interface{}) (dashboardOverviewResponse, error) {
	result := dashboardOverviewResponse{}
	for section, data := range sections {
		if err := assignDashboardSection(&result, section, data); err != nil {
			return dashboardOverviewResponse{}, err
		}
	}
	return result, nil
}

func assignDashboardSection(result *dashboardOverviewResponse, section string, data interface{}) error {
	switch section {
	case "incidents":
		typed, ok := data.(*engagementrepo.IncidentSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Incidents = typed
	case "cmdb":
		typed, ok := data.(*engagementrepo.CMDBSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.CMDB = typed
	case "healing":
		typed, ok := data.(*engagementrepo.HealingSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Healing = typed
	case "execution":
		typed, ok := data.(*engagementrepo.ExecutionSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Execution = typed
	case "plugins":
		typed, ok := data.(*engagementrepo.PluginSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Plugins = typed
	case "notifications":
		typed, ok := data.(*engagementrepo.NotificationSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Notifications = typed
	case "git":
		typed, ok := data.(*engagementrepo.GitSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Git = typed
	case "playbooks":
		typed, ok := data.(*engagementrepo.PlaybookSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Playbooks = typed
	case "secrets":
		typed, ok := data.(*engagementrepo.SecretsSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Secrets = typed
	case "users":
		typed, ok := data.(*engagementrepo.UsersSection)
		if !ok {
			return unexpectedDashboardSectionType(section, data)
		}
		result.Users = typed
	default:
		return fmt.Errorf("unknown dashboard section: %s", section)
	}
	return nil
}

func unexpectedDashboardSectionType(section string, data interface{}) error {
	return fmt.Errorf("dashboard section %s returned %T", section, data)
}

func newDashboardConfigResponse(config *model.DashboardConfig, workspaces []model.SystemWorkspace) dashboardConfigResponse {
	result := dashboardConfigResponse{
		Config:           model.JSON{},
		SystemWorkspaces: buildSystemWorkspaceList(workspaces),
	}
	if config != nil && config.Config != nil {
		result.Config = config.Config
	}
	return result
}

func buildSystemWorkspaceList(workspaces []model.SystemWorkspace) []dashboardSystemWorkspaceResponse {
	items := make([]dashboardSystemWorkspaceResponse, 0, len(workspaces))
	for _, workspace := range workspaces {
		items = append(items, dashboardSystemWorkspaceResponse{
			ID:          workspace.ID,
			Name:        workspace.Name,
			Description: workspace.Description,
			Config:      workspace.Config,
			IsSystem:    true,
			IsReadonly:  true,
			IsDefault:   workspace.IsDefault,
		})
	}
	return items
}
