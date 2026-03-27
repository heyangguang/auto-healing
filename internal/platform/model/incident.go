package model

import (
	"time"

	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

// Incident 工单/事件模型
type Incident struct {
	ID                    uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID              *uuid.UUID      `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	PluginID              *uuid.UUID      `json:"plugin_id" gorm:"type:uuid"`
	SourcePluginName      string          `json:"source_plugin_name" gorm:"type:varchar(100)"`
	ExternalID            string          `json:"external_id" gorm:"type:varchar(200);not null"`
	Title                 string          `json:"title" gorm:"type:varchar(500);not null"`
	Description           string          `json:"description" gorm:"type:text"`
	Severity              string          `json:"severity" gorm:"type:varchar(20)"`
	Priority              string          `json:"priority" gorm:"type:varchar(20)"`
	Status                string          `json:"status" gorm:"type:varchar(50)"`
	Category              string          `json:"category" gorm:"type:varchar(100)"`
	AffectedCI            string          `json:"affected_ci" gorm:"type:varchar(200)"`
	AffectedService       string          `json:"affected_service" gorm:"type:varchar(200)"`
	Assignee              string          `json:"assignee" gorm:"type:varchar(200)"`
	Reporter              string          `json:"reporter" gorm:"type:varchar(200)"`
	RawData               modeltypes.JSON `json:"raw_data" gorm:"type:jsonb;not null"`
	HealingStatus         string          `json:"healing_status" gorm:"type:varchar(50);default:'pending'"`
	WorkflowInstanceID    *uuid.UUID      `json:"workflow_instance_id" gorm:"type:uuid"`
	Scanned               bool            `json:"scanned" gorm:"default:false"`
	MatchedRuleID         *uuid.UUID      `json:"matched_rule_id,omitempty" gorm:"type:uuid;index"`
	HealingFlowInstanceID *uuid.UUID      `json:"healing_flow_instance_id,omitempty" gorm:"type:uuid;index"`
	SourceCreatedAt       *time.Time      `json:"source_created_at"`
	SourceUpdatedAt       *time.Time      `json:"source_updated_at"`
	CreatedAt             time.Time       `json:"created_at" gorm:"default:now()"`
	UpdatedAt             time.Time       `json:"updated_at" gorm:"default:now()"`

	Plugin *integrationsmodel.Plugin `json:"-" gorm:"foreignKey:PluginID"`
}

func (Incident) TableName() string {
	return "incidents"
}
