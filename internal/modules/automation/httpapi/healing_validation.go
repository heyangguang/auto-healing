package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

var allowedHealingNodeTypes = map[string]struct{}{
	model.NodeTypeStart:         {},
	model.NodeTypeEnd:           {},
	model.NodeTypeHostExtractor: {},
	model.NodeTypeCMDBValidator: {},
	model.NodeTypeApproval:      {},
	model.NodeTypeExecution:     {},
	model.NodeTypeNotification:  {},
	model.NodeTypeCondition:     {},
	model.NodeTypeSetVariable:   {},
	model.NodeTypeCompute:       {},
}

var allowedRuleOperators = map[string]struct{}{
	model.OperatorEquals:   {},
	model.OperatorContains: {},
	model.OperatorIn:       {},
	model.OperatorRegex:    {},
	model.OperatorGt:       {},
	model.OperatorLt:       {},
	model.OperatorGte:      {},
	model.OperatorLte:      {},
}

func validateRawJSONArray(raw json.RawMessage, field string) error {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var values []interface{}
	if err := decoder.Decode(&values); err != nil {
		return fmt.Errorf("%s 必须是合法 JSON 数组: %w", field, err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s 只能包含一个 JSON 数组", field)
	}
	if values == nil {
		return fmt.Errorf("%s 不能为 null", field)
	}
	return nil
}

func validateRawJSONObject(raw json.RawMessage, field string) error {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value map[string]interface{}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s 必须是合法 JSON 对象: %w", field, err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s 只能包含一个 JSON 对象", field)
	}
	if value == nil {
		return fmt.Errorf("%s 不能为 null", field)
	}
	return nil
}

func (r *CreateFlowRequest) ValidatePayload() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("流程名称不能为空")
	}
	if err := validateRawJSONArray(r.Nodes, "nodes"); err != nil {
		return err
	}
	if err := validateRawJSONArray(r.Edges, "edges"); err != nil {
		return err
	}
	return validateRawJSONObject(r.ClosePolicy, "close_policy")
}

func (r *UpdateFlowRequest) ValidatePayload() error {
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		return fmt.Errorf("流程名称不能为空")
	}
	if err := validateRawJSONArray(r.Nodes, "nodes"); err != nil {
		return err
	}
	if err := validateRawJSONArray(r.Edges, "edges"); err != nil {
		return err
	}
	return validateRawJSONObject(r.ClosePolicy, "close_policy")
}

func (r *CreateRuleRequest) ValidatePayload() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if err := validateTriggerMode(defaultString(r.TriggerMode, model.TriggerModeAuto)); err != nil {
		return err
	}
	if err := validateMatchMode(defaultString(r.MatchMode, model.MatchModeAll)); err != nil {
		return err
	}
	if err := validateRawJSONArray(r.Conditions, "conditions"); err != nil {
		return err
	}
	return validateRuleConditionsRaw(r.Conditions)
}

