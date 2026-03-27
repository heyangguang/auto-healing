package httpapi

import (
	"github.com/company/auto-healing/internal/modules/automation/model"
	healing "github.com/company/auto-healing/internal/modules/automation/service/healing"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DryRunFlow Dry-Run 模拟执行自愈流程
func (h *HealingHandler) DryRunFlow(c *gin.Context) {
	flow, req, ok := h.loadDryRunFlowRequest(c)
	if !ok {
		return
	}
	result := healing.NewDryRunExecutor().Execute(c.Request.Context(), flow, buildMockIncident(req), req.FromNodeID, req.Context, req.MockApprovals)
	response.Success(c, result)
}

// DryRunFlowStream Dry-Run 模拟执行自愈流程（SSE 流式输出）
func (h *HealingHandler) DryRunFlowStream(c *gin.Context) {
	flow, req, ok := h.loadDryRunFlowRequest(c)
	if !ok {
		return
	}

	sseWriter, err := NewSSEWriter(c)
	if err != nil {
		response.InternalError(c, "SSE 不支持")
		return
	}

	callback := func(eventType string, data map[string]interface{}) {
		sseWriter.WriteEvent(eventType, data)
	}
	healing.NewDryRunExecutor().ExecuteWithCallback(c.Request.Context(), flow, buildMockIncident(req), req.FromNodeID, req.Context, req.MockApprovals, callback)
}

func (h *HealingHandler) loadDryRunFlowRequest(c *gin.Context) (*model.HealingFlow, DryRunFlowRequest, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的流程ID")
		return nil, DryRunFlowRequest{}, false
	}

	flow, err := h.flowRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "自愈流程不存在")
		return nil, DryRunFlowRequest{}, false
	}

	var req DryRunFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return nil, DryRunFlowRequest{}, false
	}
	return flow, req, true
}

func buildMockIncident(req DryRunFlowRequest) *healing.MockIncident {
	return &healing.MockIncident{
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
}
