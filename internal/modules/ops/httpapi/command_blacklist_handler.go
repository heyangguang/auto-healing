package httpapi

import (
	"time"

	"github.com/company/auto-healing/internal/modules/ops/model"
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
	response.Success(c, commandBlacklistSearchSchemaResponse{
		Fields: []commandBlacklistSearchField{
			{Key: "name", Label: "规则名称", Type: "string", SupportExact: true},
			{Key: "pattern", Label: "匹配模式", Type: "string", SupportExact: true},
			{
				Key:   "category",
				Label: "分类",
				Type:  "enum",
				Options: []commandBlacklistSearchOption{
					{Value: "filesystem", Label: "文件系统"},
					{Value: "network", Label: "网络"},
					{Value: "system", Label: "系统"},
					{Value: "database", Label: "数据库"},
				},
			},
			{
				Key:   "severity",
				Label: "严重级别",
				Type:  "enum",
				Options: []commandBlacklistSearchOption{
					{Value: "critical", Label: "严重"},
					{Value: "high", Label: "高危"},
					{Value: "medium", Label: "中危"},
				},
			},
			{
				Key:   "is_active",
				Label: "状态",
				Type:  "enum",
				Options: []commandBlacklistSearchOption{
					{Value: "true", Label: "启用"},
					{Value: "false", Label: "禁用"},
				},
			},
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
	})
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

	response.Success(c, commandBlacklistBatchToggleResponse{Count: count})
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

	response.Success(c, commandBlacklistSimulateResponse{
		Results:      results,
		TotalLines:   len(results),
		MatchCount:   matchCount,
		MatchedFiles: matchedFiles,
	})
}

// GetService 返回服务实例（用于 Seed）
func (h *CommandBlacklistHandler) GetService() *opsservice.CommandBlacklistService {
	return h.svc
}
