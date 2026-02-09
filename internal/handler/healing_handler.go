package handler

import (
	"context"
	"log"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	healing "github.com/company/auto-healing/internal/service/healing"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HealingHandler 自愈引擎处理器
type HealingHandler struct {
	flowRepo     *repository.HealingFlowRepository
	ruleRepo     *repository.HealingRuleRepository
	instanceRepo *repository.FlowInstanceRepository
	approvalRepo *repository.ApprovalTaskRepository
	incidentRepo *repository.IncidentRepository
	executor     *healing.FlowExecutor
	scheduler    *healing.Scheduler
}

// NewHealingHandler 创建自愈引擎处理器
func NewHealingHandler() *HealingHandler {
	return &HealingHandler{
		flowRepo:     repository.NewHealingFlowRepository(),
		ruleRepo:     repository.NewHealingRuleRepository(),
		instanceRepo: repository.NewFlowInstanceRepository(),
		approvalRepo: repository.NewApprovalTaskRepository(),
		incidentRepo: repository.NewIncidentRepository(),
		executor:     healing.NewFlowExecutor(),
		scheduler:    healing.NewScheduler(),
	}
}

// ========== HealingFlow 相关 ==========

// GetNodeSchema 获取节点类型的配置和变量定义
// 用于前端流程设计器，帮助用户了解每种节点的配置项和输入输出
func (h *HealingHandler) GetNodeSchema(c *gin.Context) {
	schema := map[string]interface{}{
		"initial_context": map[string]interface{}{
			"incident": map[string]interface{}{
				"type":        "object",
				"description": "触发流程的工单数据",
				"properties": map[string]interface{}{
					"id":               map[string]string{"type": "string", "description": "工单ID"},
					"title":            map[string]string{"type": "string", "description": "工单标题"},
					"description":      map[string]string{"type": "string", "description": "工单描述"},
					"severity":         map[string]string{"type": "string", "description": "严重级别"},
					"priority":         map[string]string{"type": "string", "description": "优先级"},
					"status":           map[string]string{"type": "string", "description": "状态"},
					"category":         map[string]string{"type": "string", "description": "分类"},
					"affected_ci":      map[string]string{"type": "string", "description": "影响的CI（多个用逗号分隔）"},
					"affected_service": map[string]string{"type": "string", "description": "影响的服务"},
					"assignee":         map[string]string{"type": "string", "description": "处理人"},
					"reporter":         map[string]string{"type": "string", "description": "报告人"},
					"raw_data":         map[string]string{"type": "object", "description": "原始数据（来自第三方系统）"},
				},
			},
		},
		"nodes": map[string]interface{}{
			"start": map[string]interface{}{
				"name":        "开始",
				"description": "流程起始节点",
				"config":      map[string]interface{}{},
				"ports": map[string]interface{}{
					"in":  0,
					"out": 1,
					"out_ports": []map[string]string{
						{"id": "default", "name": "默认"},
					},
				},
				"inputs": []interface{}{},
				"outputs": []map[string]string{
					{"key": "incident", "type": "object", "description": "工单对象"},
				},
			},
			"end": map[string]interface{}{
				"name":        "结束",
				"description": "流程结束节点",
				"config":      map[string]interface{}{},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 0,
				},
				"inputs":  []interface{}{},
				"outputs": []interface{}{},
			},
			"host_extractor": map[string]interface{}{
				"name":        "主机提取器",
				"description": "从工单数据中提取主机列表",
				"config": map[string]interface{}{
					"source_field": map[string]string{"type": "string", "required": "true", "description": "数据来源字段，如 incident.affected_ci 或 incident.raw_data.cmdb_ci"},
					"extract_mode": map[string]string{"type": "string", "default": "split", "description": "提取模式：split(分割) 或 regex(正则)"},
					"split_by":     map[string]string{"type": "string", "default": ",", "description": "分割符（extract_mode=split时使用）"},
					"output_key":   map[string]string{"type": "string", "default": "hosts", "description": "输出变量名"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 1,
					"out_ports": []map[string]string{
						{"id": "default", "name": "默认"},
					},
				},
				"inputs": []map[string]string{
					{"key": "incident", "type": "object", "description": "工单对象"},
				},
				"outputs": []map[string]string{
					{"key": "hosts", "type": "array[string]", "description": "提取的主机列表"},
				},
			},
			"cmdb_validator": map[string]interface{}{
				"name":        "CMDB验证器",
				"description": "验证主机是否在CMDB中存在，并获取主机详细信息",
				"config": map[string]interface{}{
					"input_key":  map[string]string{"type": "string", "default": "hosts", "description": "输入变量名"},
					"output_key": map[string]string{"type": "string", "default": "validated_hosts", "description": "输出变量名"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 1,
					"out_ports": []map[string]string{
						{"id": "default", "name": "默认"},
					},
				},
				"inputs": []map[string]string{
					{"key": "hosts", "type": "array[string]", "description": "主机列表"},
				},
				"outputs": []map[string]string{
					{"key": "validated_hosts", "type": "array[object]", "description": "验证后的主机详情"},
					{"key": "validation_summary", "type": "object", "description": "验证统计 {total, valid, invalid}"},
				},
			},
			"approval": map[string]interface{}{
				"name":        "审批节点",
				"description": "等待人工审批，有两个输出分支",
				"config": map[string]interface{}{
					"title":          map[string]string{"type": "string", "required": "true", "description": "审批标题"},
					"description":    map[string]string{"type": "string", "description": "审批说明"},
					"approvers":      map[string]string{"type": "array[string]", "description": "审批人用户名列表"},
					"approver_roles": map[string]string{"type": "array[string]", "description": "审批人角色列表"},
					"timeout_hours":  map[string]string{"type": "number", "default": "24", "description": "超时时间(小时)"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 2,
					"out_ports": []map[string]string{
						{"id": "approved", "name": "通过", "condition": "审批通过时"},
						{"id": "rejected", "name": "拒绝", "condition": "审批拒绝或超时时"},
					},
				},
				"inputs":  []interface{}{},
				"outputs": []interface{}{},
			},
			"execution": map[string]interface{}{
				"name":        "执行节点",
				"description": "执行任务模板，根据执行结果走不同分支",
				"config": map[string]interface{}{
					"task_template_id": map[string]string{"type": "string", "required": "true", "description": "任务模板ID"},
					"hosts_key":        map[string]string{"type": "string", "default": "validated_hosts", "description": "主机列表变量名"},
					"extra_vars":       map[string]string{"type": "object", "description": "额外变量"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 3,
					"out_ports": []map[string]string{
						{"id": "success", "name": "成功", "condition": "所有主机执行成功"},
						{"id": "partial", "name": "部分成功", "condition": "部分主机成功，部分失败"},
						{"id": "failed", "name": "失败", "condition": "全部失败或取消/超时/错误"},
					},
				},
				"inputs": []map[string]string{
					{"key": "validated_hosts", "type": "array[object]", "description": "目标主机"},
				},
				"outputs": []map[string]string{
					{"key": "execution_result", "type": "object", "description": "执行结果，包含 status(success/partial/failed), stats 等"},
				},
			},
			"notification": map[string]interface{}{
				"name":        "通知节点",
				"description": "发送通知",
				"config": map[string]interface{}{
					"template_id": map[string]string{"type": "string", "required": "true", "description": "通知模板ID"},
					"channel_ids": map[string]string{"type": "array[string]", "required": "true", "description": "通知渠道ID列表"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 1,
					"out_ports": []map[string]string{
						{"id": "default", "name": "默认"},
					},
				},
				"inputs":  []interface{}{},
				"outputs": []interface{}{},
			},
			"condition": map[string]interface{}{
				"name":        "条件分支",
				"description": "根据条件选择执行路径，有两个输出分支",
				"config": map[string]interface{}{
					"condition":    map[string]string{"type": "string", "required": "true", "description": "条件表达式，如 execution_result.status == 'success'"},
					"true_target":  map[string]string{"type": "string", "description": "条件为真时跳转的节点ID（前端自动填充）"},
					"false_target": map[string]string{"type": "string", "description": "条件为假时跳转的节点ID（前端自动填充）"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 2,
					"out_ports": []map[string]string{
						{"id": "true", "name": "是", "condition": "条件为真"},
						{"id": "false", "name": "否", "condition": "条件为假"},
					},
				},
				"inputs":  []interface{}{},
				"outputs": []interface{}{},
			},
			"set_variable": map[string]interface{}{
				"name":        "设置变量",
				"description": "设置或修改上下文变量",
				"config": map[string]interface{}{
					"key":   map[string]string{"type": "string", "required": "true", "description": "变量名"},
					"value": map[string]string{"type": "any", "required": "true", "description": "变量值"},
				},
				"ports": map[string]interface{}{
					"in":  1,
					"out": 1,
					"out_ports": []map[string]string{
						{"id": "default", "name": "默认"},
					},
				},
				"inputs":  []interface{}{},
				"outputs": []interface{}{},
			},
		},
	}

	response.Success(c, schema)
}

// ListFlows 获取自愈流程列表

func (h *HealingHandler) ListFlows(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)

	var isActive *bool
	if str := c.Query("is_active"); str != "" {
		val := str == "true"
		isActive = &val
	}

	flows, total, err := h.flowRepo.List(c.Request.Context(), page, pageSize, isActive)
	if err != nil {
		response.InternalError(c, "获取自愈流程列表失败")
		return
	}

	response.List(c, flows, total, page, pageSize)
}

// CreateFlow 创建自愈流程
func (h *HealingHandler) CreateFlow(c *gin.Context) {
	var req CreateFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	flow := req.ToModel()
	if err := h.flowRepo.Create(c.Request.Context(), flow); err != nil {
		response.InternalError(c, "创建自愈流程失败")
		return
	}

	response.Created(c, flow)
}

// GetFlow 获取自愈流程详情
func (h *HealingHandler) GetFlow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return
	}

	flow, err := h.flowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈流程不存在")
		return
	}

	response.Success(c, flow)
}

// UpdateFlow 更新自愈流程
func (h *HealingHandler) UpdateFlow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return
	}

	flow, err := h.flowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈流程不存在")
		return
	}

	var req UpdateFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	req.ApplyTo(flow)
	if err := h.flowRepo.Update(c.Request.Context(), flow); err != nil {
		response.InternalError(c, "更新自愈流程失败")
		return
	}

	response.Success(c, flow)
}

