package model

import (
	"time"

	"github.com/google/uuid"
)

// Workflow 工作流模型
type Workflow struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name          string     `json:"name" gorm:"type:varchar(200);not null"`
	Description   string     `json:"description,omitempty" gorm:"type:text"`
	Version       int        `json:"version" gorm:"default:1"`
	Status        string     `json:"status" gorm:"type:varchar(20);default:'draft'"` // draft, active, inactive, archived
	TriggerType   string     `json:"trigger_type" gorm:"type:varchar(50);not null"`  // incident, scheduled, manual, webhook
	TriggerConfig JSON       `json:"trigger_config" gorm:"type:jsonb;default:'{}'"`
	CreatedBy     string     `json:"created_by,omitempty" gorm:"type:varchar(200)"`
	CreatedAt     time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	Nodes []WorkflowNode `json:"nodes,omitempty" gorm:"foreignKey:WorkflowID"`
	Edges []WorkflowEdge `json:"edges,omitempty" gorm:"foreignKey:WorkflowID"`
}

// TableName 表名
func (Workflow) TableName() string {
	return "workflows"
}

// WorkflowNode 工作流节点
type WorkflowNode struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	WorkflowID  uuid.UUID `json:"workflow_id" gorm:"type:uuid;not null"`
	NodeType    string    `json:"node_type" gorm:"type:varchar(50);not null"` // start, end, condition, approval, notification, execution, delay, parallel
	Name        string    `json:"name" gorm:"type:varchar(200);not null"`
	Description string    `json:"description,omitempty" gorm:"type:text"`
	Config      JSON      `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	PositionX   int       `json:"position_x" gorm:"default:0"`
	PositionY   int       `json:"position_y" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"default:now()"`
}

// TableName 表名
func (WorkflowNode) TableName() string {
	return "workflow_nodes"
}

// WorkflowEdge 工作流边
type WorkflowEdge struct {
	ID                  uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	WorkflowID          uuid.UUID `json:"workflow_id" gorm:"type:uuid;not null"`
	SourceNodeID        uuid.UUID `json:"source_node_id" gorm:"type:uuid;not null"`
	TargetNodeID        uuid.UUID `json:"target_node_id" gorm:"type:uuid;not null"`
	ConditionExpression string    `json:"condition_expression,omitempty" gorm:"type:text"`
	Label               string    `json:"label,omitempty" gorm:"type:varchar(100)"`
	Priority            int       `json:"priority" gorm:"default:0"`
	CreatedAt           time.Time `json:"created_at" gorm:"default:now()"`

	// 关联
	SourceNode *WorkflowNode `json:"source_node,omitempty" gorm:"foreignKey:SourceNodeID"`
	TargetNode *WorkflowNode `json:"target_node,omitempty" gorm:"foreignKey:TargetNodeID"`
}

// TableName 表名
func (WorkflowEdge) TableName() string {
	return "workflow_edges"
}

// WorkflowInstance 工作流实例
type WorkflowInstance struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	WorkflowID    uuid.UUID  `json:"workflow_id" gorm:"type:uuid;not null"`
	IncidentID    *uuid.UUID `json:"incident_id,omitempty" gorm:"type:uuid"`
	Status        string     `json:"status" gorm:"type:varchar(50);default:'running'"` // pending, running, paused, completed, failed, cancelled
	CurrentNodeID *uuid.UUID `json:"current_node_id,omitempty" gorm:"type:uuid"`
	Context       JSON       `json:"context" gorm:"type:jsonb;default:'{}'"`
	StartedAt     time.Time  `json:"started_at" gorm:"default:now()"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`

	// 关联
	Workflow       Workflow        `json:"workflow,omitempty" gorm:"foreignKey:WorkflowID"`
	Incident       *Incident       `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
	CurrentNode    *WorkflowNode   `json:"current_node,omitempty" gorm:"foreignKey:CurrentNodeID"`
	NodeExecutions []NodeExecution `json:"node_executions,omitempty" gorm:"foreignKey:WorkflowInstanceID"`
}

// TableName 表名
func (WorkflowInstance) TableName() string {
	return "workflow_instances"
}

// NodeExecution 节点执行记录
type NodeExecution struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	WorkflowInstanceID uuid.UUID  `json:"workflow_instance_id" gorm:"type:uuid;not null"`
	NodeID             uuid.UUID  `json:"node_id" gorm:"type:uuid;not null"`
	Status             string     `json:"status" gorm:"type:varchar(50);not null"` // pending, running, success, failed, skipped
	InputData          JSON       `json:"input_data" gorm:"type:jsonb;default:'{}'"`
	OutputData         JSON       `json:"output_data" gorm:"type:jsonb;default:'{}'"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	RetryCount         int        `json:"retry_count" gorm:"default:0"`
	ErrorMessage       string     `json:"error_message,omitempty" gorm:"type:text"`

	// 关联
	Node *WorkflowNode `json:"node,omitempty" gorm:"foreignKey:NodeID"`
}

// TableName 表名
func (NodeExecution) TableName() string {
	return "node_executions"
}
