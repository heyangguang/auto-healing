package healing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

// DryRunResult Dry-Run 执行结果
type DryRunResult struct {
	Success bool               `json:"success"`
	Message string             `json:"message,omitempty"`
	Nodes   []DryRunNodeResult `json:"nodes"`
}

// DryRunNodeResult 节点执行结果
type DryRunNodeResult struct {
	NodeID   string                 `json:"node_id"`
	NodeType string                 `json:"node_type"`
	NodeName string                 `json:"node_name,omitempty"`
	Status   string                 `json:"status"` // success, failed, error (与真实执行一致)
	Message  string                 `json:"message,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`   // 节点输入（上游数据+全局上下文）
	Process  []string               `json:"process,omitempty"` // 执行过程日志
	Output   map[string]interface{} `json:"output,omitempty"`  // 节点输出（传给下游）
	Config   map[string]interface{} `json:"config,omitempty"`
}

// DryRunExecutor Dry-Run 执行器
type DryRunExecutor struct {
	taskRepo *repository.ExecutionRepository
	cmdbRepo *repository.CMDBItemRepository
}

// NewDryRunExecutor 创建 Dry-Run 执行器
func NewDryRunExecutor() *DryRunExecutor {
	return &DryRunExecutor{
		taskRepo: repository.NewExecutionRepository(),
		cmdbRepo: repository.NewCMDBItemRepository(),
	}
}

