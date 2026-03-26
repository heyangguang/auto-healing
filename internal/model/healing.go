package model

import (
	"time"

	"github.com/google/uuid"
)

// HealingFlow 自愈流程
type HealingFlow struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name        string     `json:"name" gorm:"type:varchar(255);not null"`
	Description string     `json:"description,omitempty" gorm:"type:text"`
	Nodes       JSONArray  `json:"nodes" gorm:"type:jsonb;not null;default:'[]'"`
	Edges       JSONArray  `json:"edges" gorm:"type:jsonb;not null;default:'[]'"`
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	Creator *User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

// TableName 表名
func (HealingFlow) TableName() string {
	return "healing_flows"
}

// HealingRule 自愈规则
type HealingRule struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name        string     `json:"name" gorm:"type:varchar(255);not null"`
	Description string     `json:"description,omitempty" gorm:"type:text"`
	Priority    int        `json:"priority" gorm:"default:0"`
	TriggerMode string     `json:"trigger_mode" gorm:"type:varchar(20);default:'auto'"` // auto | manual
	Conditions  JSONArray  `json:"conditions" gorm:"type:jsonb;not null;default:'[]'"`
	MatchMode   string     `json:"match_mode" gorm:"type:varchar(10);default:'all'"` // all | any
	FlowID      *uuid.UUID `json:"flow_id,omitempty" gorm:"type:uuid;index"`
	IsActive    bool       `json:"is_active" gorm:"default:false"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	Flow    *HealingFlow `json:"flow,omitempty" gorm:"foreignKey:FlowID"`
	Creator *User        `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

// TableName 表名
func (HealingRule) TableName() string {
	return "healing_rules"
}

// FlowInstance 流程实例
type FlowInstance struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	FlowID        uuid.UUID  `json:"flow_id" gorm:"type:uuid;not null;index"`
	RuleID        *uuid.UUID `json:"rule_id,omitempty" gorm:"type:uuid;index"`
	IncidentID    *uuid.UUID `json:"incident_id,omitempty" gorm:"type:uuid;index"`
	Status        string     `json:"status" gorm:"type:varchar(20);default:'pending'"` // pending | running | waiting_approval | completed | failed | cancelled
	CurrentNodeID string     `json:"current_node_id,omitempty" gorm:"type:varchar(100)"`
	Context       JSON       `json:"context" gorm:"type:jsonb;not null;default:'{}'"`
	NodeStates    JSON       `json:"node_states" gorm:"type:jsonb;not null;default:'{}'"`
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"default:now()"`

	// 流程定义快照（创建时固化，不随流程修改而变化）
	FlowName  string    `json:"flow_name" gorm:"type:varchar(255)"`
	FlowNodes JSONArray `json:"flow_nodes" gorm:"type:jsonb;default:'[]'"`
	FlowEdges JSONArray `json:"flow_edges" gorm:"type:jsonb;default:'[]'"`

	// 关联
	Flow     *HealingFlow `json:"-" gorm:"foreignKey:FlowID"`
	Rule     *HealingRule `json:"rule,omitempty" gorm:"foreignKey:RuleID"`
	Incident *Incident    `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
}

// TableName 表名
func (FlowInstance) TableName() string {
	return "flow_instances"
}

// FlowInstanceStatus 流程实例状态常量
const (
	FlowInstanceStatusPending         = "pending"
	FlowInstanceStatusRunning         = "running"
	FlowInstanceStatusWaitingApproval = "waiting_approval"
	FlowInstanceStatusCompleted       = "completed"
	FlowInstanceStatusFailed          = "failed"
	FlowInstanceStatusCancelled       = "cancelled"
)

// ApprovalTask 审批任务
type ApprovalTask struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID         *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	FlowInstanceID   uuid.UUID  `json:"flow_instance_id" gorm:"type:uuid;not null;index"`
	NodeID           string     `json:"node_id" gorm:"type:varchar(100);not null"`
	InitiatedBy      *uuid.UUID `json:"initiated_by,omitempty" gorm:"type:uuid"`
	Approvers        JSONArray  `json:"approvers" gorm:"type:jsonb;not null;default:'[]'"`
	ApproverRoles    JSONArray  `json:"approver_roles" gorm:"type:jsonb;not null;default:'[]'"`
	Status           string     `json:"status" gorm:"type:varchar(20);default:'pending'"` // pending | approved | rejected | expired
	TimeoutAt        *time.Time `json:"timeout_at,omitempty"`
	DecidedBy        *uuid.UUID `json:"decided_by,omitempty" gorm:"type:uuid"`
	DecidedAt        *time.Time `json:"decided_at,omitempty"`
	DecisionComment  string     `json:"decision_comment,omitempty" gorm:"type:text"`
	NotificationSent bool       `json:"notification_sent" gorm:"default:false"`
	CreatedAt        time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	FlowInstance *FlowInstance `json:"flow_instance,omitempty" gorm:"foreignKey:FlowInstanceID"`
	Initiator    *User         `json:"initiator,omitempty" gorm:"foreignKey:InitiatedBy"`
	Decider      *User         `json:"decider,omitempty" gorm:"foreignKey:DecidedBy"`
}

// TableName 表名
func (ApprovalTask) TableName() string {
	return "approval_tasks"
}

// ApprovalTaskStatus 审批任务状态常量
const (
	ApprovalTaskStatusPending   = "pending"
	ApprovalTaskStatusApproved  = "approved"
	ApprovalTaskStatusRejected  = "rejected"
	ApprovalTaskStatusExpired   = "expired"
	ApprovalTaskStatusCancelled = "cancelled"
)