// DeleteFlow 删除自愈流程
func (h *HealingHandler) DeleteFlow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return
	}

	if err := h.flowRepo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除自愈流程失败")
		return
	}

	response.Message(c, "删除成功")
}

// DryRunFlow Dry-Run 模拟执行自愈流程
func (h *HealingHandler) DryRunFlow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return
	}

	flow, err := h.flowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈流程不存在")
		return
	}

	var req DryRunFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 创建 Dry-Run 执行器
	dryRunExecutor := healing.NewDryRunExecutor()

	// 转换请求为 MockIncident
	mockIncident := &healing.MockIncident{
		Title:           req.MockIncident.Title,
		Description:     req.MockIncident.Description,
		Severity:        req.MockIncident.Severity,
		Priority:        req.MockIncident.Priority,
		Status:          req.MockIncident.Status,
		Category:        req.MockIncident.Category,
		AffectedCI:      req.MockIncident.AffectedCI,
		AffectedService: req.MockIncident.AffectedService,
		Assignee:        req.MockIncident.Assignee,
		Reporter:        req.MockIncident.Reporter,
		RawData:         req.MockIncident.RawData,
	}

	// 执行 Dry-Run（支持从指定节点开始重试）
	result := dryRunExecutor.Execute(c.Request.Context(), flow, mockIncident, req.FromNodeID, req.Context, req.MockApprovals)

	response.Success(c, result)
}