// MockIncident 模拟工单数据
type MockIncident struct {
	Title           string                 `json:"title"`
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

// ToIncident 转换为 Incident 模型
func (m *MockIncident) ToIncident() *model.Incident {
	return &model.Incident{
		Title:           m.Title,
		Description:     m.Description,
		Severity:        m.Severity,
		Priority:        m.Priority,
		Status:          m.Status,
		Category:        m.Category,
		AffectedCI:      m.AffectedCI,
		AffectedService: m.AffectedService,
		Assignee:        m.Assignee,
		Reporter:        m.Reporter,
		RawData:         m.RawData,
	}
}

// NodeCallback 节点执行回调函数
// eventType: flow_start, node_start, node_log, node_complete, flow_complete
type NodeCallback func(eventType string, data map[string]interface{})

// Execute 执行 Dry-Run 测试
// fromNodeID: 从哪个节点开始（用于重试），为空则从 start 节点开始
// initialContext: 初始上下文（用于重试），为空则使用默认上下文
// mockApprovals: 模拟审批结果，node_id -> "approved" | "rejected"
func (e *DryRunExecutor) Execute(ctx context.Context, flow *model.HealingFlow, mockIncident *MockIncident, fromNodeID string, initialContext map[string]interface{}, mockApprovals map[string]string) *DryRunResult {
	return e.ExecuteWithCallback(ctx, flow, mockIncident, fromNodeID, initialContext, mockApprovals, nil)
}

// ExecuteWithCallback 执行 Dry-Run 测试（带回调，用于 SSE）
func (e *DryRunExecutor) ExecuteWithCallback(ctx context.Context, flow *model.HealingFlow, mockIncident *MockIncident, fromNodeID string, initialContext map[string]interface{}, mockApprovals map[string]string, callback NodeCallback) *DryRunResult {
	result := &DryRunResult{
		Success: true,
		Nodes:   []DryRunNodeResult{},
	}

	// 辅助函数：发送回调
	emit := func(eventType string, data map[string]interface{}) {
		if callback != nil {
			callback(eventType, data)
		}
	}

	// 解析节点和边
	nodes := e.parseNodes(flow.Nodes)
	edges := e.parseEdges(flow.Edges)

	if len(nodes) == 0 {
		result.Success = false
		result.Message = "流程没有定义节点"
		emit(model.SSEEventFlowComplete, map[string]interface{}{
			"success": false,
			"message": result.Message,
		})
		return result
	}

	// 初始化上下文
	// 将 incident 转换为 map，确保表达式可以用小写字段名访问（如 incident.title）
	incident := mockIncident.ToIncident()
	incidentMap := incidentToMap(incident)
	flowContext := map[string]interface{}{
		"incident": incidentMap,
	}

	// 存入模拟审批配置
	if mockApprovals != nil {
		flowContext["_mock_approvals"] = mockApprovals
	}

	// 如果有初始上下文（重试场景），合并到 flowContext
	if initialContext != nil {
		for k, v := range initialContext {
			flowContext[k] = v
		}
	}

	// 确定起始节点
	var startNode *model.FlowNode
	if fromNodeID != "" {
		// 从指定节点开始（重试）
		startNode = e.findNodeByID(nodes, fromNodeID)
		if startNode == nil {
			result.Success = false
			result.Message = fmt.Sprintf("指定的节点 %s 不存在", fromNodeID)
			emit(model.SSEEventFlowComplete, map[string]interface{}{
				"success": false,
				"message": result.Message,
			})
			return result
		}
	} else {
		// 从 start 节点开始
		startNode = e.findNodeByType(nodes, model.NodeTypeStart)
		if startNode == nil {
			result.Success = false
			result.Message = "流程缺少起始节点"
			emit(model.SSEEventFlowComplete, map[string]interface{}{
				"success": false,
				"message": result.Message,
			})
			return result
		}
	}

	// 发送 flow_start 事件
	emit(model.SSEEventFlowStart, map[string]interface{}{
		"flow_id":   flow.ID.String(),
		"flow_name": flow.Name,
	})

	// 从起始节点开始遍历
	currentNode := startNode
	visited := make(map[string]bool)

	for currentNode != nil {
		// 防止死循环
		if visited[currentNode.ID] {
			break
		}
		visited[currentNode.ID] = true

		// 发送 node_start 事件
		config := e.getNodeConfig(currentNode)
		nodeName := currentNode.Name
		if nodeName == "" {
			if label, ok := config["label"].(string); ok {
				nodeName = label
			}
		}

		emit(model.SSEEventNodeStart, map[string]interface{}{
			"node_id":   currentNode.ID,
			"node_type": currentNode.Type,
			"node_name": nodeName,
			"status":    model.NodeStatusRunning,
		})

		// 执行节点
		nodeResult := e.executeNode(ctx, currentNode, flowContext)
		result.Nodes = append(result.Nodes, nodeResult)

		// 发送 node_complete 事件 - 将 output_handle 提取到顶层
		outputHandle := ""
		if nodeResult.Output != nil {
			if oh, ok := nodeResult.Output["output_handle"].(string); ok {
				outputHandle = oh
			}
		}
		emit(model.SSEEventNodeComplete, map[string]interface{}{
			"node_id":       nodeResult.NodeID,
			"node_type":     nodeResult.NodeType,
			"node_name":     nodeResult.NodeName,
			"status":        nodeResult.Status,
			"message":       nodeResult.Message,
			"input":         nodeResult.Input,
			"process":       nodeResult.Process,
			"output":        nodeResult.Output,
			"output_handle": outputHandle,
		})

		// 如果节点执行失败，终止
		if nodeResult.Status == "error" {
			result.Success = false
			result.Message = fmt.Sprintf("节点 %s 执行失败: %s", currentNode.ID, nodeResult.Message)

			// 收集所有后续未执行的节点并标记为 skipped
			remainingNodes := make(map[string]bool)
			e.collectAllDownstreamNodes(nodes, edges, currentNode.ID, remainingNodes)
			for nodeID := range remainingNodes {
				if !visited[nodeID] {
					skipNode := e.findNodeByID(nodes, nodeID)
					if skipNode != nil {
						emit(model.SSEEventNodeComplete, map[string]interface{}{
							"node_id":   skipNode.ID,
							"node_type": skipNode.Type,
							"node_name": skipNode.Name,
							"status":    "skipped",
							"message":   fmt.Sprintf("上游节点 %s 执行失败，跳过执行", currentNode.ID),
							"input":     nil,
							"process":   []string{"上游节点失败"},
							"output":    nil,
						})
						result.Nodes = append(result.Nodes, DryRunNodeResult{
							NodeID:   skipNode.ID,
							NodeType: skipNode.Type,
							NodeName: skipNode.Name,
							Status:   "skipped",
							Message:  fmt.Sprintf("上游节点 %s 执行失败，跳过执行", currentNode.ID),
						})
						visited[nodeID] = true
					}
				}
			}
			break
		}

		// 结束节点，退出
		if currentNode.Type == model.NodeTypeEnd {
			break
		}

		// 条件节点特殊处理
		if currentNode.Type == model.NodeTypeCondition {
			// 条件节点根据结果选择分支，Dry-Run 默认走 true 分支
			chosenHandle := "true"
			// 发送未走分支的 skipped 事件
			skippedNodeIDs := e.getSkippedBranchNodes(nodes, edges, currentNode.ID, chosenHandle, visited)
			for _, skippedID := range skippedNodeIDs {
				skippedNode := e.findNodeByID(nodes, skippedID)
				if skippedNode != nil {
					emit(model.SSEEventNodeComplete, map[string]interface{}{
						"node_id":   skippedNode.ID,
						"node_type": skippedNode.Type,
						"node_name": skippedNode.Name,
						"status":    "skipped",
						"message":   "条件分支未选中，跳过执行",
						"input":     nil,
						"process":   []string{"分支未选中"},
						"output":    nil,
					})
					// 添加到结果
					result.Nodes = append(result.Nodes, DryRunNodeResult{
						NodeID:   skippedNode.ID,
						NodeType: skippedNode.Type,
						NodeName: skippedNode.Name,
						Status:   "skipped",
						Message:  "条件分支未选中，跳过执行",
					})
					visited[skippedID] = true
				}
			}
			if trueTarget, ok := config["true_target"].(string); ok {
				currentNode = e.findNodeByID(nodes, trueTarget)
				continue
			}
		}

		// 审批节点根据模拟结果选择分支
		if currentNode.Type == model.NodeTypeApproval {
			outputHandle := "approved" // 默认
			if handle, ok := nodeResult.Output["output_handle"].(string); ok {
				outputHandle = handle
			}
			// 发送未走分支的 skipped 事件
			skippedNodeIDs := e.getSkippedBranchNodes(nodes, edges, currentNode.ID, outputHandle, visited)
			for _, skippedID := range skippedNodeIDs {
				skippedNode := e.findNodeByID(nodes, skippedID)
				if skippedNode != nil {
					emit(model.SSEEventNodeComplete, map[string]interface{}{
						"node_id":   skippedNode.ID,
						"node_type": skippedNode.Type,
						"node_name": skippedNode.Name,
						"status":    "skipped",
						"message":   fmt.Sprintf("审批分支 %s 未选中，跳过执行", outputHandle),
						"input":     nil,
						"process":   []string{"分支未选中"},
						"output":    nil,
					})
					// 添加到结果
					result.Nodes = append(result.Nodes, DryRunNodeResult{
						NodeID:   skippedNode.ID,
						NodeType: skippedNode.Type,
						NodeName: skippedNode.Name,
						Status:   "skipped",
						Message:  fmt.Sprintf("审批分支 %s 未选中，跳过执行", outputHandle),
					})
					visited[skippedID] = true
				}
			}
			// 尝试根据 handle 找下一个节点
			nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, outputHandle)
			if nextNode != nil {
				currentNode = nextNode
				continue
			}
		}

		// 执行节点根据结果选择分支（success/partial/failed）
		if currentNode.Type == model.NodeTypeExecution {
			outputHandle := "success" // 默认
			if handle, ok := nodeResult.Output["output_handle"].(string); ok {
				outputHandle = handle
			}
			// 发送未走分支的 skipped 事件
			skippedNodeIDs := e.getSkippedBranchNodes(nodes, edges, currentNode.ID, outputHandle, visited)
			for _, skippedID := range skippedNodeIDs {
				skippedNode := e.findNodeByID(nodes, skippedID)
				if skippedNode != nil {
					emit(model.SSEEventNodeComplete, map[string]interface{}{
						"node_id":   skippedNode.ID,
						"node_type": skippedNode.Type,
						"node_name": skippedNode.Name,
						"status":    "skipped",
						"message":   fmt.Sprintf("执行分支 %s 未选中，跳过执行", outputHandle),
						"input":     nil,
						"process":   []string{"分支未选中"},
						"output":    nil,
					})
					result.Nodes = append(result.Nodes, DryRunNodeResult{
						NodeID:   skippedNode.ID,
						NodeType: skippedNode.Type,
						NodeName: skippedNode.Name,
						Status:   "skipped",
						Message:  fmt.Sprintf("执行分支 %s 未选中，跳过执行", outputHandle),
					})
					visited[skippedID] = true
				}
			}
			// 尝试根据 handle 找下一个节点
			nextNode := e.findNextNodeByHandle(nodes, edges, currentNode.ID, outputHandle)
			if nextNode != nil {
				currentNode = nextNode
				continue
			}
		}

		// 找下一个节点
		currentNode = e.findNextNode(nodes, edges, currentNode.ID)
	}

	if result.Success {
		result.Message = fmt.Sprintf("Dry-Run 完成，共执行 %d 个节点", len(result.Nodes))
	}

	// 发送 flow_complete 事件
	emit(model.SSEEventFlowComplete, map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
	})

	return result
}

