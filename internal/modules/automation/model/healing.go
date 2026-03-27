package model

import (
	"time"

	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

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

	Creator *accessmodel.User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

func (HealingFlow) TableName() string {
	return "healing_flows"
}

type HealingRule struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name        string     `json:"name" gorm:"type:varchar(255);not null"`
	Description string     `json:"description,omitempty" gorm:"type:text"`
	Priority    int        `json:"priority" gorm:"default:0"`
	TriggerMode string     `json:"trigger_mode" gorm:"type:varchar(20);default:'auto'"`
	Conditions  JSONArray  `json:"conditions" gorm:"type:jsonb;not null;default:'[]'"`
	MatchMode   string     `json:"match_mode" gorm:"type:varchar(10);default:'all'"`
	FlowID      *uuid.UUID `json:"flow_id,omitempty" gorm:"type:uuid;index"`
	IsActive    bool       `json:"is_active" gorm:"default:false"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"default:now()"`

	Flow    *HealingFlow      `json:"flow,omitempty" gorm:"foreignKey:FlowID"`
	Creator *accessmodel.User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

func (HealingRule) TableName() string {
	return "healing_rules"
}

type FlowInstance struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	FlowID        uuid.UUID  `json:"flow_id" gorm:"type:uuid;not null;index"`
	RuleID        *uuid.UUID `json:"rule_id,omitempty" gorm:"type:uuid;index"`
	IncidentID    *uuid.UUID `json:"incident_id,omitempty" gorm:"type:uuid;index"`
	Status        string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	CurrentNodeID string     `json:"current_node_id,omitempty" gorm:"type:varchar(100)"`
	Context       JSON       `json:"context" gorm:"type:jsonb;not null;default:'{}'"`
	NodeStates    JSON       `json:"node_states" gorm:"type:jsonb;not null;default:'{}'"`
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"default:now()"`

	FlowName  string    `json:"flow_name" gorm:"type:varchar(255)"`
	FlowNodes JSONArray `json:"flow_nodes" gorm:"type:jsonb;default:'[]'"`
	FlowEdges JSONArray `json:"flow_edges" gorm:"type:jsonb;default:'[]'"`

	Flow     *HealingFlow            `json:"-" gorm:"foreignKey:FlowID"`
	Rule     *HealingRule            `json:"rule,omitempty" gorm:"foreignKey:RuleID"`
	Incident *platformmodel.Incident `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
}

func (FlowInstance) TableName() string {
	return "flow_instances"
}

const (
	FlowInstanceStatusPending         = "pending"
	FlowInstanceStatusRunning         = "running"
	FlowInstanceStatusWaitingApproval = "waiting_approval"
	FlowInstanceStatusCompleted       = "completed"
	FlowInstanceStatusFailed          = "failed"
	FlowInstanceStatusCancelled       = "cancelled"
)

type ApprovalTask struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID         *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	FlowInstanceID   uuid.UUID  `json:"flow_instance_id" gorm:"type:uuid;not null;index"`
	NodeID           string     `json:"node_id" gorm:"type:varchar(100);not null"`
	InitiatedBy      *uuid.UUID `json:"initiated_by,omitempty" gorm:"type:uuid"`
	Approvers        JSONArray  `json:"approvers" gorm:"type:jsonb;not null;default:'[]'"`
	ApproverRoles    JSONArray  `json:"approver_roles" gorm:"type:jsonb;not null;default:'[]'"`
	Status           string     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	TimeoutAt        *time.Time `json:"timeout_at,omitempty"`
	DecidedBy        *uuid.UUID `json:"decided_by,omitempty" gorm:"type:uuid"`
	DecidedAt        *time.Time `json:"decided_at,omitempty"`
	DecisionComment  string     `json:"decision_comment,omitempty" gorm:"type:text"`
	NotificationSent bool       `json:"notification_sent" gorm:"default:false"`
	CreatedAt        time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"default:now()"`

	FlowInstance *FlowInstance     `json:"flow_instance,omitempty" gorm:"foreignKey:FlowInstanceID"`
	Initiator    *accessmodel.User `json:"initiator,omitempty" gorm:"foreignKey:InitiatedBy"`
	Decider      *accessmodel.User `json:"decider,omitempty" gorm:"foreignKey:DecidedBy"`
}

func (ApprovalTask) TableName() string {
	return "approval_tasks"
}

const (
	ApprovalTaskStatusPending   = "pending"
	ApprovalTaskStatusApproved  = "approved"
	ApprovalTaskStatusRejected  = "rejected"
	ApprovalTaskStatusExpired   = "expired"
	ApprovalTaskStatusCancelled = "cancelled"
)
