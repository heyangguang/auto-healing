package httpapi

import (
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/model"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CommandBlacklistHandler 高危指令黑名单处理器
type CommandBlacklistHandler struct {
	svc *opsservice.CommandBlacklistService
}

type CommandBlacklistHandlerDeps struct {
	Service *opsservice.CommandBlacklistService
}

// NewCommandBlacklistHandler 创建处理器
func NewCommandBlacklistHandler() *CommandBlacklistHandler {
	return NewCommandBlacklistHandlerWithDeps(CommandBlacklistHandlerDeps{
		Service: opsservice.NewCommandBlacklistService(),
	})
}

func NewCommandBlacklistHandlerWithDeps(deps CommandBlacklistHandlerDeps) *CommandBlacklistHandler {
	return &CommandBlacklistHandler{
		svc: deps.Service,
	}
}

// List 列表查询
// GET /api/v1/command-blacklist
func (h *CommandBlacklistHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts, err := buildCommandBlacklistListOptions(c, page, pageSize)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rules, total, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "BLACKLIST", "查询黑名单规则失败", err)
		return
	}

	response.List(c, rules, total, page, pageSize)
}

// Create 创建规则
// POST /api/v1/command-blacklist
func (h *CommandBlacklistHandler) Create(c *gin.Context) {
	var rule model.CommandBlacklist
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	if rule.Name == "" || rule.Pattern == "" {
		response.BadRequest(c, "name 和 pattern 为必填项")
		return
	}

	if err := h.svc.Create(c.Request.Context(), &rule); err != nil {
		respondCommandBlacklistError(c, "创建黑名单规则失败", err)
		return
	}

	response.Created(c, rule)
}

// Get 获取详情
// GET /api/v1/command-blacklist/:id
func (h *CommandBlacklistHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	rule, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondCommandBlacklistError(c, "获取黑名单规则失败", err)
		return
	}

	response.Success(c, rule)
}

// Update 更新规则
// PUT /api/v1/command-blacklist/:id
func (h *CommandBlacklistHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	var input model.CommandBlacklist
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	rule, err := h.svc.Update(c.Request.Context(), id, &input)
	if err != nil {
		respondCommandBlacklistError(c, "更新黑名单规则失败", err)
		return
	}

	response.Success(c, rule)
}

// Delete 删除规则
// DELETE /api/v1/command-blacklist/:id
func (h *CommandBlacklistHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondCommandBlacklistError(c, "删除黑名单规则失败", err)
		return
	}

	response.Message(c, "删除成功")
}

// ToggleActive 启用/禁用
// POST /api/v1/command-blacklist/:id/toggle
func (h *CommandBlacklistHandler) ToggleActive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	rule, err := h.svc.ToggleActive(c.Request.Context(), id)
	if err != nil {
		respondCommandBlacklistError(c, "切换黑名单规则状态失败", err)
		return
	}

	response.Success(c, rule)
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

	response.Success(c, schema)
}

// BatchToggle 批量启用/禁用
// POST /api/v1/command-blacklist/batch-toggle
func (h *CommandBlacklistHandler) BatchToggle(c *gin.Context) {
	var input struct {
		IDs      []string `json:"ids" binding:"required"`
		IsActive *bool    `json:"is_active" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	var uuids []uuid.UUID
	for _, idStr := range input.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			response.BadRequest(c, "无效的 ID: "+idStr)
			return
		}
		uuids = append(uuids, id)
	}

	count, err := h.svc.BatchToggle(c.Request.Context(), uuids, *input.IsActive)
	if err != nil {
		respondInternalError(c, "BLACKLIST", "批量更新黑名单规则状态失败", err)
		return
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("已%s %d 条规则", map[bool]string{true: "启用", false: "禁用"}[*input.IsActive], count),
		"count":   count,
	})
}

// Simulate 仿真测试
// POST /api/v1/command-blacklist/simulate
func (h *CommandBlacklistHandler) Simulate(c *gin.Context) {
	var req opsservice.SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	if req.Pattern == "" || req.MatchType == "" {
		response.BadRequest(c, "pattern 和 match_type 为必填项")
		return
	}

	results, err := h.svc.Simulate(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
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

	response.Success(c, gin.H{
		"data": gin.H{
			"results":       results,
			"total_lines":   len(results),
			"match_count":   matchCount,
			"matched_files": matchedFiles,
		},
	})
}

// GetService 返回服务实例（用于 Seed）
func (h *CommandBlacklistHandler) GetService() *opsservice.CommandBlacklistService {
	return h.svc
}
