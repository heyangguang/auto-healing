package handler

import (
	"strconv"

	"github.com/company/auto-healing/internal/pkg/response"
	gitSvc "github.com/company/auto-healing/internal/service/git"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GitRepoHandler Git 仓库处理器
type GitRepoHandler struct {
	svc *gitSvc.Service
}

// NewGitRepoHandler 创建 Git 仓库处理器
func NewGitRepoHandler() *GitRepoHandler {
	return &GitRepoHandler{
		svc: gitSvc.NewService(),
	}
}

// ListRepos 获取仓库列表
func (h *GitRepoHandler) ListRepos(c *gin.Context) {
	status := c.Query("status")

	repos, err := h.svc.ListRepos(c.Request.Context(), status)
	if err != nil {
		response.InternalError(c, "获取仓库列表失败")
		return
	}

	response.Success(c, repos)
}

// CreateRepo 创建仓库
func (h *GitRepoHandler) CreateRepo(c *gin.Context) {
	var req CreateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	repo, err := h.svc.CreateRepo(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, repo)
}

// GetRepo 获取仓库详情
func (h *GitRepoHandler) GetRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	repo, err := h.svc.GetRepo(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "仓库不存在")
		return
	}

	response.Success(c, repo)
}

// UpdateRepo 更新仓库
func (h *GitRepoHandler) UpdateRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req UpdateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	repo, err := h.svc.UpdateRepo(c.Request.Context(), id, req.DefaultBranch, req.AuthType, req.AuthConfig, req.SyncEnabled, req.SyncInterval)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, repo)
}

// DeleteRepo 删除仓库
func (h *GitRepoHandler) DeleteRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.DeleteRepo(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除失败")
		return
	}

	response.Message(c, "删除成功")
}

// SyncRepo 同步仓库
func (h *GitRepoHandler) SyncRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.SyncRepo(c.Request.Context(), id); err != nil {
		response.InternalError(c, "同步失败: "+err.Error())
		return
	}

	response.Message(c, "同步成功")
}

// ResetStatus 强制重置仓库状态
// 当仓库卡在 syncing 状态时，可以使用此接口强制重置为 pending 或 error
func (h *GitRepoHandler) ResetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 可选参数：目标状态，默认为 pending
	targetStatus := c.DefaultQuery("status", "pending")
	if targetStatus != "pending" && targetStatus != "error" {
		response.BadRequest(c, "目标状态只能是 pending 或 error")
		return
	}

	if err := h.svc.ResetStatus(c.Request.Context(), id, targetStatus); err != nil {
		response.InternalError(c, "重置状态失败: "+err.Error())
		return
	}

	response.Message(c, "状态已重置为 "+targetStatus)
}

// GetSyncLogs 获取仓库同步日志
func (h *GitRepoHandler) GetSyncLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.svc.GetSyncLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.InternalError(c, "获取同步日志失败")
		return
	}

	response.List(c, logs, total, page, pageSize)
}

// ValidateRepo 验证仓库（创建前获取分支列表）
func (h *GitRepoHandler) ValidateRepo(c *gin.Context) {
	var req ValidateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.svc.ValidateRepo(c.Request.Context(), req.URL, req.AuthType, req.AuthConfig)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// GetCommits 获取仓库 commit 历史
func (h *GitRepoHandler) GetCommits(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	commits, err := h.svc.GetCommits(c.Request.Context(), id, limit)
	if err != nil {
		response.InternalError(c, "获取 commit 历史失败: "+err.Error())
		return
	}

	response.Success(c, commits)
}

// GetFiles 获取文件树
func (h *GitRepoHandler) GetFiles(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	// 如果有 path 参数，返回文件内容
	path := c.Query("path")
	if path != "" {
		content, err := h.svc.GetFileContent(c.Request.Context(), id, path)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, map[string]any{"path": path, "content": content})
		return
	}

	// 否则返回文件树
	files, err := h.svc.GetFiles(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, map[string]any{"files": files})
}
