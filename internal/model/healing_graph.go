package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	NodeStatusPending         = "pending"
	NodeStatusRunning         = "running"
	NodeStatusSuccess         = "success"
	NodeStatusPartial         = "partial"
	NodeStatusFailed          = "failed"
	NodeStatusSkipped         = "skipped"
	NodeStatusWaitingApproval = "waiting_approval"
)

const (
	SSEEventFlowStart    = "flow_start"
	SSEEventNodeStart    = "node_start"
	SSEEventNodeLog      = "node_log"
	SSEEventNodeComplete = "node_complete"
	SSEEventFlowComplete = "flow_complete"
)

const (
	TriggerModeAuto   = "auto"
	TriggerModeManual = "manual"
)

const (
	MatchModeAll = "all"
	MatchModeAny = "any"
)

type FlowNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Position *FlowNodePosition      `json:"position,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

type FlowNodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type FlowEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle"`
	Condition    string `json:"condition,omitempty"`
}

func (e *FlowEdge) GetFrom() string {
	if e.Source != "" {
		return e.Source
	}
	return e.From
}

func (e *FlowEdge) GetTo() string {
	if e.Target != "" {
		return e.Target
	}
	return e.To
}

func (e *FlowEdge) GetSourceHandle() string {
	if e.SourceHandle != "" {
		return e.SourceHandle
	}
	return "default"
}

type RuleCondition struct {
	Type       string          `json:"type"`
	Field      string          `json:"field,omitempty"`
	Operator   string          `json:"operator,omitempty"`
	Value      interface{}     `json:"value,omitempty"`
	Logic      string          `json:"logic,omitempty"`
	Conditions []RuleCondition `json:"conditions,omitempty"`
}

func (c *RuleCondition) IsGroup() bool {
	return c.Type == "group"
}

func (c *RuleCondition) IsCondition() bool {
	return c.Type == "condition" || c.Type == ""
}

const (
	NodeTypeStart         = "start"
	NodeTypeEnd           = "end"
	NodeTypeHostExtractor = "host_extractor"
	NodeTypeCMDBValidator = "cmdb_validator"
	NodeTypeApproval      = "approval"
	NodeTypeExecution     = "execution"
	NodeTypeNotification  = "notification"
	NodeTypeCondition     = "condition"
	NodeTypeSetVariable   = "set_variable"
	NodeTypeCompute       = "compute"
)

const (
	OperatorEquals   = "equals"
	OperatorContains = "contains"
	OperatorIn       = "in"
	OperatorRegex    = "regex"
	OperatorGt       = "gt"
	OperatorLt       = "lt"
	OperatorGte      = "gte"
	OperatorLte      = "lte"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

type FlowExecutionLog struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	FlowInstanceID uuid.UUID  `json:"flow_instance_id" gorm:"type:uuid;not null;index"`
	NodeID         string     `json:"node_id" gorm:"type:varchar(100);not null"`
	NodeType       string     `json:"node_type" gorm:"type:varchar(50);not null"`
	Level          string     `json:"level" gorm:"type:varchar(20);not null;default:'info'"`
	Message        string     `json:"message" gorm:"type:text;not null"`
	Details        JSON       `json:"details,omitempty" gorm:"type:jsonb"`
	CreatedAt      time.Time  `json:"created_at" gorm:"default:now()"`

	FlowInstance *FlowInstance `json:"-" gorm:"foreignKey:FlowInstanceID"`
}

func (FlowExecutionLog) TableName() string {
	return "flow_execution_logs"
}
