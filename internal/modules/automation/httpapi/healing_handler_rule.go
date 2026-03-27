package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== HealingRule 相关 ==========

// ListRules 获取自愈规则列表
// 支持 Query 参数：search, is_active, flow_id, trigger_mode, priority, match_mode, has_flow, created_from, created_to, sort_by, sort_order
func (h *HealingHandler) ListRules(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	triggerMode := c.Query("trigger_mode")
	matchMode := c.Query("match_mode")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")
	createdFrom := c.Query("created_from")
	createdTo := c.Query("created_to")

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

	var priority *int
	if str := c.Query("priority"); str != "" {
		if val := getQueryInt(c, "priority", -1); val >= 0 {
			priority = &val
		}
	}

	var hasFlow *bool
	if str := c.Query("has_flow"); str != "" {
		val := str == "true"
		hasFlow = &val
	}

	scopes := BuildSchemaScopes(c, ruleSearchSchema)

	rules, total, err := h.ruleRepo.List(c.Request.Context(), page, pageSize, isActive, flowID, query.StringFilter{}, triggerMode, sortBy, sortOrder, priority, matchMode, hasFlow, createdFrom, createdTo, scopes...)
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
