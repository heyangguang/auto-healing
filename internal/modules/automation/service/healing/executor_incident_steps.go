package healing

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

func (e *FlowExecutor) buildAutoCloseTemplateVars(ctx context.Context, instance *model.FlowInstance, flow *model.HealingFlow) map[string]any {
	return map[string]any{
		"flow": map[string]any{
			"id":          flow.ID.String(),
			"name":        flow.Name,
			"instance_id": instance.ID.String(),
		},
		"execution":  buildAutoCloseExecutionContext(instance),
		"steps":      e.buildAutoCloseSteps(ctx, instance, flow),
		"steps_text": e.buildAutoCloseStepsText(ctx, instance, flow),
	}
}

func (e *FlowExecutor) buildAutoCloseStepsText(ctx context.Context, instance *model.FlowInstance, flow *model.HealingFlow) string {
	steps := e.buildAutoCloseSteps(ctx, instance, flow)
	lines := make([]string, 0, len(steps))
	for index, step := range steps {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, strings.TrimSpace(stringifyAutoCloseValue(step["summary"]))))
	}
	return strings.Join(lines, "\n")
}

func (e *FlowExecutor) buildAutoCloseSteps(ctx context.Context, instance *model.FlowInstance, flow *model.HealingFlow) []map[string]any {
	if instance == nil || flow == nil {
		return nil
	}
	orderedNodeIDs := e.executedNodeOrder(ctx, instance)
	if len(orderedNodeIDs) == 0 {
		return nil
	}

	nodeByID := make(map[string]model.FlowNode, len(flow.Nodes))
	for _, rawNode := range flow.Nodes {
		node, ok := parseFlowNode(rawNode)
		if ok {
			nodeByID[node.ID] = node
		}
	}

	steps := make([]map[string]any, 0, len(orderedNodeIDs))
	for _, nodeID := range orderedNodeIDs {
		node, exists := nodeByID[nodeID]
		if !exists || !autoCloseStepIncluded(node.Type) {
			continue
		}
		step := autoCloseStepFromNodeState(node, instance.NodeStates[nodeID])
		if step != nil {
			steps = append(steps, step)
		}
	}
	return steps
}

func (e *FlowExecutor) executedNodeOrder(ctx context.Context, instance *model.FlowInstance) []string {
	logs, err := e.flowLogRepo.GetByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil
	}
	order := make([]string, 0, len(logs))
	seen := make(map[string]bool, len(logs))
	for _, logEntry := range logs {
		if logEntry == nil || logEntry.NodeID == "" || seen[logEntry.NodeID] {
			continue
		}
		seen[logEntry.NodeID] = true
		order = append(order, logEntry.NodeID)
	}
	return order
}

func parseFlowNode(raw interface{}) (model.FlowNode, bool) {
	switch typed := raw.(type) {
	case model.FlowNode:
		return typed, true
	case map[string]any:
		node := model.FlowNode{}
		if value, ok := typed["id"].(string); ok {
			node.ID = value
		}
		if value, ok := typed["type"].(string); ok {
			node.Type = value
		}
		if value, ok := typed["name"].(string); ok {
			node.Name = value
		}
		if value, ok := typed["config"].(map[string]any); ok {
			node.Config = value
		}
		return node, node.ID != ""
	default:
		return model.FlowNode{}, false
	}
}

func autoCloseStepIncluded(nodeType string) bool {
	switch nodeType {
	case model.NodeTypeHostExtractor, model.NodeTypeCMDBValidator, model.NodeTypeExecution, model.NodeTypeApproval:
		return true
	default:
		return false
	}
}

func autoCloseStepFromNodeState(node model.FlowNode, rawState interface{}) map[string]any {
	state, _ := rawState.(map[string]any)
	if state == nil {
		return nil
	}
	status := strings.TrimSpace(stringValue(state["status"]))
	if status == "" {
		status = strings.TrimSpace(stringValue(state["output_handle"]))
	}
	summary, detail := autoCloseStepSummary(node, state)
	return map[string]any{
		"title":   strings.TrimSpace(node.Name),
		"type":    node.Type,
		"status":  status,
		"summary": summary,
		"detail":  detail,
	}
}

func autoCloseStepSummary(node model.FlowNode, state map[string]any) (string, string) {
	switch node.Type {
	case model.NodeTypeHostExtractor:
		hosts := autoCloseHostList(state["extracted_hosts"])
		hostCount := stringifyAutoCloseValue(state["host_count"])
		return fmt.Sprintf("识别 %s 台主机：%s", hostCount, hosts), ""
	case model.NodeTypeCMDBValidator:
		return autoCloseValidationSummary(state["validation_summary"]), ""
	case model.NodeTypeExecution:
		message := strings.TrimSpace(stringValue(state["message"]))
		runID := strings.TrimSpace(stringValue(state["run_id"]))
		if runID == "" {
			if run, ok := state["run"].(map[string]any); ok {
				runID = strings.TrimSpace(stringValue(run["run_id"]))
			}
		}
		detail := strings.TrimSpace(stringValue(state["stdout"]))
		if detail == "" {
			if run, ok := state["run"].(map[string]any); ok {
				detail = strings.TrimSpace(stringValue(run["stdout"]))
			}
		}
		detail = autoCloseExecutionTaskDetail(detail)
		return fmt.Sprintf("%s（run=%s）", message, runID), detail
	case model.NodeTypeApproval:
		title := strings.TrimSpace(stringValue(state["title"]))
		return fmt.Sprintf("审批节点：%s", title), strings.TrimSpace(stringValue(state["description"]))
	default:
		return strings.TrimSpace(node.Name), ""
	}
}

func stringifyAutoCloseValue(value interface{}) string {
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func autoCloseHostList(value interface{}) string {
	switch typed := value.(type) {
	case []string:
		return strings.Join(typed, ", ")
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, stringifyAutoCloseValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		return stringifyAutoCloseValue(value)
	}
}

func autoCloseValidationSummary(value interface{}) string {
	summary, ok := value.(map[string]interface{})
	if !ok {
		return stringifyAutoCloseValue(value)
	}
	return fmt.Sprintf(
		"共 %s 台主机，验证通过 %s 台，失败 %s 台",
		autoCloseMapInt(summary["total"]),
		autoCloseMapInt(summary["valid"]),
		autoCloseMapInt(summary["invalid"]),
	)
}

func autoCloseMapInt(value interface{}) string {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.Itoa(int(typed))
	default:
		return stringifyAutoCloseValue(value)
	}
}
