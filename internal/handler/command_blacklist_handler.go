package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CommandBlacklistHandler 高危指令黑名单处理器
type CommandBlacklistHandler struct {
	svc *service.CommandBlacklistService
}

// NewCommandBlacklistHandler 创建处理器
func NewCommandBlacklistHandler() *CommandBlacklistHandler {
	return &CommandBlacklistHandler{
		svc: service.NewCommandBlacklistService(),
	}
}

// List 列表查询
// GET /api/v1/command-blacklist
func (h *CommandBlacklistHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.CommandBlacklistListOptions{
		Page:         page,
		PageSize:     pageSize,
		Name:         c.Query("name"),
		NameExact:    c.Query("name__exact"),
		Category:     c.Query("category"),
		Severity:     c.Query("severity"),
		Pattern:      c.Query("pattern"),
		PatternExact: c.Query("pattern__exact"),
	}

	// is_active 筛选
	if activeParam := c.Query("is_active"); activeParam != "" {
		active := activeParam == "true"
		opts.IsActive = &active
	}

	rules, total, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      rules,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create 创建规则
// POST /api/v1/command-blacklist
func (h *CommandBlacklistHandler) Create(c *gin.Context) {
	var rule model.CommandBlacklist
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
		return
	}

	if rule.Name == "" || rule.Pattern == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 和 pattern 为必填项"})
		return
	}

	if err := h.svc.Create(c.Request.Context(), &rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": rule})
}

// Get 获取详情
// GET /api/v1/command-blacklist/:id
func (h *CommandBlacklistHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	rule, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rule})
}

// Update 更新规则
// PUT /api/v1/command-blacklist/:id
func (h *CommandBlacklistHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	var input model.CommandBlacklist
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
		return
	}

	rule, err := h.svc.Update(c.Request.Context(), id, &input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rule})
}

// Delete 删除规则
// DELETE /api/v1/command-blacklist/:id
func (h *CommandBlacklistHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ToggleActive 启用/禁用
// POST /api/v1/command-blacklist/:id/toggle
func (h *CommandBlacklistHandler) ToggleActive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	rule, err := h.svc.ToggleActive(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rule})
}

// GetSearchSchema 搜索 Schema
// GET /api/v1/command-blacklist/search-schema
func (h *CommandBlacklistHandler) GetSearchSchema(c *gin.Context) {
	schema := map[string]interface{}{
		"fields": []map[string]interface{}{
			{
				"key":           "name",
				"label":         "规则名称",
				"type":          "string",
				"support_exact": true,
			},
			{
				"key":           "pattern",
				"label":         "匹配模式",
				"type":          "string",
				"support_exact": true,
			},
			{
				"key":   "category",
				"label": "分类",
				"type":  "enum",
				"options": []map[string]string{
					{"value": "filesystem", "label": "文件系统"},
					{"value": "network", "label": "网络"},
					{"value": "system", "label": "系统"},
					{"value": "database", "label": "数据库"},
				},
			},
			{
				"key":   "severity",
				"label": "严重级别",
				"type":  "enum",
				"options": []map[string]string{
					{"value": "critical", "label": "严重"},
					{"value": "high", "label": "高危"},
					{"value": "medium", "label": "中危"},
				},
			},
			{
				"key":   "is_active",
				"label": "状态",
				"type":  "enum",
				"options": []map[string]string{
					{"value": "true", "label": "启用"},
					{"value": "false", "label": "禁用"},
				},
			},
		},
		"generated_at": time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, schema)
}

// BatchToggle 批量启用/禁用
// POST /api/v1/command-blacklist/batch-toggle
func (h *CommandBlacklistHandler) BatchToggle(c *gin.Context) {
	var input struct {
		IDs      []string `json:"ids" binding:"required"`
		IsActive bool     `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
		return
	}

	var uuids []uuid.UUID
	for _, idStr := range input.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID: " + idStr})
			return
		}
		uuids = append(uuids, id)
	}

	count, err := h.svc.BatchToggle(c.Request.Context(), uuids, input.IsActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已%s %d 条规则", map[bool]string{true: "启用", false: "禁用"}[input.IsActive], count),
		"count":   count,
	})
}

// Simulate 仿真测试
// POST /api/v1/command-blacklist/simulate
func (h *CommandBlacklistHandler) Simulate(c *gin.Context) {
	var req service.SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
		return
	}

	if req.Pattern == "" || req.MatchType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pattern 和 match_type 为必填项"})
		return
	}

	results, err := h.svc.Simulate(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 统计命中
	matchCount := 0
	matchedFiles := map[string]int{}
	for _, r := range results {
		if r.Matched {
			matchCount++
			if r.File != "" {
				matchedFiles[r.File]++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"results":       results,
			"total_lines":   len(results),
			"match_count":   matchCount,
			"matched_files": matchedFiles,
		},
	})
}

// GetService 返回服务实例（用于 Seed）
func (h *CommandBlacklistHandler) GetService() *service.CommandBlacklistService {
	return h.svc
}