// DryRunFlowStream Dry-Run 模拟执行自愈流程（SSE 流式输出）
func (h *HealingHandler) DryRunFlowStream(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return
	}

	flow, err := h.flowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈流程不存在")
		return
	}

	var req DryRunFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 创建 SSE 写入器
	sseWriter, err := NewSSEWriter(c)
	if err != nil {
		response.InternalError(c, "SSE 不支持")
		return
	}

	// 创建 Dry-Run 执行器
	dryRunExecutor := healing.NewDryRunExecutor()

	// 转换请求为 MockIncident
	mockIncident := &healing.MockIncident{
		Title:           req.MockIncident.Title,
		Description:     req.MockIncident.Description,
		Severity:        req.MockIncident.Severity,
		Priority:        req.MockIncident.Priority,
		Status:          req.MockIncident.Status,
		Category:        req.MockIncident.Category,
		AffectedCI:      req.MockIncident.AffectedCI,
		AffectedService: req.MockIncident.AffectedService,
		Assignee:        req.MockIncident.Assignee,
		Reporter:        req.MockIncident.Reporter,
		RawData:         req.MockIncident.RawData,
	}

	// SSE 回调函数
	callback := func(eventType string, data map[string]interface{}) {
		sseWriter.WriteEvent(eventType, data)
	}

	// 执行 Dry-Run（带 SSE 回调）
	dryRunExecutor.ExecuteWithCallback(c.Request.Context(), flow, mockIncident, req.FromNodeID, req.Context, req.MockApprovals, callback)
}