func (r *UpdateRuleRequest) ValidatePayload() error {
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if r.TriggerMode != nil {
		if err := validateTriggerMode(*r.TriggerMode); err != nil {
			return err
		}
	}
	if r.MatchMode != nil {
		if err := validateMatchMode(*r.MatchMode); err != nil {
			return err
		}
	}
	if err := validateRawJSONArray(r.Conditions, "conditions"); err != nil {
		return err
	}
	return validateRuleConditionsRaw(r.Conditions)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func validateTriggerMode(value string) error {
	switch value {
	case model.TriggerModeAuto, model.TriggerModeManual:
		return nil
	default:
		return fmt.Errorf("trigger_mode 仅支持 auto 或 manual")
	}
}

func validateMatchMode(value string) error {
	switch value {
	case model.MatchModeAll, model.MatchModeAny:
		return nil
	default:
		return fmt.Errorf("match_mode 仅支持 all 或 any")
	}
}

func validateRuleConditionsRaw(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var conditions []model.RuleCondition
	if err := json.Unmarshal(raw, &conditions); err != nil {
		return fmt.Errorf("conditions 结构不合法: %w", err)
	}
	for i, condition := range conditions {
		if err := validateRuleCondition(condition); err != nil {
			return fmt.Errorf("conditions[%d] %w", i, err)
		}
	}
	return nil
}

func validateRuleCondition(condition model.RuleCondition) error {
	switch condition.Type {
	case "", "condition":
		if strings.TrimSpace(condition.Field) == "" {
			return fmt.Errorf("field 不能为空")
		}
		if _, ok := allowedRuleOperators[condition.Operator]; !ok {
			return fmt.Errorf("operator 不支持: %s", condition.Operator)
		}
		if condition.Operator == model.OperatorRegex {
			pattern, ok := condition.Value.(string)
			if !ok || pattern == "" {
				return fmt.Errorf("regex 操作符的 value 必须是非空字符串")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("regex value 不合法: %w", err)
			}
		}
		if condition.Operator == model.OperatorIn {
			if _, ok := condition.Value.([]interface{}); !ok {
				return fmt.Errorf("in 操作符的 value 必须是数组")
			}
		}
		return nil
	case "group":
		if condition.Logic != "" && condition.Logic != "AND" && condition.Logic != "OR" {
			return fmt.Errorf("group logic 仅支持 AND 或 OR")
		}
		if len(condition.Conditions) == 0 {
			return fmt.Errorf("group conditions 不能为空")
		}
		for i, child := range condition.Conditions {
			if err := validateRuleCondition(child); err != nil {
				return fmt.Errorf("group.conditions[%d] %w", i, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("type 不支持: %s", condition.Type)
	}
}

func (h *HealingHandler) validateHealingFlow(ctx context.Context, flow *model.HealingFlow) error {
	nodes, err := decodeFlowNodes(flow.Nodes)
	if err != nil {
		return err
	}
	edges, err := decodeFlowEdges(flow.Edges)
	if err != nil {
		return err
	}
	if err := h.validateFlowGraph(ctx, nodes, edges); err != nil {
		return err
	}
	return h.validateClosePolicy(ctx, flow)
}

func decodeFlowNodes(raw model.JSONArray) ([]model.FlowNode, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("nodes 序列化失败: %w", err)
	}
	var nodes []model.FlowNode
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("nodes 结构不合法: %w", err)
	}
	return nodes, nil
}

func decodeFlowEdges(raw model.JSONArray) ([]model.FlowEdge, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("edges 序列化失败: %w", err)
	}
	var rawEdges []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawEdges); err != nil {
		return nil, fmt.Errorf("edges 结构不合法: %w", err)
	}
	for i, edge := range rawEdges {
		if _, exists := edge["from"]; exists {
			return nil, fmt.Errorf("edges[%d].from 已废弃，请使用 source", i)
		}
		if _, exists := edge["to"]; exists {
			return nil, fmt.Errorf("edges[%d].to 已废弃，请使用 target", i)
		}
	}
	var edges []model.FlowEdge
	if err := json.Unmarshal(data, &edges); err != nil {
		return nil, fmt.Errorf("edges 结构不合法: %w", err)
	}
	return edges, nil
}

func (h *HealingHandler) validateFlowGraph(ctx context.Context, nodes []model.FlowNode, edges []model.FlowEdge) error {
	nodeIDs := make(map[string]struct{}, len(nodes))
	for i, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("nodes[%d].id 不能为空", i)
		}
		if _, exists := nodeIDs[node.ID]; exists {
			return fmt.Errorf("nodes[%d].id 重复: %s", i, node.ID)
		}
		nodeIDs[node.ID] = struct{}{}
		if _, ok := allowedHealingNodeTypes[node.Type]; !ok {
			return fmt.Errorf("nodes[%d].type 不支持: %s", i, node.Type)
		}
		if err := h.validateFlowNodeReference(ctx, i, node); err != nil {
			return err
		}
	}
	for i, edge := range edges {
		if edge.Source == "" || edge.Target == "" {
			return fmt.Errorf("edges[%d] 必须配置 source/target", i)
		}
		if _, ok := nodeIDs[edge.Source]; !ok {
			return fmt.Errorf("edges[%d].source 引用了不存在的节点: %s", i, edge.Source)
		}
		if _, ok := nodeIDs[edge.Target]; !ok {
			return fmt.Errorf("edges[%d].target 引用了不存在的节点: %s", i, edge.Target)
		}
	}
	return nil
}

func (h *HealingHandler) validateFlowNodeReference(ctx context.Context, index int, node model.FlowNode) error {
	switch node.Type {
	case model.NodeTypeExecution:
		return h.validateExecutionNodeReference(ctx, index, node.Config)
	case model.NodeTypeNotification:
		return h.validateNotificationNodeReference(ctx, index, node.Config)
	default:
		return nil
	}
}

func (h *HealingHandler) validateExecutionNodeReference(ctx context.Context, index int, config map[string]interface{}) error {
	taskID, _ := config["task_template_id"].(string)
	if taskID == "" {
		return fmt.Errorf("nodes[%d].config.task_template_id 不能为空", index)
	}
	id, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("nodes[%d].config.task_template_id 不是合法 UUID", index)
	}
	if h.executionRepo == nil {
		return nil
	}
	if _, err := h.executionRepo.GetTaskByID(ctx, id); err != nil {
		return fmt.Errorf("nodes[%d].config.task_template_id 引用了不存在的任务模板: %s", index, taskID)
	}
	return nil
}

