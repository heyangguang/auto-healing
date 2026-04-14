package model

import (
	"time"

	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

const (
	IncidentWritebackActionClose  = "close"
	IncidentWritebackActionUpdate = "update"
)

const (
	IncidentWritebackTriggerManualClose   = "manual_close"
	IncidentWritebackTriggerFlowAutoClose = "flow_auto_close"
	IncidentWritebackTriggerFlowUpdate    = "flow_update"
)

const (
	IncidentWritebackStatusPending = "pending"
	IncidentWritebackStatusSuccess = "success"
	IncidentWritebackStatusFailed  = "failed"
	IncidentWritebackStatusSkipped = "skipped"
)

type IncidentWritebackLog struct {
	ID                 uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID      `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	IncidentID         uuid.UUID       `json:"incident_id" gorm:"type:uuid;not null;index"`
	PluginID           *uuid.UUID      `json:"plugin_id,omitempty" gorm:"type:uuid;index"`
	ExternalID         string          `json:"external_id" gorm:"type:varchar(200);not null"`
	Action             string          `json:"action" gorm:"type:varchar(20);not null"`
	TriggerSource      string          `json:"trigger_source" gorm:"type:varchar(50);not null"`
	Status             string          `json:"status" gorm:"type:varchar(20);not null;default:'pending'"`
	RequestMethod      string          `json:"request_method,omitempty" gorm:"type:varchar(10)"`
	RequestURL         string          `json:"request_url,omitempty" gorm:"type:text"`
	RequestPayload     modeltypes.JSON `json:"request_payload,omitempty" gorm:"type:jsonb;default:'{}'"`
	ResponseStatusCode *int            `json:"response_status_code,omitempty"`
	ResponseBody       string          `json:"response_body,omitempty" gorm:"type:text"`
	ErrorMessage       string          `json:"error_message,omitempty" gorm:"type:text"`
	OperatorUserID     *uuid.UUID      `json:"operator_user_id,omitempty" gorm:"type:uuid;index"`
	OperatorName       string          `json:"operator_name,omitempty" gorm:"type:varchar(200)"`
	FlowInstanceID     *uuid.UUID      `json:"flow_instance_id,omitempty" gorm:"type:uuid;index"`
	ExecutionRunID     *uuid.UUID      `json:"execution_run_id,omitempty" gorm:"type:uuid;index"`
	StartedAt          time.Time       `json:"started_at" gorm:"default:now()"`
	FinishedAt         *time.Time      `json:"finished_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at" gorm:"default:now()"`
}

func (IncidentWritebackLog) TableName() string {
	return "incident_writeback_logs"
}
