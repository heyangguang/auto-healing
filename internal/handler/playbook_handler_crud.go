package handler

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
	page, pageSize := parsePagination(c, 20)
	opts := buildPlaybookListOptions(c, page, pageSize)

	playbooks, total, err := h.svc.ListWithOptions(c.Request.Context(), opts)
	if err != nil {
		respondInternalError(c, "PLAYBOOK", "获取 Playbook 列表失败", err)
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
		respondResourceError(c, "PLAYBOOK", "获取 Playbook 详情失败", "Playbook不存在", repository.ErrPlaybookNotFound, resourceErrorModeInternal, err)
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

	if err := h.svc.Update(c.Request.Context(), id, req.ToUpdateInput()); err != nil {
		respondResourceError(c, "PLAYBOOK", "更新 Playbook 失败", "Playbook不存在", repository.ErrPlaybookNotFound, resourceErrorModeInternal, err)
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
		respondResourceError(c, "PLAYBOOK", "删除 Playbook 失败", "Playbook不存在", repository.ErrPlaybookNotFound, resourceErrorModeInternal, err)
		return
	}
	response.Message(c, "删除成功")
}

// SetReady 设置为 ready 状态
func (h *PlaybookHandler) SetReady(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.SetReady(c.Request.Context(), id); err != nil {
		respondResourceError(c, "PLAYBOOK", "设置 Playbook ready 失败", "Playbook不存在", repository.ErrPlaybookNotFound, resourceErrorModeBadRequest, err)
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
		respondResourceError(c, "PLAYBOOK", "下线 Playbook 失败", "Playbook不存在", repository.ErrPlaybookNotFound, resourceErrorModeBadRequest, err)
		return
	}
	response.Message(c, "已下线")
}

// GetStats 获取 Playbook 统计信息
func (h *PlaybookHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "PLAYBOOK", "获取统计信息失败", err)
		return
	}
	response.Success(c, stats)
}
