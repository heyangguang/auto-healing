package model

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowLog struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	WorkflowInstanceID uuid.UUID  `json:"workflow_instance_id" gorm:"type:uuid;not null"`
	NodeID             *uuid.UUID `json:"node_id,omitempty" gorm:"type:uuid"`
	LogLevel           string     `json:"log_level" gorm:"type:varchar(20);not null"`
	Stage              string     `json:"stage" gorm:"type:varchar(50);not null"`
	Message            string     `json:"message" gorm:"type:text;not null"`
	Details            JSON       `json:"details" gorm:"type:jsonb;default:'{}'"`
	Sequence           int        `json:"sequence" gorm:"not null"`
	CreatedAt          time.Time  `json:"created_at" gorm:"default:now()"`

	WorkflowInstance *WorkflowInstance `json:"workflow_instance,omitempty" gorm:"foreignKey:WorkflowInstanceID"`
	Node             *WorkflowNode     `json:"node,omitempty" gorm:"foreignKey:NodeID"`
}

func (WorkflowLog) TableName() string {
	return "workflow_logs"
}
