package model

import (
	"time"

	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

type VariableConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
	Description string `json:"description"`
	SourceFile  string `json:"source_file"`
	SourceLine  int    `json:"source_line"`
	InCode      bool   `json:"in_code"`
}

type Playbook struct {
	ID                    uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID              *uuid.UUID           `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	RepositoryID          uuid.UUID            `json:"repository_id" gorm:"type:uuid;not null"`
	Name                  string               `json:"name" gorm:"type:varchar(200);not null"`
	Description           string               `json:"description,omitempty" gorm:"type:text"`
	FilePath              string               `json:"file_path" gorm:"type:varchar(500);not null"`
	Variables             modeltypes.JSONArray `json:"variables" gorm:"type:jsonb;default:'[]'"`
	ScannedVariables      modeltypes.JSONArray `json:"scanned_variables" gorm:"type:jsonb;default:'[]'"`
	LastScannedAt         *time.Time           `json:"last_scanned_at"`
	ConfigMode            string               `json:"config_mode" gorm:"type:varchar(20)"`
	Status                string               `json:"status" gorm:"type:varchar(20);default:'pending'"`
	Tags                  modeltypes.JSONArray `json:"tags" gorm:"type:jsonb;default:'[]'"`
	DefaultExtraVars      modeltypes.JSON      `json:"default_extra_vars" gorm:"type:jsonb;default:'{}'"`
	DefaultTimeoutMinutes int                  `json:"default_timeout_minutes" gorm:"default:60"`
	CreatedBy             *uuid.UUID           `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt             time.Time            `json:"created_at" gorm:"default:now()"`
	UpdatedAt             time.Time            `json:"updated_at" gorm:"default:now()"`

	Repository *GitRepository `json:"repository,omitempty" gorm:"foreignKey:RepositoryID"`
}

func (Playbook) TableName() string {
	return "playbooks"
}

type PlaybookScanLog struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       *uuid.UUID      `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	PlaybookID     uuid.UUID       `json:"playbook_id" gorm:"type:uuid;not null"`
	TriggerType    string          `json:"trigger_type" gorm:"type:varchar(20);not null"`
	FilesScanned   int             `json:"files_scanned" gorm:"default:0"`
	VariablesFound int             `json:"variables_found" gorm:"default:0"`
	NewCount       int             `json:"new_count" gorm:"default:0"`
	RemovedCount   int             `json:"removed_count" gorm:"default:0"`
	Details        modeltypes.JSON `json:"details" gorm:"type:jsonb;default:'{}'"`
	CreatedAt      time.Time       `json:"created_at" gorm:"default:now()"`

	Playbook *Playbook `json:"playbook,omitempty" gorm:"foreignKey:PlaybookID"`
}

func (PlaybookScanLog) TableName() string {
	return "playbook_scan_logs"
}
