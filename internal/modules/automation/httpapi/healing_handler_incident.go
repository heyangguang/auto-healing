package httpapi

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== Incident 手动触发相关 ==========

// ListPendingTriggerIncidents 获取待触发工单列表
// 用于待办中心的"待触发工单"标签页
// 支持 Query 参数：title（模糊搜索 title, external_id, affected_ci）、severity、date_from、date_to
func (h *HealingHandler) ListPendingTriggerIncidents(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	title := c.Query("title")
	severity := c.Query("severity")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	incidents, total, err := h.incidentRepo.ListPendingTrigger(c.Request.Context(), page, pageSize, title, severity, dateFrom, dateTo)
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
		respondInternalError(c, "HEAL", "触发自愈流程失败", err)
		return
	}

	response.Created(c, instance)
}

// DismissIncident 忽略待触发工单
// 将工单 healing_status 从 pending 改为 skipped，使其不再出现在待触发列表中
func (h *HealingHandler) DismissIncident(c *gin.Context) {
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

	// 检查工单是否处于待触发状态
	if incident.HealingStatus != "pending" {
		response.BadRequest(c, "只能忽略待触发状态的工单")
		return
	}

	// 更新状态为 dismissed（用户主动忽略）
	incident.HealingStatus = "dismissed"
	if err := h.incidentRepo.Update(c.Request.Context(), incident); err != nil {
		response.InternalError(c, "忽略工单失败")
		return
	}

	response.Message(c, "工单已忽略")
}

// ListDismissedTriggerIncidents 获取已忽略的待触发工单列表
// 用于待办中心的"已忽略"标签页
func (h *HealingHandler) ListDismissedTriggerIncidents(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	title := c.Query("title")
	severity := c.Query("severity")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	incidents, total, err := h.incidentRepo.ListDismissedTrigger(c.Request.Context(), page, pageSize, title, severity, dateFrom, dateTo)
	if err != nil {
		response.InternalError(c, "获取已忽略工单列表失败")
		return
	}

	response.List(c, incidents, total, page, pageSize)
}

// ==================== 统计 ====================

// GetFlowStats 获取自愈流程统计信息
func (h *HealingHandler) GetFlowStats(c *gin.Context) {
	stats, err := h.flowRepo.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "HEAL", "获取流程统计信息失败", err)
		return
	}
	response.Success(c, stats)
}

// GetRuleStats 获取自愈规则统计信息
func (h *HealingHandler) GetRuleStats(c *gin.Context) {
	stats, err := h.ruleRepo.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "HEAL", "获取规则统计信息失败", err)
		return
	}
	response.Success(c, stats)
}

// GetInstanceStats 获取流程实例统计信息
func (h *HealingHandler) GetInstanceStats(c *gin.Context) {
	stats, err := h.instanceRepo.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "HEAL", "获取实例统计信息失败", err)
		return
	}
	response.Success(c, stats)
}
