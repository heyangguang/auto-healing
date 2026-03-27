package httpapi

import (
	"errors"
	"strings"

	"github.com/company/auto-healing/internal/middleware"
	rootmodel "github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	opsmodel "github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

func NewBlacklistExemptionHandler() *BlacklistExemptionHandler {
	return NewBlacklistExemptionHandlerWithDeps(BlacklistExemptionHandlerDeps{
		Service:       opsservice.NewBlacklistExemptionService(),
		TaskRepo:      automationrepo.NewExecutionRepository(),
		BlacklistRepo: opsrepo.NewCommandBlacklistRepository(),
	})
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

type blacklistExemptionCreateInput struct {
	TaskID       string `json:"task_id" binding:"required"`
	TaskName     string `json:"task_name"`
	RuleID       string `json:"rule_id" binding:"required"`
	RuleName     string `json:"rule_name"`
	RuleSeverity string `json:"rule_severity"`
	RulePattern  string `json:"rule_pattern"`
	Reason       string `json:"reason" binding:"required"`
	ValidityDays int    `json:"validity_days"`
}

func parseBlacklistExemptionCreateInput(c *gin.Context) (*blacklistExemptionCreateInput, bool) {
	var input blacklistExemptionCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return nil, false
	}
	if input.ValidityDays <= 0 {
		input.ValidityDays = 30
	}
	return &input, true
}

func parseBlacklistExemptionIDs(c *gin.Context, taskIDValue, ruleIDValue string) (uuid.UUID, uuid.UUID, bool) {
	taskID, err := uuid.Parse(taskIDValue)
	if err != nil {
		response.BadRequest(c, "无效的任务模板 ID")
		return uuid.Nil, uuid.Nil, false
	}
	ruleID, err := uuid.Parse(ruleIDValue)
	if err != nil {
		response.BadRequest(c, "无效的规则 ID")
		return uuid.Nil, uuid.Nil, false
	}
	return taskID, ruleID, true
}

func (h *BlacklistExemptionHandler) loadBlacklistExemptionDependencies(c *gin.Context, taskID, ruleID uuid.UUID) (*rootmodel.ExecutionTask, *opsmodel.CommandBlacklist, bool) {
	task, err := h.taskRepo.GetTaskByID(c.Request.Context(), taskID)
	if err != nil {
		respondBlacklistExemptionDependencyError(c, err, "任务模板不存在或不属于当前租户", "查询任务模板失败")
		return nil, nil, false
	}
	rule, err := h.blacklistRepo.GetByID(c.Request.Context(), ruleID)
	if err != nil {
		respondBlacklistExemptionDependencyError(c, err, "黑名单规则不存在或不属于当前租户", "查询黑名单规则失败")
		return nil, nil, false
	}
	return task, rule, true
}

func buildBlacklistExemptionModel(c *gin.Context, userID uuid.UUID, input *blacklistExemptionCreateInput, taskID, ruleID uuid.UUID, taskName, ruleName, ruleSeverity, rulePattern string) *opsmodel.BlacklistExemption {
	return &opsmodel.BlacklistExemption{
		TaskID:        taskID,
		TaskName:      taskName,
		RuleID:        ruleID,
		RuleName:      ruleName,
		RuleSeverity:  ruleSeverity,
		RulePattern:   rulePattern,
		Reason:        input.Reason,
		RequestedBy:   userID,
		RequesterName: middleware.GetUsername(c),
		ValidityDays:  input.ValidityDays,
	}
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
	response.Success(c, gin.H{
		"fields": []map[string]interface{}{
			{"key": "task_name", "label": "任务模板", "type": "text"},
			{"key": "rule_name", "label": "规则名称", "type": "text"},
			{"key": "requester_name", "label": "申请人", "type": "text"},
			{
				"key":   "status",
				"label": "状态",
				"type":  "enum",
				"options": []map[string]string{
					{"label": "待审批", "value": "pending"},
					{"label": "已批准", "value": "approved"},
					{"label": "已拒绝", "value": "rejected"},
					{"label": "已过期", "value": "expired"},
				},
			},
		},
	})
}

func requireBlacklistExemptionUserID(c *gin.Context) (uuid.UUID, bool) {
	userID := parseBlacklistExemptionUserID(middleware.GetUserID(c))
	if userID == uuid.Nil {
		respondInternalError(c, "BLACKLIST", "用户上下文缺失", errors.New("invalid user id in context"))
		return uuid.Nil, false
	}
	return userID, true
}

func parseBlacklistExemptionUserID(raw string) uuid.UUID {
	userID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return userID
}

func respondBlacklistExemptionLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "豁免申请不存在")
		return
	}
	respondInternalError(c, "BLACKLIST", "查询豁免申请失败", err)
}

func respondBlacklistExemptionDependencyError(c *gin.Context, err error, notFoundMsg, internalMsg string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.BadRequest(c, notFoundMsg)
		return
	}
	respondInternalError(c, "BLACKLIST", internalMsg, err)
}

func respondBlacklistExemptionMutationError(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.NotFound(c, "豁免申请不存在")
	case errors.Is(err, opsrepo.ErrBlacklistExemptionNotPending):
		response.Conflict(c, "该豁免申请已被其他审批人处理")
	case errors.Is(err, platformrepo.ErrTenantContextRequired):
		respondInternalError(c, "BLACKLIST", internalMsg, err)
	default:
		if isBlacklistExemptionBadRequest(err) {
			response.BadRequest(c, err.Error())
			return
		}
		respondInternalError(c, "BLACKLIST", internalMsg, err)
	}
}

func isBlacklistExemptionBadRequest(err error) bool {
	if err == nil {
		return false
	}
	for _, prefix := range []string{
		"该任务模板已有相同规则的待审批豁免申请",
		"只有待审批的申请才能审批",
		"申请人不能审批自己的豁免申请",
	} {
		if strings.HasPrefix(err.Error(), prefix) {
			return true
		}
	}
	return false
}
