package projection

import (
	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
)

type Plugin = integrationsmodel.Plugin
type PluginSyncLog = integrationsmodel.PluginSyncLog
type GitSyncLog = integrationsmodel.GitSyncLog
type Incident = platformmodel.Incident
type CMDBItem = platformmodel.CMDBItem
type CMDBMaintenanceLog = platformmodel.CMDBMaintenanceLog
type SecretsSource = secretsmodel.SecretsSource
type AuditLog = platformmodel.AuditLog
type UserTenantRole = accessmodel.UserTenantRole
