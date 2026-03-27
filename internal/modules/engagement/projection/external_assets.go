package projection

import (
	"time"

	"github.com/google/uuid"
)

type Plugin struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty"`
}

func (Plugin) TableName() string {
	return "plugins"
}

type PluginSyncLog struct {
	ID        uuid.UUID `json:"id"`
	PluginID  uuid.UUID `json:"plugin_id"`
	Plugin    Plugin    `json:"plugin,omitempty" gorm:"foreignKey:PluginID"`
	Status    string    `json:"status"`
	SyncType  string    `json:"sync_type"`
	StartedAt time.Time `json:"started_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (PluginSyncLog) TableName() string {
	return "plugin_sync_logs"
}

type GitSyncLog struct {
	ID           uuid.UUID      `json:"id"`
	RepositoryID uuid.UUID      `json:"repository_id"`
	Status       string         `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	Repository   *GitRepository `json:"repository,omitempty" gorm:"foreignKey:RepositoryID"`
}

func (GitSyncLog) TableName() string {
	return "git_sync_logs"
}

type Incident struct {
	ID               uuid.UUID `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	ExternalID       string    `json:"external_id"`
	Severity         string    `json:"severity"`
	Status           string    `json:"status"`
	HealingStatus    string    `json:"healing_status"`
	AffectedCI       string    `json:"affected_ci"`
	SourcePluginName string    `json:"source_plugin_name"`
	CreatedAt        time.Time `json:"created_at"`
	Scanned          bool      `json:"scanned"`
}

func (Incident) TableName() string {
	return "incidents"
}

type CMDBItem struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Hostname     string    `json:"hostname"`
	IPAddress    string    `json:"ip_address"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Environment  string    `json:"environment"`
	OS           string    `json:"os"`
	Department   string    `json:"department"`
	Manufacturer string    `json:"manufacturer"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (CMDBItem) TableName() string {
	return "cmdb_items"
}

type CMDBMaintenanceLog struct {
	ID           uuid.UUID `json:"id"`
	CMDBItemName string    `json:"cmdb_item_name"`
	Action       string    `json:"action"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

func (CMDBMaintenanceLog) TableName() string {
	return "cmdb_maintenance_logs"
}

type SecretsSource struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

func (SecretsSource) TableName() string {
	return "secrets_sources"
}

type AuditLog struct {
	ID           uuid.UUID `json:"id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name"`
	CreatedAt    time.Time `json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

type UserTenantRole struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	RoleID   uuid.UUID `json:"role_id"`
}

func (UserTenantRole) TableName() string {
	return "user_tenant_roles"
}
