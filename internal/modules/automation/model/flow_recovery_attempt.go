package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	FlowRecoveryTriggerManual    = "manual"
	FlowRecoveryTriggerScheduler = "scheduler"
)

const (
	FlowRecoveryStatusStarted = "started"
	FlowRecoveryStatusSuccess = "success"
	FlowRecoveryStatusFailed  = "failed"
	FlowRecoveryStatusSkipped = "skipped"
)

const (
	FlowRecoveryActionResumeApproval   = "resume_approval"
	FlowRecoveryActionResumeExecution  = "resume_execution"
	FlowRecoveryActionResumeDefault    = "resume_default"
	FlowRecoveryActionResumeFromStart  = "resume_from_start"
	FlowRecoveryActionRerunCurrentNode = "rerun_current_node"
	FlowRecoveryActionCompleteInstance = "complete_instance"
	FlowRecoveryActionFailInstance     = "fail_instance"
	FlowRecoveryActionWaitExternalRun  = "wait_external_run"
	FlowRecoveryActionWaitApproval     = "wait_approval"
)

type FlowRecoveryAttempt struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID        *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	FlowInstanceID  uuid.UUID  `json:"flow_instance_id" gorm:"type:uuid;not null;index"`
	TriggerSource   string     `json:"trigger_source" gorm:"type:varchar(20);not null"`
	CurrentNodeID   string     `json:"current_node_id,omitempty" gorm:"type:varchar(100)"`
	CurrentNodeType string     `json:"current_node_type,omitempty" gorm:"type:varchar(50)"`
	DetectReason    string     `json:"detect_reason,omitempty" gorm:"type:text"`
	RecoveryAction  string     `json:"recovery_action,omitempty" gorm:"type:varchar(50)"`
	Status          string     `json:"status" gorm:"type:varchar(20);not null;default:'started'"`
	Details         JSON       `json:"details,omitempty" gorm:"type:jsonb;default:'{}'"`
	ErrorMessage    string     `json:"error_message,omitempty" gorm:"type:text"`
	StartedAt       time.Time  `json:"started_at" gorm:"default:now()"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"default:now()"`
}

func (FlowRecoveryAttempt) TableName() string {
	return "flow_recovery_attempts"
}