func (h *HealingHandler) validateNotificationNodeReference(ctx context.Context, index int, config map[string]interface{}) error {
	templateIDs, channelIDs, err := extractNotificationReferenceIDs(config)
	if err != nil {
		return fmt.Errorf("nodes[%d].config.%w", index, err)
	}
	if len(templateIDs) == 0 {
		return fmt.Errorf("nodes[%d].config.template_id 不能为空", index)
	}
	if len(channelIDs) == 0 {
		return fmt.Errorf("nodes[%d].config.channel_ids 不能为空", index)
	}
	if h.notifRepo == nil {
		return nil
	}
	templates, err := h.notifRepo.GetTemplatesByIDs(ctx, templateIDs)
	if err != nil || len(templates) != len(templateIDs) {
		return fmt.Errorf("nodes[%d].config.template_id 引用了不存在的通知模板", index)
	}
	channels, err := h.notifRepo.GetChannelsByIDs(ctx, channelIDs)
	if err != nil || len(channels) != len(channelIDs) {
		return fmt.Errorf("nodes[%d].config.channel_ids 引用了不存在的通知渠道", index)
	}
	return nil
}

func extractNotificationReferenceIDs(config map[string]interface{}) ([]uuid.UUID, []uuid.UUID, error) {
	templateIDs := make(map[uuid.UUID]struct{})
	channelIDs := make(map[uuid.UUID]struct{})
	addTemplate := func(raw string) error {
		id, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("template_id 不是合法 UUID")
		}
		templateIDs[id] = struct{}{}
		return nil
	}
	addChannel := func(raw string) error {
		id, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("channel_id 不是合法 UUID")
		}
		channelIDs[id] = struct{}{}
		return nil
	}

	if rawConfigs, ok := config["notification_configs"].([]interface{}); ok && len(rawConfigs) > 0 {
		for i, raw := range rawConfigs {
			cfg, ok := raw.(map[string]interface{})
			if !ok {
				return nil, nil, fmt.Errorf("notification_configs[%d] 必须是对象", i)
			}
			channelID, _ := cfg["channel_id"].(string)
			templateID, _ := cfg["template_id"].(string)
			if channelID == "" || templateID == "" {
				return nil, nil, fmt.Errorf("notification_configs[%d] 必须配置 channel_id 和 template_id", i)
			}
			if err := addChannel(channelID); err != nil {
				return nil, nil, err
			}
			if err := addTemplate(templateID); err != nil {
				return nil, nil, err
			}
		}
		return uuidKeys(templateIDs), uuidKeys(channelIDs), nil
	}

	if templateID, _ := config["template_id"].(string); templateID != "" {
		if err := addTemplate(templateID); err != nil {
			return nil, nil, err
		}
	}
	if channelID, _ := config["channel_id"].(string); channelID != "" {
		if err := addChannel(channelID); err != nil {
			return nil, nil, err
		}
	}
	if rawIDs, ok := config["channel_ids"].([]interface{}); ok {
		for i, raw := range rawIDs {
			channelID, ok := raw.(string)
			if !ok || channelID == "" {
				return nil, nil, fmt.Errorf("channel_ids[%d] 必须是非空字符串", i)
			}
			if err := addChannel(channelID); err != nil {
				return nil, nil, err
			}
		}
	}
	return uuidKeys(templateIDs), uuidKeys(channelIDs), nil
}

func uuidKeys(values map[uuid.UUID]struct{}) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	return ids
}

func (h *HealingHandler) validateClosePolicy(ctx context.Context, flow *model.HealingFlow) error {
	if flow.ClosePolicy == nil {
		return nil
	}
	enabled, _ := flow.ClosePolicy["enabled"].(bool)
	triggerOn, _ := flow.ClosePolicy["trigger_on"].(string)
	if enabled && triggerOn != "" && triggerOn != "flow_success" {
		return fmt.Errorf("close_policy.trigger_on 仅支持 flow_success")
	}
	rawTemplateID, _ := flow.ClosePolicy["solution_template_id"].(string)
	if !enabled && rawTemplateID == "" {
		return nil
	}
	if enabled && rawTemplateID == "" {
		return fmt.Errorf("close_policy 已启用时必须配置 solution_template_id")
	}
	if rawTemplateID == "" {
		return nil
	}
	templateID, err := uuid.Parse(rawTemplateID)
	if err != nil {
		return fmt.Errorf("close_policy.solution_template_id 不是合法 UUID")
	}
	if h.solutionRepo == nil {
		return nil
	}
	if _, err := h.solutionRepo.GetByID(ctx, templateID); err != nil {
		return fmt.Errorf("close_policy.solution_template_id 引用了不存在的解决方案模板: %s", rawTemplateID)
	}
	return nil
}

func (h *HealingHandler) validateHealingRule(ctx context.Context, rule *model.HealingRule) error {
	if err := validateTriggerMode(rule.TriggerMode); err != nil {
		return err
	}
	if err := validateMatchMode(rule.MatchMode); err != nil {
		return err
	}
	if rule.FlowID == nil {
		return nil
	}
	if h.flowRepo == nil {
		return nil
	}
	if _, err := h.flowRepo.GetByID(ctx, *rule.FlowID); err != nil {
		return fmt.Errorf("flow_id 引用了不存在的自愈流程: %s", rule.FlowID.String())
	}
	return nil
}
