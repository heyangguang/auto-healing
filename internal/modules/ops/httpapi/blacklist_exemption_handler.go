package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BlacklistExemptionHandler struct {
	svc           *opsservice.BlacklistExemptionService
	taskRepo      *automationrepo.ExecutionRepository
	blacklistRepo *opsrepo.CommandBlacklistRepository
}

type BlacklistExemptionHandlerDeps struct {
	Service       *opsservice.BlacklistExemptionService
	TaskRepo      *automationrepo.ExecutionRepository
	BlacklistRepo *opsrepo.CommandBlacklistRepository
}

func NewBlacklistExemptionHandlerWithDeps(deps BlacklistExemptionHandlerDeps) *BlacklistExemptionHandler {
	return &BlacklistExemptionHandler{
		svc:           deps.Service,
		taskRepo:      deps.TaskRepo,
		blacklistRepo: deps.BlacklistRepo,
	}
}

// List 豁免申请列表
func (h *BlacklistExemptionHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c, 10)

	opts := opsrepo.ExemptionListOptions{
		Page:      page,
		PageSize:  pageSize,
		Status:    c.Query("status"),
		TaskID:    c.Query("task_id"),
		RuleID:    c.Query("rule_id"),
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	items, total, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "BLACKLIST", "获取黑名单豁免列表失败", err)
		return
	}
	response.List(c, items, total, page, pageSize)
}

// Get 获取单条
func (h *BlacklistExemptionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondBlacklistExemptionLookupError(c, err)
		return
	}
	response.Success(c, item)
}

// Create 创建豁免申请
func (h *BlacklistExemptionHandler) Create(c *gin.Context) {
	input, ok := parseBlacklistExemptionCreateInput(c)
	if !ok {
		return
	}
	taskID, ruleID, ok := parseBlacklistExemptionIDs(c, input.TaskID, input.RuleID)
	if !ok {
		return
	}
	task, rule, ok := h.loadBlacklistExemptionDependencies(c, taskID, ruleID)
	if !ok {
		return
	}
	userID, ok := requireBlacklistExemptionUserID(c)
	if !ok {
		return
	}
	item := buildBlacklistExemptionModel(c, userID, input, taskID, ruleID, task.Name, rule.Name, rule.Severity, rule.Pattern)
	if err := h.svc.Create(c.Request.Context(), item); err != nil {
		respondBlacklistExemptionMutationError(c, err, "创建豁免申请失败")
		return
	}
	response.Created(c, item)
}

// Approve 审批通过
func (h *BlacklistExemptionHandler) Approve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	userID, ok := requireBlacklistExemptionUserID(c)
	if !ok {
		return
	}
	username := middleware.GetUsername(c)

	if err := h.svc.Approve(c.Request.Context(), id, userID, username); err != nil {
		respondBlacklistExemptionMutationError(c, err, "审批豁免申请失败")
		return
	}
	response.Message(c, "豁免已批准")
}

// Reject 审批拒绝
func (h *BlacklistExemptionHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	var input struct {
		RejectReason string `json:"reject_reason"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&input); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
	}

	userID, ok := requireBlacklistExemptionUserID(c)
	if !ok {
		return
	}
	username := middleware.GetUsername(c)

	if err := h.svc.Reject(c.Request.Context(), id, userID, username, input.RejectReason); err != nil {
		respondBlacklistExemptionMutationError(c, err, "拒绝豁免申请失败")
		return
	}
	response.Message(c, "豁免已拒绝")
}

// GetPending 获取待审批列表（审批中心用）
func (h *BlacklistExemptionHandler) GetPending(c *gin.Context) {
	page, pageSize := parsePagination(c, 10)

	opts := opsrepo.ExemptionListOptions{
		Page:      page,
		PageSize:  pageSize,
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	items, total, err := h.svc.ListPending(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "BLACKLIST", "获取待审批豁免列表失败", err)
		return
	}
	response.List(c, items, total, page, pageSize)
}

// GetSearchSchema 搜索字段定义
func (h *BlacklistExemptionHandler) GetSearchSchema(c *gin.Context) {
	response.Success(c, []blacklistExemptionSearchField{
		{Key: "task_name", Label: "任务模板", Type: "text"},
		{Key: "rule_name", Label: "规则名称", Type: "text"},
		{Key: "requester_name", Label: "申请人", Type: "text"},
		{
			Key:   "status",
			Label: "状态",
			Type:  "enum",
			Options: []blacklistExemptionSearchOption{
				{Label: "待审批", Value: "pending"},
				{Label: "已批准", Value: "approved"},
				{Label: "已拒绝", Value: "rejected"},
				{Label: "已过期", Value: "expired"},
			},
		},
	})
}