// executeNode 执行单个节点
func (e *DryRunExecutor) executeNode(ctx context.Context, node *model.FlowNode, flowContext map[string]interface{}) DryRunNodeResult {
	result := DryRunNodeResult{
		NodeID:   node.ID,
		NodeType: node.Type,
		NodeName: node.Name,
		Status:   "success",
		Input:    make(map[string]interface{}),
		Process:  []string{},
		Output:   make(map[string]interface{}),
	}

	// 记录节点输入（当前上下文快照，排除内部字段）
	for k, v := range flowContext {
		if k != "_mock_approvals" { // 排除内部配置
			result.Input[k] = v
		}
	}

	config := e.getNodeConfig(node)
	result.Config = config

	switch node.Type {
	case model.NodeTypeStart:
		result.Process = append(result.Process, "初始化流程上下文")
		result.Message = "流程开始"
		// 输出初始上下文中的 incident
		if incident, ok := flowContext["incident"]; ok {
			result.Output["incident"] = incident
			result.Process = append(result.Process, "输出 incident 到下游")
		}

	case model.NodeTypeEnd:
		result.Process = append(result.Process, "流程执行完毕")
		result.Message = "流程结束"

	case model.NodeTypeHostExtractor:
		// 真实执行：从工单中提取主机
		sourceField, _ := config["source_field"].(string)
		result.Process = append(result.Process, fmt.Sprintf("读取配置 source_field: %s", sourceField))
		if sourceField == "" {
			result.Status = "error"
			result.Message = "主机提取失败: 未配置 source_field（数据源字段）"
			result.Process = append(result.Process, "错误: source_field 未配置")
			return result
		}
		hosts := e.extractHosts(flowContext, config)
		result.Process = append(result.Process, fmt.Sprintf("从字段 %s 提取主机", sourceField))
		if len(hosts) == 0 {
			result.Status = "error"
			result.Message = fmt.Sprintf("主机提取失败: 从字段「%s」提取的主机列表为空", sourceField)
			result.Process = append(result.Process, "错误: 提取的主机列表为空")
			return result
		}
		result.Process = append(result.Process, fmt.Sprintf("成功提取 %d 个主机: %v", len(hosts), hosts))
		result.Message = fmt.Sprintf("提取主机: %v", hosts)
		result.Output["hosts"] = hosts
		// 更新上下文
		outputKey := "hosts"
		if ok, hasOk := config["output_key"].(string); hasOk && ok != "" {
			outputKey = ok
		}
		flowContext[outputKey] = hosts
		result.Process = append(result.Process, fmt.Sprintf("写入上下文 %s", outputKey))

	case model.NodeTypeCMDBValidator:
		// 获取配置
		inputKey := "hosts"
		if ik, ok := config["input_key"].(string); ok && ik != "" {
			inputKey = ik
		}
		outputKey := "validated_hosts"
		if outK, hasOutK := config["output_key"].(string); hasOutK && outK != "" {
			outputKey = outK
		}
		result.Process = append(result.Process, fmt.Sprintf("读取配置 input_key: %s, output_key: %s", inputKey, outputKey))

		// 获取主机列表
		hosts := e.getHostsFromContext(flowContext, inputKey)
		result.Process = append(result.Process, fmt.Sprintf("从上下文 %s 获取 %d 个主机", inputKey, len(hosts)))
		if len(hosts) == 0 {
			result.Status = "error"
			result.Message = fmt.Sprintf("CMDB 验证失败: 输入主机列表为空（来源: %s）", inputKey)
			result.Process = append(result.Process, "错误: 输入主机列表为空")
			return result
		}

		// 真实执行 CMDB 验证
		result.Process = append(result.Process, "开始查询 CMDB 数据库")
		var validatedHosts []map[string]interface{}
		var invalidHosts []string

		for _, host := range hosts {
			cmdbItem, err := e.cmdbRepo.FindByNameOrIP(ctx, host)
			if err != nil {
				invalidHosts = append(invalidHosts, host)
				result.Process = append(result.Process, fmt.Sprintf("主机 %s: 未在 CMDB 找到", host))
				continue
			}

			// 检查状态
			if cmdbItem.Status == "maintenance" || cmdbItem.Status == "offline" {
				invalidHosts = append(invalidHosts, host)
				result.Process = append(result.Process, fmt.Sprintf("主机 %s: 状态为 %s，跳过", host, cmdbItem.Status))
				continue
			}

			result.Process = append(result.Process, fmt.Sprintf("主机 %s: 验证通过 (IP: %s, 状态: %s)", host, cmdbItem.IPAddress, cmdbItem.Status))
			// 验证通过
			validatedHosts = append(validatedHosts, map[string]interface{}{
				"original_name": host,
				"cmdb_name":     cmdbItem.Name,
				"ip":            cmdbItem.IPAddress,
				"status":        cmdbItem.Status,
			})
		}

		// 判断结果
		if len(validatedHosts) == 0 {
			result.Status = "error"
			result.Message = fmt.Sprintf("CMDB 验证失败: 所有主机验证失败 %v", invalidHosts)
			result.Process = append(result.Process, "错误: 所有主机验证失败")
			return result
		}

		result.Status = "success"
		result.Message = fmt.Sprintf("CMDB 验证通过: %d/%d 台主机", len(validatedHosts), len(hosts))
		result.Output["validated_hosts"] = validatedHosts
		result.Output["invalid_hosts"] = invalidHosts
		result.Process = append(result.Process, fmt.Sprintf("验证完成: %d 通过, %d 失败", len(validatedHosts), len(invalidHosts)))

		// 更新上下文
		flowContext[outputKey] = validatedHosts
		result.Process = append(result.Process, fmt.Sprintf("写入上下文 %s", outputKey))

	case model.NodeTypeApproval:
		// 审批节点：验证必要配置
		title, hasTitle := config["title"].(string)
		result.Process = append(result.Process, fmt.Sprintf("读取配置 title: %s", title))
		if !hasTitle || title == "" {
			result.Status = "error"
			result.Message = "审批节点配置错误: 未设置审批标题"
			result.Process = append(result.Process, "错误: 未设置审批标题")
			return result
		}

		// 读取模拟审批配置
		mockResult := "approved" // 默认通过
		if mockApprovals, ok := flowContext["_mock_approvals"].(map[string]string); ok {
			if specified, exists := mockApprovals[node.ID]; exists {
				mockResult = specified
				result.Process = append(result.Process, fmt.Sprintf("使用 mock_approvals 配置: %s", mockResult))
			} else {
				result.Process = append(result.Process, "未指定模拟结果，默认 approved")
			}
		} else {
			result.Process = append(result.Process, "未指定模拟结果，默认 approved")
		}

		// 根据模拟结果设置状态和输出分支
		if mockResult == "rejected" {
			result.Status = "success"
			result.Message = fmt.Sprintf("审批节点「%s」(模拟拒绝)", title)
			result.Output["approval_result"] = "rejected"
			result.Output["output_handle"] = "rejected"
			result.Process = append(result.Process, "模拟结果: 拒绝，走 rejected 分支")
		} else {
			result.Status = "success"
			result.Message = fmt.Sprintf("审批节点「%s」(模拟通过)", title)
			result.Output["approval_result"] = "approved"
			result.Output["output_handle"] = "approved"
			result.Process = append(result.Process, "模拟结果: 通过，走 approved 分支")
		}

	case model.NodeTypeExecution:
		// 验证必要配置
		taskTemplateID, hasTaskID := config["task_template_id"].(string)
		result.Process = append(result.Process, fmt.Sprintf("读取配置 task_template_id: %s", taskTemplateID))
		if !hasTaskID || taskTemplateID == "" {
			result.Status = "error"
			result.Message = "执行节点配置错误: 未配置 task_template_id（任务模板）"
			result.Process = append(result.Process, "错误: 未配置任务模板ID")
			return result
		}
		// 验证任务模板存在
		taskTemplateName := "未知任务"
		taskUUID, err := uuid.Parse(taskTemplateID)
		if err != nil {
			result.Status = "error"
			result.Message = fmt.Sprintf("执行节点配置错误: task_template_id 格式无效「%s」", taskTemplateID)
			result.Process = append(result.Process, "错误: 任务模板ID格式无效")
			return result
		}
		task, err := e.taskRepo.GetTaskByID(ctx, taskUUID)
		if err != nil || task == nil {
			result.Status = "error"
			result.Message = fmt.Sprintf("执行节点配置错误: 任务模板不存在「%s」", taskTemplateID)
			result.Process = append(result.Process, "错误: 任务模板不存在")
			return result
		}
		taskTemplateName = task.Name
		result.Process = append(result.Process, fmt.Sprintf("任务模板验证通过: %s", taskTemplateName))

		// 读取任务模板详细信息
		result.Process = append(result.Process, "--- 任务模板配置 ---")
		result.Process = append(result.Process, fmt.Sprintf("模板名称: %s", task.Name))
		if task.Description != "" {
			result.Process = append(result.Process, fmt.Sprintf("模板描述: %s", task.Description))
		}
		if task.PlaybookID != uuid.Nil {
			result.Process = append(result.Process, fmt.Sprintf("Playbook ID: %s", task.PlaybookID.String()))
		}

		// 读取节点级配置的变量
		result.Process = append(result.Process, "--- 变量配置 ---")
		nodeExtraVars := make(map[string]interface{})
		if extraVars, ok := config["extra_vars"].(map[string]interface{}); ok {
			nodeExtraVars = extraVars
		}
		variableMappings := make(map[string]string)
		if mappings, ok := config["variable_mappings"].(map[string]interface{}); ok {
			for k, v := range mappings {
				if vs, ok := v.(string); ok {
					variableMappings[k] = vs
				}
			}
		}

		// 合并变量：模板默认值 + 节点配置的静态值 + 表达式映射
		finalVars := make(map[string]interface{})
		// 1. 模板默认值
		if task.ExtraVars != nil {
			for k, v := range task.ExtraVars {
				finalVars[k] = v
			}
		}
		// 2. 节点配置的静态值
		for k, v := range nodeExtraVars {
			finalVars[k] = v
		}
		// 3. 表达式映射 - 从上下文计算
		evaluator := NewExpressionEvaluator()
		for varName, expression := range variableMappings {
			result.Process = append(result.Process, fmt.Sprintf("计算表达式 %s = %s", varName, expression))
			exprResult, err := evaluator.Evaluate(expression, flowContext)
			if err != nil {
				result.Process = append(result.Process, fmt.Sprintf("  → 计算失败: %v", err))
			} else {
				finalVars[varName] = exprResult
				result.Process = append(result.Process, fmt.Sprintf("  → 结果: %v", exprResult))
			}
		}

		// 打印最终变量
		if len(finalVars) > 0 {
			result.Process = append(result.Process, "最终变量值:")
			for k, v := range finalVars {
				result.Process = append(result.Process, fmt.Sprintf("  %s = %v", k, v))
			}
		} else {
			result.Process = append(result.Process, "无额外变量")
		}

		// 验证目标主机
		result.Process = append(result.Process, "--- 目标主机 ---")
		hostsKey := "validated_hosts"
		if hk, ok := config["hosts_key"].(string); ok && hk != "" {
			hostsKey = hk
		}
		hosts := e.getHostsFromContext(flowContext, hostsKey)
		result.Process = append(result.Process, fmt.Sprintf("主机来源: 上下文变量 %s", hostsKey))
		result.Process = append(result.Process, fmt.Sprintf("获取到 %d 个目标主机", len(hosts)))
		if len(hosts) > 0 {
			result.Process = append(result.Process, fmt.Sprintf("主机列表: %v", hosts))
		}
		if len(hosts) == 0 {
			result.Status = "error"
			result.Message = fmt.Sprintf("执行节点失败: 目标主机列表为空（来源: %s）", hostsKey)
			result.Process = append(result.Process, "错误: 目标主机列表为空")
			return result
		}

		// 模拟执行
		result.Status = "success"
		result.Message = fmt.Sprintf("将执行任务「%s」，目标主机: %v", taskTemplateName, hosts)
		result.Output["task_template_id"] = taskTemplateID
		result.Output["task_template"] = taskTemplateName
		result.Output["target_hosts"] = hosts
		result.Output["final_vars"] = finalVars
		result.Output["output_handle"] = "success" // 分支信息，用于前端高亮正确的出边
		result.Process = append(result.Process, "--- 执行结果 ---")
		result.Process = append(result.Process, fmt.Sprintf("模拟执行完成，将在 %d 台主机上执行任务「%s」", len(hosts), taskTemplateName))

	case model.NodeTypeNotification:
		// 验证必要配置
		templateID, hasTemplateID := config["template_id"].(string)
		result.Process = append(result.Process, fmt.Sprintf("读取配置 template_id: %s", templateID))
		if !hasTemplateID || templateID == "" {
			result.Status = "error"
			result.Message = "通知节点配置错误: 未配置 template_id（通知模板）"
			result.Process = append(result.Process, "错误: 未配置通知模板ID")
			return result
		}
		// 验证模板存在
		var tpl model.NotificationTemplate
		if err := database.DB.Where("id = ?", templateID).First(&tpl).Error; err != nil {
			result.Status = "error"
			result.Message = fmt.Sprintf("通知节点配置错误: 通知模板不存在「%s」", templateID)
			result.Process = append(result.Process, "错误: 通知模板不存在")
			return result
		}
		result.Process = append(result.Process, fmt.Sprintf("通知模板验证通过: %s", tpl.Name))
		// 验证渠道配置
		channels, hasChannels := config["channel_ids"].([]interface{})
		if !hasChannels || len(channels) == 0 {
			result.Status = "error"
			result.Message = "通知节点配置错误: 未配置 channel_ids（通知渠道）"
			result.Process = append(result.Process, "错误: 未配置通知渠道")
			return result
		}
		result.Process = append(result.Process, fmt.Sprintf("通知渠道验证通过: %d 个渠道", len(channels)))
		// 模拟发送
		result.Status = "success"
		result.Message = fmt.Sprintf("将发送通知「%s」到 %d 个渠道", tpl.Name, len(channels))
		result.Process = append(result.Process, "模拟发送完成")

	case model.NodeTypeCondition:
		// 真实执行条件判断
		result.Message = "条件判断(Dry-Run 默认走 true 分支)"
		if cond, ok := config["condition"].(string); ok {
			result.Output["condition"] = cond
		}
		result.Output["result"] = true
		result.Output["output_handle"] = "true" // 分支信息，用于前端高亮正确的出边

	case model.NodeTypeSetVariable:
		// 真实执行设置变量
		key, _ := config["key"].(string)
		value := config["value"]
		result.Process = append(result.Process, fmt.Sprintf("读取配置 key: %s", key))
		result.Process = append(result.Process, fmt.Sprintf("读取配置 value: %v", value))
		result.Message = fmt.Sprintf("设置变量 %s = %v", key, value)
		if key != "" {
			flowContext[key] = value
			result.Process = append(result.Process, fmt.Sprintf("写入上下文 %s", key))
		}
		result.Output["key"] = key
		result.Output["value"] = value

	case model.NodeTypeCompute:
		// 真实执行计算节点
		operations, ok := config["operations"].([]interface{})
		if !ok || len(operations) == 0 {
			result.Process = append(result.Process, "计算节点配置为空")
			result.Message = "计算节点无操作"
			return result
		}

		result.Process = append(result.Process, fmt.Sprintf("读取 %d 个计算操作", len(operations)))

		// 创建表达式求值器
		evaluator := NewExpressionEvaluator()
		computedVars := make(map[string]interface{})

		for i, opRaw := range operations {
			op, ok := opRaw.(map[string]interface{})
			if !ok {
				result.Process = append(result.Process, fmt.Sprintf("操作 %d: 格式无效，跳过", i+1))
				continue
			}

			outputKey, _ := op["output_key"].(string)
			expression, _ := op["expression"].(string)

			if outputKey == "" || expression == "" {
				result.Process = append(result.Process, fmt.Sprintf("操作 %d: output_key 或 expression 为空，跳过", i+1))
				continue
			}

			result.Process = append(result.Process, fmt.Sprintf("计算 %s = %s", outputKey, expression))

			// 执行表达式
			computeResult, err := evaluator.Evaluate(expression, flowContext)
			if err != nil {
				result.Process = append(result.Process, fmt.Sprintf("  → 错误: %v", err))
				continue
			}

			// 写入上下文
			flowContext[outputKey] = computeResult
			computedVars[outputKey] = computeResult
			result.Process = append(result.Process, fmt.Sprintf("  → %s = %v", outputKey, computeResult))
		}

		result.Status = "success"
		result.Message = fmt.Sprintf("计算完成: %d 个变量", len(computedVars))
		result.Output = computedVars

	default:
		result.Status = "error"
		result.Message = fmt.Sprintf("未知节点类型: %s", node.Type)
	}

	return result
}

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
		if edge.Source == currentID {
			return e.findNodeByID(nodes, edge.Target)
		}
	}
	return nil
}

