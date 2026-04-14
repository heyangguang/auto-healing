package healing

import (
	"encoding/json"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
)

// 辅助方法

func (e *DryRunExecutor) parseNodes(nodesData model.JSONArray) []model.FlowNode {
	var nodes []model.FlowNode
	data, _ := json.Marshal(nodesData)
	json.Unmarshal(data, &nodes)
	return nodes
}

func (e *DryRunExecutor) parseEdges(edgesData model.JSONArray) []model.FlowEdge {
	var edges []model.FlowEdge
	data, _ := json.Marshal(edgesData)
	json.Unmarshal(data, &edges)
	return edges
}

func (e *DryRunExecutor) findNodeByType(nodes []model.FlowNode, nodeType string) *model.FlowNode {
	for i := range nodes {
		if nodes[i].Type == nodeType {
			return &nodes[i]
		}
	}
	return nil
}

func (e *DryRunExecutor) findNodeByID(nodes []model.FlowNode, id string) *model.FlowNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func (e *DryRunExecutor) findNextNode(nodes []model.FlowNode, edges []model.FlowEdge, currentID string) *model.FlowNode {
	for _, edge := range edges {
		if edge.GetFrom() == currentID {
			return e.findNodeByID(nodes, edge.GetTo())
		}
	}
	return nil
}

// findNextNodeByHandle 根据 sourceHandle 找下一个节点
func (e *DryRunExecutor) findNextNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentID string, handle string) *model.FlowNode {
	// 优先精确匹配
	for _, edge := range edges {
		if edge.GetFrom() == currentID && edge.GetSourceHandle() == handle {
			return e.findNodeByID(nodes, edge.GetTo())
		}
	}
	// 回退到无 handle 的边
	for _, edge := range edges {
		if edge.GetFrom() == currentID && edge.SourceHandle == "" {
			return e.findNodeByID(nodes, edge.GetTo())
		}
	}
	return nil
}

// getSkippedBranchNodes 获取从源节点出发但排除已选分支的所有其他分支节点
// sourceID: 分支起点节点ID
// chosenHandle: 已选择的分支handle（如 "approved", "rejected", "true", "false"）
// 返回所有未走分支的节点ID列表（排除执行路径会经过的节点）
func (e *DryRunExecutor) getSkippedBranchNodes(nodes []model.FlowNode, edges []model.FlowEdge, sourceID string, chosenHandle string, executedNodeIDs map[string]bool) []string {
	var skippedNodeIDs []string

	// 首先，收集选中分支会经过的所有节点（执行路径）
	executionPathNodes := make(map[string]bool)
	for _, edge := range edges {
		if edge.GetFrom() == sourceID && edge.GetSourceHandle() == chosenHandle {
			// 从选中分支开始，收集所有下游节点
			e.collectAllDownstreamNodes(nodes, edges, edge.GetTo(), executionPathNodes)
		}
	}

	// 找出所有从 sourceID 出发但不是 chosenHandle 的边
	for _, edge := range edges {
		if edge.GetFrom() == sourceID && edge.GetSourceHandle() != chosenHandle && edge.SourceHandle != "" {
			// 递归收集这个分支的所有下游节点（排除执行路径节点）
			e.collectSkippedNodes(nodes, edges, edge.GetTo(), executedNodeIDs, executionPathNodes, &skippedNodeIDs)
		}
	}

	return skippedNodeIDs
}

// collectAllDownstreamNodes 递归收集从某节点开始的所有下游节点（不排除任何节点）
func (e *DryRunExecutor) collectAllDownstreamNodes(nodes []model.FlowNode, edges []model.FlowEdge, startNodeID string, result map[string]bool) {
	if result[startNodeID] {
		return
	}
	result[startNodeID] = true

	for _, edge := range edges {
		if edge.GetFrom() == startNodeID {
			e.collectAllDownstreamNodes(nodes, edges, edge.GetTo(), result)
		}
	}
}

// collectSkippedNodes 递归收集 skipped 节点（排除已执行和执行路径节点）
func (e *DryRunExecutor) collectSkippedNodes(nodes []model.FlowNode, edges []model.FlowEdge, startNodeID string, executedNodeIDs map[string]bool, executionPathNodes map[string]bool, result *[]string) {
	// 如果节点已执行，跳过
	if executedNodeIDs[startNodeID] {
		return
	}
	// 如果节点在执行路径上，跳过（不标记为 skipped）
	if executionPathNodes[startNodeID] {
		return
	}
	// 如果已收集过，跳过
	for _, id := range *result {
		if id == startNodeID {
			return
		}
	}

	// 添加到结果
	*result = append(*result, startNodeID)

	// 递归收集下游节点
	for _, edge := range edges {
		if edge.GetFrom() == startNodeID {
			e.collectSkippedNodes(nodes, edges, edge.GetTo(), executedNodeIDs, executionPathNodes, result)
		}
	}
}

func (e *DryRunExecutor) getNodeConfig(node *model.FlowNode) map[string]interface{} {
	if node.Config == nil {
		return map[string]interface{}{}
	}
	return node.Config
}

func (e *DryRunExecutor) extractHosts(flowContext map[string]interface{}, config map[string]interface{}) []string {
	sourceField, _ := config["source_field"].(string)
	splitBy, _ := config["split_by"].(string)

	if splitBy == "" {
		splitBy = ","
	}

	// 从上下文中获取值
	value := resolveFlowContextSourceValue(flowContext, sourceField)
	if value == nil {
		return []string{}
	}

	valueStr, ok := value.(string)
	if !ok {
		return []string{}
	}

	// 分割
	parts := strings.Split(valueStr, splitBy)
	hosts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			hosts = append(hosts, p)
		}
	}
	return hosts
}

func (e *DryRunExecutor) getHostsFromContext(flowContext map[string]interface{}, key string) []string {
	val, ok := flowContext[key]
	if !ok {
		return []string{}
	}
	if hosts := collectExecutionHosts(val); len(hosts) > 0 {
		return hosts
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		hosts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				hosts = append(hosts, s)
			}
		}
		return hosts
	case string:
		return []string{v}
	default:
		return []string{}
	}
}
