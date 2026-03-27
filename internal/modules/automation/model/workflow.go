package model

import (
	"time"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

type Workflow struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name          string     `json:"name" gorm:"type:varchar(200);not null"`
	Description   string     `json:"description,omitempty" gorm:"type:text"`
	Version       int        `json:"version" gorm:"default:1"`
	Status        string     `json:"status" gorm:"type:varchar(20);default:'draft'"`
	TriggerType   string     `json:"trigger_type" gorm:"type:varchar(50);not null"`
	TriggerConfig JSON       `json:"trigger_config" gorm:"type:jsonb;default:'{}'"`
	CreatedBy     string     `json:"created_by,omitempty" gorm:"type:varchar(200)"`
	CreatedAt     time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"default:now()"`

	Nodes []WorkflowNode `json:"nodes,omitempty" gorm:"foreignKey:WorkflowID"`
	Edges []WorkflowEdge `json:"edges,omitempty" gorm:"foreignKey:WorkflowID"`
}

func (Workflow) TableName() string {
	return "workflows"
}

type WorkflowNode struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	WorkflowID  uuid.UUID  `json:"workflow_id" gorm:"type:uuid;not null"`
	NodeType    string     `json:"node_type" gorm:"type:varchar(50);not null"`
	Name        string     `json:"name" gorm:"type:varchar(200);not null"`
	Description string     `json:"description,omitempty" gorm:"type:text"`
	Config      JSON       `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	PositionX   int        `json:"position_x" gorm:"default:0"`
	PositionY   int        `json:"position_y" gorm:"default:0"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
}

func (WorkflowNode) TableName() string {
	return "workflow_nodes"
}

type WorkflowEdge struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID            *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	WorkflowID          uuid.UUID  `json:"workflow_id" gorm:"type:uuid;not null"`
	SourceNodeID        uuid.UUID  `json:"source_node_id" gorm:"type:uuid;not null"`
	TargetNodeID        uuid.UUID  `json:"target_node_id" gorm:"type:uuid;not null"`
	ConditionExpression string     `json:"condition_expression,omitempty" gorm:"type:text"`
	Label               string     `json:"label,omitempty" gorm:"type:varchar(100)"`
	Priority            int        `json:"priority" gorm:"default:0"`
	CreatedAt           time.Time  `json:"created_at" gorm:"default:now()"`

	SourceNode *WorkflowNode `json:"source_node,omitempty" gorm:"foreignKey:SourceNodeID"`
	TargetNode *WorkflowNode `json:"target_node,omitempty" gorm:"foreignKey:TargetNodeID"`
}

func (WorkflowEdge) TableName() string {
	return "workflow_edges"
}

type WorkflowInstance struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	WorkflowID    uuid.UUID  `json:"workflow_id" gorm:"type:uuid;not null"`
	IncidentID    *uuid.UUID `json:"incident_id,omitempty" gorm:"type:uuid"`
	Status        string     `json:"status" gorm:"type:varchar(50);default:'running'"`
	CurrentNodeID *uuid.UUID `json:"current_node_id,omitempty" gorm:"type:uuid"`
	Context       JSON       `json:"context" gorm:"type:jsonb;default:'{}'"`
	StartedAt     time.Time  `json:"started_at" gorm:"default:now()"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`

	Workflow       Workflow                `json:"workflow,omitempty" gorm:"foreignKey:WorkflowID"`
	Incident       *platformmodel.Incident `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
	CurrentNode    *WorkflowNode           `json:"current_node,omitempty" gorm:"foreignKey:CurrentNodeID"`
	NodeExecutions []NodeExecution         `json:"node_executions,omitempty" gorm:"foreignKey:WorkflowInstanceID"`
}

func (WorkflowInstance) TableName() string {
	return "workflow_instances"
}

type NodeExecution struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	WorkflowInstanceID uuid.UUID  `json:"workflow_instance_id" gorm:"type:uuid;not null"`
	NodeID             uuid.UUID  `json:"node_id" gorm:"type:uuid;not null"`
	Status             string     `json:"status" gorm:"type:varchar(50);not null"`
	InputData          JSON       `json:"input_data" gorm:"type:jsonb;default:'{}'"`
	OutputData         JSON       `json:"output_data" gorm:"type:jsonb;default:'{}'"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	RetryCount         int        `json:"retry_count" gorm:"default:0"`
	ErrorMessage       string     `json:"error_message,omitempty" gorm:"type:text"`

	Node *WorkflowNode `json:"node,omitempty" gorm:"foreignKey:NodeID"`
}

func (NodeExecution) TableName() string {
	return "node_executions"
}
