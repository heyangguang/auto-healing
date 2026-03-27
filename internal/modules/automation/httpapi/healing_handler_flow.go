package httpapi

import (
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListFlows 获取自愈流程列表
// 支持 Query 参数：search, is_active, name, description, node_type, min_nodes, max_nodes, created_from, created_to, updated_from, updated_to, sort_by, sort_order
func (h *HealingHandler) ListFlows(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	name := GetStringFilter(c, "name")
	description := GetStringFilter(c, "description")
	nodeType := c.Query("node_type")
	createdFrom := c.Query("created_from")
	createdTo := c.Query("created_to")
	updatedFrom := c.Query("updated_from")
	updatedTo := c.Query("updated_to")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	var isActive *bool
	if str := c.Query("is_active"); str != "" {
		val := str == "true"
		isActive = &val
	}

	var minNodes *int
	if str := c.Query("min_nodes"); str != "" {
		if val := getQueryInt(c, "min_nodes", -1); val >= 0 {
			minNodes = &val
		}
	}

	var maxNodes *int
	if str := c.Query("max_nodes"); str != "" {
		if val := getQueryInt(c, "max_nodes", -1); val >= 0 {
			maxNodes = &val
		}
	}

	flows, total, err := h.flowRepo.List(c.Request.Context(), page, pageSize, isActive, query.StringFilter{}, name, description, nodeType, minNodes, maxNodes, createdFrom, createdTo, updatedFrom, updatedTo, sortBy, sortOrder)
	if err != nil {
		response.InternalError(c, "获取自愈流程列表失败")
		return
	}

	// 填充通知节点名称
	h.enrichFlowNodes(c.Request.Context(), flows)

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

	// 填充通知节点名称（Nodes 内部是 map 引用，enrichFlowNodes 会直接修改）
	enriched := []model.HealingFlow{*flow}
	h.enrichFlowNodes(c.Request.Context(), enriched)

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

// DeleteFlow 删除自愈流程（保护性删除）
func (h *HealingHandler) DeleteFlow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return
	}

	ctx := c.Request.Context()

	// 检查是否有规则引用该流程
	ruleCount, err := h.flowRepo.CountRulesUsingFlow(ctx, id)
	if err != nil {
		response.InternalError(c, "检查关联规则失败")
		return
	}
	if ruleCount > 0 {
		response.Conflict(c, "无法删除：有 "+fmt.Sprintf("%d", ruleCount)+" 个自愈规则引用此流程，请先修改这些规则的流程关联")
		return
	}

	// 检查是否有运行中/待审批的实例
	activeCount, err := h.flowRepo.CountActiveInstancesByFlowID(ctx, id)
	if err != nil {
		response.InternalError(c, "检查关联实例失败")
		return
	}
	if activeCount > 0 {
		response.Conflict(c, "无法删除：有 "+fmt.Sprintf("%d", activeCount)+" 个运行中或待审批的流程实例，请等待执行完成或取消后再删除")
		return
	}

	if err := h.flowRepo.Delete(ctx, id); err != nil {
		response.InternalError(c, "删除自愈流程失败")
		return
	}

	response.Message(c, "删除成功")
}