// ========== HealingRule 相关 ==========

// ListRules 获取自愈规则列表
func (h *HealingHandler) ListRules(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)

	var isActive *bool
	if str := c.Query("is_active"); str != "" {
		val := str == "true"
		isActive = &val
	}

	var flowID *uuid.UUID
	if str := c.Query("flow_id"); str != "" {
		if val, err := uuid.Parse(str); err == nil {
			flowID = &val
		}
	}

	rules, total, err := h.ruleRepo.List(c.Request.Context(), page, pageSize, isActive, flowID)
	if err != nil {
		response.InternalError(c, "获取自愈规则列表失败")
		return
	}

	response.List(c, rules, total, page, pageSize)
}

// CreateRule 创建自愈规则
func (h *HealingHandler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	rule := req.ToModel()
	if err := h.ruleRepo.Create(c.Request.Context(), rule); err != nil {
		response.InternalError(c, "创建自愈规则失败")
		return
	}

	response.Created(c, rule)
}

// GetRule 获取自愈规则详情
func (h *HealingHandler) GetRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	rule, err := h.ruleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈规则不存在")
		return
	}

	response.Success(c, rule)
}

// UpdateRule 更新自愈规则
func (h *HealingHandler) UpdateRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	rule, err := h.ruleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈规则不存在")
		return
	}

	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	req.ApplyTo(rule)
	if err := h.ruleRepo.Update(c.Request.Context(), rule); err != nil {
		response.InternalError(c, "更新自愈规则失败")
		return
	}

	response.Success(c, rule)
}

// DeleteRule 删除自愈规则
// 支持 force=true 参数强制删除（自动解除关联的流程实例）
func (h *HealingHandler) DeleteRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	force := c.Query("force") == "true"

	if err := h.ruleRepo.Delete(c.Request.Context(), id, force); err != nil {
		if err.Error() == "规则存在关联的执行记录，请使用 force=true 强制删除" {
			response.Conflict(c, err.Error())
			return
		}
		response.InternalError(c, "删除自愈规则失败")
		return
	}

	response.Message(c, "删除成功")
}

// ActivateRule 启用自愈规则
func (h *HealingHandler) ActivateRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	// 检查规则是否关联了流程
	rule, err := h.ruleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "规则不存在")
		return
	}
	if rule.FlowID == nil {
		response.BadRequest(c, "规则必须关联自愈流程才能激活")
		return
	}

	if err := h.ruleRepo.Activate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "启用规则失败")
		return
	}

	response.Message(c, "规则已启用")
}

// DeactivateRule 停用自愈规则
func (h *HealingHandler) DeactivateRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的规则ID")
		return
	}

	if err := h.ruleRepo.Deactivate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "停用规则失败")
		return
	}

	response.Message(c, "规则已停用")
}

// ========== FlowInstance 相关 ==========

