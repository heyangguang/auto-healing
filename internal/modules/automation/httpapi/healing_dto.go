package httpapi

import (
	"encoding/json"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== HealingFlow 请求体 ==========

// CreateFlowRequest 创建自愈流程请求
type CreateFlowRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Nodes       json.RawMessage `json:"nodes"`
	Edges       json.RawMessage `json:"edges"`
	IsActive    *bool           `json:"is_active"`
}

// ToModel 转换为模型
func (r *CreateFlowRequest) ToModel() *model.HealingFlow {
	flow := &model.HealingFlow{
		Name:        r.Name,
		Description: r.Description,
		IsActive:    true,
	}
	if r.Nodes != nil {
		var nodes model.JSONArray
		if err := json.Unmarshal(r.Nodes, &nodes); err == nil {
			flow.Nodes = nodes
		}
	}
	if r.Edges != nil {
		var edges model.JSONArray
		if err := json.Unmarshal(r.Edges, &edges); err == nil {
			flow.Edges = edges
		}
	}
	if r.IsActive != nil {
		flow.IsActive = *r.IsActive
	}
	return flow
}

// UpdateFlowRequest 更新自愈流程请求
type UpdateFlowRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Nodes       json.RawMessage `json:"nodes"`
	Edges       json.RawMessage `json:"edges"`
	IsActive    *bool           `json:"is_active"`
}

// ApplyTo 应用更新到模型
func (r *UpdateFlowRequest) ApplyTo(flow *model.HealingFlow) {
	if r.Name != nil {
		flow.Name = *r.Name
	}
	if r.Description != nil {
		flow.Description = *r.Description
	}
	if r.Nodes != nil {
		var nodes model.JSONArray
		if err := json.Unmarshal(r.Nodes, &nodes); err == nil {
			flow.Nodes = nodes
		}
	}
	if r.Edges != nil {
		var edges model.JSONArray
		if err := json.Unmarshal(r.Edges, &edges); err == nil {
			flow.Edges = edges
		}
	}
	if r.IsActive != nil {
		flow.IsActive = *r.IsActive
	}
}

// DryRunFlowRequest Dry-Run 自愈流程请求
type DryRunFlowRequest struct {
	MockIncident  MockIncidentRequest    `json:"mock_incident" binding:"required"`
	FromNodeID    string                 `json:"from_node_id,omitempty"`   // 从哪个节点开始（用于重试）
	Context       map[string]interface{} `json:"context,omitempty"`        // 初始上下文（用于重试）
	MockApprovals map[string]string      `json:"mock_approvals,omitempty"` // 模拟审批结果: node_id -> "approved" | "rejected"
}

// MockIncidentRequest 模拟工单请求
type MockIncidentRequest struct {
	Title           string                 `json:"title" binding:"required"`
	Description     string                 `json:"description,omitempty"`
	Severity        string                 `json:"severity,omitempty"`
	Priority        string                 `json:"priority,omitempty"`
	Status          string                 `json:"status,omitempty"`
	Category        string                 `json:"category,omitempty"`
	AffectedCI      string                 `json:"affected_ci,omitempty"`
	AffectedService string                 `json:"affected_service,omitempty"`
	Assignee        string                 `json:"assignee,omitempty"`
	Reporter        string                 `json:"reporter,omitempty"`
	RawData         map[string]interface{} `json:"raw_data,omitempty"`
}

// ========== HealingRule 请求体 ==========

// CreateRuleRequest 创建自愈规则请求
type CreateRuleRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Priority    int             `json:"priority"`
	TriggerMode string          `json:"trigger_mode"` // auto | manual
	Conditions  json.RawMessage `json:"conditions"`
	MatchMode   string          `json:"match_mode"` // all | any
	FlowID      *uuid.UUID      `json:"flow_id"`
	IsActive    *bool           `json:"is_active"`
}

// ToModel 转换为模型
func (r *CreateRuleRequest) ToModel() *model.HealingRule {
	rule := &model.HealingRule{
		Name:        r.Name,
		Description: r.Description,
		Priority:    r.Priority,
		TriggerMode: r.TriggerMode,
		MatchMode:   r.MatchMode,
		FlowID:      r.FlowID,
		IsActive:    false,
	}
	if rule.TriggerMode == "" {
		rule.TriggerMode = model.TriggerModeAuto
	}
	if rule.MatchMode == "" {
		rule.MatchMode = model.MatchModeAll
	}
	if r.Conditions != nil {
		var conditions model.JSONArray
		if err := json.Unmarshal(r.Conditions, &conditions); err == nil {
			rule.Conditions = conditions
		}
	}
	if r.IsActive != nil {
		rule.IsActive = *r.IsActive
	}
	return rule
}

// UpdateRuleRequest 更新自愈规则请求
type UpdateRuleRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Priority    *int            `json:"priority"`
	TriggerMode *string         `json:"trigger_mode"`
	Conditions  json.RawMessage `json:"conditions"`
	MatchMode   *string         `json:"match_mode"`
	FlowID      *uuid.UUID      `json:"flow_id"`
	IsActive    *bool           `json:"is_active"`
	// FlowIDSet 标识 flow_id 是否在请求中被设置(包括设为null)
	FlowIDSet bool `json:"-"`
}

// UnmarshalJSON 自定义JSON解析，检测flow_id是否被设置
func (r *UpdateRuleRequest) UnmarshalJSON(data []byte) error {
	// 使用别名避免递归
	type Alias UpdateRuleRequest
	aux := &struct {
		*Alias
	}{Alias: (*Alias)(r)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// 检查flow_id字段是否存在于JSON中
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if _, exists := raw["flow_id"]; exists {
			r.FlowIDSet = true
		}
	}
	return nil
}

// ApplyTo 应用更新到模型
func (r *UpdateRuleRequest) ApplyTo(rule *model.HealingRule) {
	if r.Name != nil {
		rule.Name = *r.Name
	}
	if r.Description != nil {
		rule.Description = *r.Description
	}
	if r.Priority != nil {
		rule.Priority = *r.Priority
	}
	if r.TriggerMode != nil {
		rule.TriggerMode = *r.TriggerMode
	}
	if r.Conditions != nil {
		var conditions model.JSONArray
		if err := json.Unmarshal(r.Conditions, &conditions); err == nil {
			rule.Conditions = conditions
		}
	}
	if r.MatchMode != nil {
		rule.MatchMode = *r.MatchMode
	}
	// 使用 FlowIDSet 判断是否需要更新（支持设为null）
	if r.FlowIDSet {
		rule.FlowID = r.FlowID
	}
	if r.IsActive != nil {
		rule.IsActive = *r.IsActive
	}
}

// ========== Approval 请求体 ==========

// ApproveRequest 审批请求
type ApproveRequest struct {
	Comment string `json:"comment"`
}

// ========== 辅助函数 ==========

// getCurrentUserID 从 context 获取当前用户ID
func getCurrentUserID(c *gin.Context) *uuid.UUID {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		return nil
	}

	// middleware 存储的是字符串类型的 UUID
	switch v := userIDVal.(type) {
	case string:
		// 解析字符串为 UUID
		userID, err := uuid.Parse(v)
		if err != nil {
			return nil
		}
		return &userID
	case uuid.UUID:
		return &v
	default:
		return nil
	}
}
