package model

import (
	"time"

	"github.com/google/uuid"
)

// HealingFlow 自愈流程
type HealingFlow struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
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

	// 关联
	Flow     *HealingFlow `json:"flow,omitempty" gorm:"foreignKey:FlowID"`
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
	ApprovalTaskStatusPending  = "pending"
	ApprovalTaskStatusApproved = "approved"
	ApprovalTaskStatusRejected = "rejected"
	ApprovalTaskStatusExpired  = "expired"
)

// NodeStatus 节点执行状态常量
const (
	NodeStatusPending         = "pending"          // 等待执行
	NodeStatusRunning         = "running"          // 执行中
	NodeStatusSuccess         = "success"          // 执行成功
	NodeStatusPartial         = "partial"          // 部分成功（仅 execution 节点）
	NodeStatusFailed          = "failed"           // 执行失败
	NodeStatusSkipped         = "skipped"          // 因分支条件被跳过
	NodeStatusWaitingApproval = "waiting_approval" // 等待审批（仅 approval 节点）
)

// SSE 事件类型常量
const (
	SSEEventFlowStart    = "flow_start"
	SSEEventNodeStart    = "node_start"
	SSEEventNodeLog      = "node_log"
	SSEEventNodeComplete = "node_complete"
	SSEEventFlowComplete = "flow_complete"
)

// TriggerMode 触发模式常量
const (
	TriggerModeAuto   = "auto"
	TriggerModeManual = "manual"
)

// MatchMode 匹配模式常量
const (
	MatchModeAll = "all"
	MatchModeAny = "any"
)

// FlowNode 流程节点（用于 nodes JSONB 字段的解析）
type FlowNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Position *FlowNodePosition      `json:"position,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// FlowNodePosition 节点位置
type FlowNodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// FlowEdge 流程边（用于 edges JSONB 字段的解析）
// 支持两种格式: from/to 或 source/target
type FlowEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Source       string `json:"source"`       // 别名，优先使用
	Target       string `json:"target"`       // 别名，优先使用
	SourceHandle string `json:"sourceHandle"` // 源节点的输出口ID（如 success, failed, approved 等）
	Condition    string `json:"condition,omitempty"`
}

// GetFrom 获取源节点ID（兼容 from/source）
func (e *FlowEdge) GetFrom() string {
	if e.Source != "" {
		return e.Source
	}
	return e.From
}

// GetTo 获取目标节点ID（兼容 to/target）
func (e *FlowEdge) GetTo() string {
	if e.Target != "" {
		return e.Target
	}
	return e.To
}

// GetSourceHandle 获取源节点的输出口ID
// 如果没有指定，返回 "default"
func (e *FlowEdge) GetSourceHandle() string {
	if e.SourceHandle != "" {
		return e.SourceHandle
	}
	return "default"
}

// RuleCondition 规则条件（用于 conditions JSONB 字段的解析）
// 支持两种类型：
// 1. "condition" - 单个条件（包含 field, operator, value）
// 2. "group" - 条件组（包含 logic 和嵌套的 conditions）
type RuleCondition struct {
	// 类型标识
	Type string `json:"type"` // "condition" | "group"

	// ========== 条件字段 (type="condition" 时使用) ==========
	Field    string      `json:"field,omitempty"`
	Operator string      `json:"operator,omitempty"`
	Value    interface{} `json:"value,omitempty"`

	// ========== 条件组字段 (type="group" 时使用) ==========
	Logic      string          `json:"logic,omitempty"`      // "AND" | "OR"
	Conditions []RuleCondition `json:"conditions,omitempty"` // 递归嵌套
}

// IsGroup 判断是否为条件组
func (c *RuleCondition) IsGroup() bool {
	return c.Type == "group"
}

// IsCondition 判断是否为单个条件
func (c *RuleCondition) IsCondition() bool {
	return c.Type == "condition" || c.Type == ""
}

// FlowNodeType 节点类型常量
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
	NodeTypeCompute       = "compute" // 计算节点：执行表达式计算，结果写入 context
)

// ConditionOperator 条件操作符常量
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

// FlowLogLevel 日志级别常量
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// FlowExecutionLog 流程执行日志
type FlowExecutionLog struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	FlowInstanceID uuid.UUID `json:"flow_instance_id" gorm:"type:uuid;not null;index"`
	NodeID         string    `json:"node_id" gorm:"type:varchar(100);not null"`
	NodeType       string    `json:"node_type" gorm:"type:varchar(50);not null"`
	Level          string    `json:"level" gorm:"type:varchar(20);not null;default:'info'"`
	Message        string    `json:"message" gorm:"type:text;not null"`
	Details        JSON      `json:"details,omitempty" gorm:"type:jsonb"`
	CreatedAt      time.Time `json:"created_at" gorm:"default:now()"`

	// 关联
	FlowInstance *FlowInstance `json:"-" gorm:"foreignKey:FlowInstanceID"`
}

// TableName 表名
func (FlowExecutionLog) TableName() string {
	return "flow_execution_logs"
}
