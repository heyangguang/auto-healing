package httpapi

import (
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SyncRepo 同步仓库
func (h *GitRepoHandler) SyncRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.SyncRepo(c.Request.Context(), id); err != nil {
		respondResourceError(c, "GIT", "同步仓库失败", "仓库不存在", integrationrepo.ErrGitRepositoryNotFound, resourceErrorModeInternal, err)
		return
	}
	response.Message(c, "同步成功")
}

// ResetStatus 强制重置仓库状态
func (h *GitRepoHandler) ResetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	targetStatus := c.DefaultQuery("status", "pending")
	if targetStatus != "pending" && targetStatus != "error" {
		response.BadRequest(c, "目标状态只能是 pending 或 error")
		return
	}

	if err := h.svc.ResetStatus(c.Request.Context(), id, targetStatus); err != nil {
		respondResourceError(c, "GIT", "重置仓库状态失败", "仓库不存在", integrationrepo.ErrGitRepositoryNotFound, resourceErrorModeInternal, err)
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

	page, pageSize := parsePagination(c, 20)
	logs, total, err := h.svc.GetSyncLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		response.InternalError(c, "获取同步日志失败")
		return
	}
	response.List(c, logs, total, page, pageSize)
}

// GetCommits 获取仓库 commit 历史
func (h *GitRepoHandler) GetCommits(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	limit := parsePositiveIntQuery(c, "limit", 10, 200)
	commits, err := h.svc.GetCommits(c.Request.Context(), id, limit)
	if err != nil {
		respondResourceError(c, "GIT", "获取 commit 历史失败", "仓库不存在", integrationrepo.ErrGitRepositoryNotFound, resourceErrorModeInternal, err)
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

	path := c.Query("path")
	if path != "" {
		content, err := h.svc.GetFileContent(c.Request.Context(), id, path)
		if err != nil {
			respondResourceError(c, "GIT", "获取文件内容失败", "仓库不存在", integrationrepo.ErrGitRepositoryNotFound, resourceErrorModeBadRequest, err)
			return
		}
		response.Success(c, map[string]any{"path": path, "content": content})
		return
	}

	files, err := h.svc.GetFiles(c.Request.Context(), id)
	if err != nil {
		respondResourceError(c, "GIT", "获取文件树失败", "仓库不存在", integrationrepo.ErrGitRepositoryNotFound, resourceErrorModeBadRequest, err)
		return
	}
	response.Success(c, map[string]any{"files": files})
}
