package healing

import (
	"encoding/json"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// parseFlowSnapshot 从实例快照解析节点和边
func (e *FlowExecutor) parseFlowSnapshot(instance *model.FlowInstance) ([]model.FlowNode, []model.FlowEdge, error) {
	var nodes []model.FlowNode
	var edges []model.FlowEdge

	nodesData, _ := json.Marshal(instance.FlowNodes)
	if err := json.Unmarshal(nodesData, &nodes); err != nil {
		return nil, nil, err
	}
	edgesData, _ := json.Marshal(instance.FlowEdges)
	if err := json.Unmarshal(edgesData, &edges); err != nil {
		return nil, nil, err
	}
	return nodes, edges, nil
}

// findStartNode 找到起始节点
func (e *FlowExecutor) findStartNode(nodes []model.FlowNode) *model.FlowNode {
	for i := range nodes {
		if nodes[i].Type == model.NodeTypeStart {
			return &nodes[i]
		}
	}
	return nil
}

// findNextNode 找到下一个节点
func (e *FlowExecutor) findNextNode(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string) *model.FlowNode {
	for _, edge := range edges {
		if edge.GetFrom() == currentNodeID {
			for i := range nodes {
				if nodes[i].ID == edge.GetTo() {
					return &nodes[i]
				}
			}
		}
	}
	return nil
}

// findNextNodeByHandle 根据输出口ID找到下一个节点
func (e *FlowExecutor) findNextNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string, handle string) *model.FlowNode {
	if nextNode := matchNodeByHandle(nodes, edges, currentNodeID, handle); nextNode != nil {
		return nextNode
	}
	if nextNode := matchDefaultNodeByHandle(nodes, edges, currentNodeID, handle); nextNode != nil {
		return nextNode
	}
	return matchUnnamedNodeByHandle(nodes, edges, currentNodeID, handle)
}

func matchNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string, handle string) *model.FlowNode {
	for _, edge := range edges {
		if edge.GetFrom() == currentNodeID && edge.GetSourceHandle() == handle {
			for i := range nodes {
				if nodes[i].ID == edge.GetTo() {
					logger.Exec("FLOW").Debug("找到分支 %s -> %s (handle=%s)", currentNodeID, nodes[i].ID, handle)
					return &nodes[i]
				}
			}
		}
	}
	return nil
}

func matchDefaultNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string, handle string) *model.FlowNode {
	if handle == "default" || handle == "rejected" || handle == "failed" {
		return nil
	}
	for _, edge := range edges {
		if edge.GetFrom() == currentNodeID && edge.GetSourceHandle() == "default" {
			for i := range nodes {
				if nodes[i].ID == edge.GetTo() {
					logger.Exec("FLOW").Debug("回退到 default 分支 %s -> %s", currentNodeID, nodes[i].ID)
					return &nodes[i]
				}
			}
		}
	}
	return nil
}

func matchUnnamedNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentNodeID string, handle string) *model.FlowNode {
	if handle != "" && handle != "default" && handle != "success" {
		return nil
	}
	for _, edge := range edges {
		if edge.GetFrom() == currentNodeID && edge.SourceHandle == "" {
			for i := range nodes {
				if nodes[i].ID == edge.GetTo() {
					logger.Exec("FLOW").Debug("使用无 handle 的边 %s -> %s", currentNodeID, nodes[i].ID)
					return &nodes[i]
				}
			}
		}
	}
	return nil
}
