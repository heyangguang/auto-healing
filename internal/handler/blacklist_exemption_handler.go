package handler

import (
	"net/http"
	"strconv"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BlacklistExemptionHandler struct {
	svc *service.BlacklistExemptionService
}

func NewBlacklistExemptionHandler() *BlacklistExemptionHandler {
	return &BlacklistExemptionHandler{
		svc: service.NewBlacklistExemptionService(),
	}
}

// List 豁免申请列表
func (h *BlacklistExemptionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	opts := repository.ExemptionListOptions{
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// Get 获取单条
func (h *BlacklistExemptionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "豁免申请不存在"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Create 创建豁免申请
func (h *BlacklistExemptionHandler) Create(c *gin.Context) {
	var input struct {
		TaskID       string `json:"task_id" binding:"required"`
		TaskName     string `json:"task_name"`
		RuleID       string `json:"rule_id" binding:"required"`
		RuleName     string `json:"rule_name"`
		RuleSeverity string `json:"rule_severity"`
		RulePattern  string `json:"rule_pattern"`
		Reason       string `json:"reason" binding:"required"`
		ValidityDays int    `json:"validity_days"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	taskID, err := uuid.Parse(input.TaskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务模板 ID"})
		return
	}
	ruleID, err := uuid.Parse(input.RuleID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的规则 ID"})
		return
	}

	userID, _ := uuid.Parse(middleware.GetUserID(c))
	username := middleware.GetUsername(c)

	if input.ValidityDays <= 0 {
		input.ValidityDays = 30
	}

	item := &model.BlacklistExemption{
		TaskID:        taskID,
		TaskName:      input.TaskName,
		RuleID:        ruleID,
		RuleName:      input.RuleName,
		RuleSeverity:  input.RuleSeverity,
		RulePattern:   input.RulePattern,
		Reason:        input.Reason,
		RequestedBy:   userID,
		RequesterName: username,
		ValidityDays:  input.ValidityDays,
	}

	if err := h.svc.Create(c.Request.Context(), item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// Approve 审批通过
func (h *BlacklistExemptionHandler) Approve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	userID, _ := uuid.Parse(middleware.GetUserID(c))
	username := middleware.GetUsername(c)

	if err := h.svc.Approve(c.Request.Context(), id, userID, username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "豁免已批准"})
}

// Reject 审批拒绝
func (h *BlacklistExemptionHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	var input struct {
		RejectReason string `json:"reject_reason"`
	}
	_ = c.ShouldBindJSON(&input)

	userID, _ := uuid.Parse(middleware.GetUserID(c))
	username := middleware.GetUsername(c)

	if err := h.svc.Reject(c.Request.Context(), id, userID, username, input.RejectReason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "豁免已拒绝"})
}

// GetPending 获取待审批列表（审批中心用）
func (h *BlacklistExemptionHandler) GetPending(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	opts := repository.ExemptionListOptions{
		Page:      page,
		PageSize:  pageSize,
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	items, total, err := h.svc.ListPending(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// GetSearchSchema 搜索字段定义
func (h *BlacklistExemptionHandler) GetSearchSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
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