// ListInstances 获取流程实例列表
func (h *HealingHandler) ListInstances(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	status := c.Query("status")

	var flowID, ruleID *uuid.UUID
	if str := c.Query("flow_id"); str != "" {
		if val, err := uuid.Parse(str); err == nil {
			flowID = &val
		}
	}
	if str := c.Query("rule_id"); str != "" {
		if val, err := uuid.Parse(str); err == nil {
			ruleID = &val
		}
	}

	instances, total, err := h.instanceRepo.List(c.Request.Context(), page, pageSize, flowID, ruleID, nil, status)
	if err != nil {
		response.InternalError(c, "获取流程实例列表失败")
		return
	}

	response.List(c, instances, total, page, pageSize)
}

// GetInstance 获取流程实例详情
func (h *HealingHandler) GetInstance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	response.Success(c, instance)
}

// CancelInstance 取消流程实例
func (h *HealingHandler) CancelInstance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	// 获取流程实例以获取关联的 IncidentID
	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	if err := h.instanceRepo.UpdateStatus(c.Request.Context(), id, "cancelled", "用户手动取消"); err != nil {
		response.InternalError(c, "取消流程实例失败")
		return
	}

	// 更新关联的 Incident 状态为 skipped（用户主动取消视为跳过）
	if instance.IncidentID != nil {
		if incident, err := h.incidentRepo.GetByID(c.Request.Context(), *instance.IncidentID); err == nil {
			incident.HealingStatus = "skipped"
			h.incidentRepo.Update(c.Request.Context(), incident)
		}
	}

	response.Message(c, "流程实例已取消")
}

// RetryInstance 重试流程实例
func (h *HealingHandler) RetryInstance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	// 解析请求体
	var req struct {
		FromNodeID string `json:"from_node_id"` // 可选，从哪个节点开始
	}
	c.ShouldBindJSON(&req) // 允许为空

	// 异步执行重试
	go func() {
		ctx := context.Background()
		if err := h.executor.RetryFromNode(ctx, instance, req.FromNodeID); err != nil {
			log.Printf("[RETRY] 重试失败: %v", err)
		}
	}()

	response.Message(c, "流程实例正在重试")
}

// InstanceEvents 获取流程实例事件流 (SSE)
func (h *HealingHandler) InstanceEvents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的实例ID")
		return
	}

	// 验证实例存在
	instance, err := h.instanceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "流程实例不存在")
		return
	}

	// 创建 SSE 写入器
	sseWriter, err := NewSSEWriter(c)
	if err != nil {
		response.InternalError(c, "SSE 不支持")
		return
	}

	// 订阅事件
	eventBus := healing.GetEventBus()
	eventCh := eventBus.Subscribe(instance.ID)
	defer eventBus.Unsubscribe(instance.ID, eventCh)

	// 发送初始状态
	sseWriter.WriteEvent("connected", map[string]interface{}{
		"instance_id": instance.ID.String(),
		"status":      instance.Status,
	})

	// 监听事件
	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			// 客户端断开连接
			return
		case event, ok := <-eventCh:
			if !ok {
				// 通道关闭
				return
			}
			// 发送事件
			sseWriter.WriteEvent(string(event.Type), event.Data)

			// 如果是流程完成事件，关闭连接
			if event.Type == healing.EventFlowComplete {
				return
			}
		}
	}
}

// ========== ApprovalTask 相关 ==========

// ListApprovals 获取审批任务列表
func (h *HealingHandler) ListApprovals(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	status := c.Query("status")

	var flowInstanceID *uuid.UUID
	if str := c.Query("flow_instance_id"); str != "" {
		if val, err := uuid.Parse(str); err == nil {
			flowInstanceID = &val
		}
	}

	tasks, total, err := h.approvalRepo.List(c.Request.Context(), page, pageSize, flowInstanceID, status)
	if err != nil {
		response.InternalError(c, "获取审批任务列表失败")
		return
	}

	response.List(c, tasks, total, page, pageSize)
}

// ListPendingApprovals 获取待审批任务列表
// 支持 Query 参数：search（模糊搜索 node_id, flow_instance_id）、date_from、date_to
func (h *HealingHandler) ListPendingApprovals(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	search := c.Query("search")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	tasks, total, err := h.approvalRepo.ListPending(c.Request.Context(), page, pageSize, search, dateFrom, dateTo)
	if err != nil {
		response.InternalError(c, "获取待审批任务列表失败")
		return
	}

	response.List(c, tasks, total, page, pageSize)
}