// findNextNodeByHandle 根据 sourceHandle 找下一个节点
func (e *DryRunExecutor) findNextNodeByHandle(nodes []model.FlowNode, edges []model.FlowEdge, currentID string, handle string) *model.FlowNode {
	// 优先精确匹配
	for _, edge := range edges {
		if edge.Source == currentID && edge.SourceHandle == handle {
			return e.findNodeByID(nodes, edge.Target)
		}
	}
	// 回退到无 handle 的边
	for _, edge := range edges {
		if edge.Source == currentID && edge.SourceHandle == "" {
			return e.findNodeByID(nodes, edge.Target)
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
		if edge.Source == sourceID && edge.SourceHandle == chosenHandle {
			// 从选中分支开始，收集所有下游节点
			e.collectAllDownstreamNodes(nodes, edges, edge.Target, executionPathNodes)
		}
	}

	// 找出所有从 sourceID 出发但不是 chosenHandle 的边
	for _, edge := range edges {
		if edge.Source == sourceID && edge.SourceHandle != chosenHandle && edge.SourceHandle != "" {
			// 递归收集这个分支的所有下游节点（排除执行路径节点）
			e.collectSkippedNodes(nodes, edges, edge.Target, executedNodeIDs, executionPathNodes, &skippedNodeIDs)
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
		if edge.Source == startNodeID {
			e.collectAllDownstreamNodes(nodes, edges, edge.Target, result)
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
		if edge.Source == startNodeID {
			e.collectSkippedNodes(nodes, edges, edge.Target, executedNodeIDs, executionPathNodes, result)
		}
	}
}

// collectDownstreamNodes 递归收集从某节点开始的所有下游节点（保留兼容性）
func (e *DryRunExecutor) collectDownstreamNodes(nodes []model.FlowNode, edges []model.FlowEdge, startNodeID string, executedNodeIDs map[string]bool, result *[]string) {
	// 如果节点已执行或已收集，跳过
	if executedNodeIDs[startNodeID] {
		return
	}
	for _, id := range *result {
		if id == startNodeID {
			return
		}
	}

	// 添加到结果
	*result = append(*result, startNodeID)

	// 递归收集下游节点
	for _, edge := range edges {
		if edge.Source == startNodeID {
			e.collectDownstreamNodes(nodes, edges, edge.Target, executedNodeIDs, result)
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
	value := e.getNestedValue(flowContext, sourceField)
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

func (e *DryRunExecutor) getNestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data
	for i, part := range parts {
		if current == nil {
			return nil
		}
		val, ok := current[part]
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			return val
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			// 尝试从 incident 结构体中获取
			if incident, ok := val.(*model.Incident); ok && len(parts) > i+1 {
				return e.getIncidentField(incident, parts[i+1])
			}
			return nil
		}
	}
	return nil
}

func (e *DryRunExecutor) getIncidentField(incident *model.Incident, field string) interface{} {
	switch field {
	case "affected_ci":
		return incident.AffectedCI
	case "affected_service":
		return incident.AffectedService
	case "title":
		return incident.Title
	case "description":
		return incident.Description
	case "severity":
		return incident.Severity
	case "priority":
		return incident.Priority
	case "status":
		return incident.Status
	case "category":
		return incident.Category
	default:
		if incident.RawData != nil {
			return incident.RawData[field]
		}
		return nil
	}
}

func (e *DryRunExecutor) getHostsFromContext(flowContext map[string]interface{}, key string) []string {
	val, ok := flowContext[key]
	if !ok {
		return []string{}
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
