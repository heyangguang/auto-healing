package handler

import (
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/service/playbook"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PlaybookHandler Playbook 处理器
type PlaybookHandler struct {
	svc *playbook.Service
}

// NewPlaybookHandler 创建 Playbook 处理器
func NewPlaybookHandler() *PlaybookHandler {
	return &PlaybookHandler{
		svc: playbook.NewService(),
	}
}

// ==================== CRUD ====================

// Create 创建 Playbook
func (h *PlaybookHandler) Create(c *gin.Context) {
	var req CreatePlaybookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	playbook, err := h.svc.Create(c.Request.Context(), req.RepositoryID, req.Name, req.FilePath, req.Description, req.ConfigMode)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, playbook)
}

// List 列出 Playbooks
func (h *PlaybookHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	opts := &repository.PlaybookListOptions{
		Page:       page,
		PageSize:   pageSize,
		Search:     c.Query("search"),
		Name:       c.Query("name"),
		FilePath:   c.Query("file_path"),
		Status:     c.Query("status"),
		ConfigMode: c.Query("config_mode"),
		SortField:  c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
	}

	// 解析 repository_id
	if repoIDStr := c.Query("repository_id"); repoIDStr != "" {
		if id, err := uuid.Parse(repoIDStr); err == nil {
			opts.RepositoryID = &id
		}
	}

	// 解析 has_variables (bool)
	if hasVarsStr := c.Query("has_variables"); hasVarsStr != "" {
		hasVars := hasVarsStr == "true"
		opts.HasVariables = &hasVars
	}

	// 解析 min_variables (int)
	if minVarsStr := c.Query("min_variables"); minVarsStr != "" {
		if v, err := strconv.Atoi(minVarsStr); err == nil && v >= 0 {
			opts.MinVariables = &v
		}
	}

	// 解析 max_variables (int)
	if maxVarsStr := c.Query("max_variables"); maxVarsStr != "" {
		if v, err := strconv.Atoi(maxVarsStr); err == nil && v >= 0 {
			opts.MaxVariables = &v
		}
	}

	// 解析 has_required_vars (bool)
	if hasReqStr := c.Query("has_required_vars"); hasReqStr != "" {
		hasReq := hasReqStr == "true"
		opts.HasRequiredVars = &hasReq
	}

	// 解析时间范围
	if createdFrom := c.Query("created_from"); createdFrom != "" {
		t, err := time.Parse(time.RFC3339, createdFrom)
		if err == nil {
			opts.CreatedFrom = &t
		}
	}
	if createdTo := c.Query("created_to"); createdTo != "" {
		t, err := time.Parse(time.RFC3339, createdTo)
		if err == nil {
			opts.CreatedTo = &t
		}
	}

	playbooks, total, err := h.svc.ListWithOptions(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, playbooks, total, page, pageSize)
}

// Get 获取 Playbook
func (h *PlaybookHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	playbook, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Playbook不存在")
		return
	}

	response.Success(c, playbook)
}

// Update 更新 Playbook
func (h *PlaybookHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req UpdatePlaybookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req.Name, req.Description); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Message(c, "更新成功")
}

// Delete 删除 Playbook
func (h *PlaybookHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Message(c, "删除成功")
}

// ==================== 状态操作 ====================

// SetReady 设置为 ready 状态
func (h *PlaybookHandler) SetReady(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.SetReady(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "已设置为 ready 状态")
}

// SetOffline 设置为 pending 状态（下线）
func (h *PlaybookHandler) SetOffline(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.SetOffline(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Message(c, "已下线")
}

// GetFiles 获取扫描过的文件列表
func (h *PlaybookHandler) GetFiles(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	files, err := h.svc.GetFiles(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, map[string]any{"files": files})
}

// ==================== 变量扫描 ====================

// ScanVariables 扫描变量
func (h *PlaybookHandler) ScanVariables(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	log, err := h.svc.ScanVariables(c.Request.Context(), id, "manual")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, log)
}

// UpdateVariables 更新变量配置
func (h *PlaybookHandler) UpdateVariables(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req UpdateVariablesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := h.svc.UpdateUserVariables(c.Request.Context(), id, req.Variables); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Message(c, "变量更新成功")
}

// GetScanLogs 获取扫描日志
func (h *PlaybookHandler) GetScanLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.svc.GetScanLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.List(c, logs, total, page, pageSize)
}

// ==================== 统计 ====================

// GetStats 获取 Playbook 统计信息
func (h *PlaybookHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取统计信息失败:"+err.Error())
		return
	}
	response.Success(c, stats)
}

// ==================== DTO ====================

// CreatePlaybookRequest 创建 Playbook 请求
type CreatePlaybookRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" binding:"required"`
	Name         string    `json:"name" binding:"required"`
	FilePath     string    `json:"file_path" binding:"required"`
	Description  string    `json:"description"`
	ConfigMode   string    `json:"config_mode"` // auto / enhanced，创建时必须指定
}

// UpdatePlaybookRequest 更新 Playbook 请求
type UpdatePlaybookRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateVariablesRequest 更新变量请求
type UpdateVariablesRequest struct {
	Variables model.JSONArray `json:"variables" binding:"required"`
}