// GetApproval 获取审批任务详情
func (h *HealingHandler) GetApproval(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的审批任务ID")
		return
	}

	task, err := h.approvalRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "审批任务不存在")
		return
	}

	response.Success(c, task)
}

// ApproveTask 批准审批任务
func (h *HealingHandler) ApproveTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的审批任务ID")
		return
	}

	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	// 从 context 获取当前用户ID
	userID := getCurrentUserID(c)
	if userID == nil {
		response.Unauthorized(c, "未授权")
		return
	}

	// 获取审批任务信息（需要流程实例 ID）
	task, err := h.approvalRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "审批任务不存在")
		return
	}

	// 执行审批
	if err := h.approvalRepo.Approve(c.Request.Context(), id, *userID, req.Comment); err != nil {
		response.InternalError(c, "批准操作失败")
		return
	}

	// 同步继续执行流程（使用新的 context）
	if err := h.executor.ResumeAfterApproval(context.Background(), task.FlowInstanceID, true); err != nil {
		log.Printf("[HealingHandler] 恢复流程执行失败: %v", err)
		// 审批已成功，流程恢复失败不影响响应
	}

	response.Message(c, "审批已通过")
}

// RejectTask 拒绝审批任务
func (h *HealingHandler) RejectTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的审批任务ID")
		return
	}

	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userID := getCurrentUserID(c)
	if userID == nil {
		response.Unauthorized(c, "未授权")
		return
	}

	// 获取审批任务信息
	task, err := h.approvalRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "审批任务不存在")
		return
	}

	if err := h.approvalRepo.Reject(c.Request.Context(), id, *userID, req.Comment); err != nil {
		response.InternalError(c, "拒绝操作失败")
		return
	}

	// 更新 FlowInstance 状态为失败
	h.instanceRepo.UpdateStatus(c.Request.Context(), task.FlowInstanceID, "failed", "审批被拒绝: "+req.Comment)

	// 更新关联的 Incident 状态为 failed
	if instance, err := h.instanceRepo.GetByID(c.Request.Context(), task.FlowInstanceID); err == nil && instance.IncidentID != nil {
		if incident, err := h.incidentRepo.GetByID(c.Request.Context(), *instance.IncidentID); err == nil {
			incident.HealingStatus = "failed"
			h.incidentRepo.Update(c.Request.Context(), incident)
		}
	}

	response.Message(c, "审批已拒绝")
}

// ========== Incident 手动触发相关 ==========

// ListPendingTriggerIncidents 获取待触发工单列表
// 用于待办中心的"待触发工单"标签页
// 支持 Query 参数：search（模糊搜索 title, external_id, affected_ci）、severity、date_from、date_to
func (h *HealingHandler) ListPendingTriggerIncidents(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	search := c.Query("search")
	severity := c.Query("severity")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	incidents, total, err := h.incidentRepo.ListPendingTrigger(c.Request.Context(), page, pageSize, search, severity, dateFrom, dateTo)
	if err != nil {
		response.InternalError(c, "获取待触发工单列表失败")
		return
	}

	response.List(c, incidents, total, page, pageSize)
}

// TriggerIncidentManually 手动触发自愈流程
// 用于待办中心点击"启动自愈"按钮
func (h *HealingHandler) TriggerIncidentManually(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的工单ID")
		return
	}

	// 获取工单
	incident, err := h.incidentRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "工单不存在")
		return
	}

	// 检查是否有匹配的规则
	if incident.MatchedRuleID == nil {
		response.BadRequest(c, "此工单未匹配任何规则")
		return
	}

	// 检查是否已经触发过
	if incident.HealingFlowInstanceID != nil {
		response.BadRequest(c, "此工单已经触发过自愈流程")
		return
	}

	// 调用 scheduler 的 TriggerManual 方法
	instance, err := h.scheduler.TriggerManual(c.Request.Context(), incident.ID.String(), *incident.MatchedRuleID)
	if err != nil {
		response.InternalError(c, "触发自愈流程失败: "+err.Error())
		return
	}

	response.Created(c, instance)
}
