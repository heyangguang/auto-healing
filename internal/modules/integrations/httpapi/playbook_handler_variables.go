package httpapi

import (
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetFiles 获取扫描过的文件列表
func (h *PlaybookHandler) GetFiles(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	files, err := h.svc.GetFiles(c.Request.Context(), id)
	if err != nil {
		respondResourceError(c, "PLAYBOOK", "获取 Playbook 文件失败", "Playbook不存在", integrationrepo.ErrPlaybookNotFound, resourceErrorModeBadRequest, err)
		return
	}
	response.Success(c, files)
}

// ScanVariables 扫描变量
func (h *PlaybookHandler) ScanVariables(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	log, err := h.svc.ScanVariables(c.Request.Context(), id, "manual")
	if err != nil {
		respondResourceError(c, "PLAYBOOK", "扫描变量失败", "Playbook不存在", integrationrepo.ErrPlaybookNotFound, resourceErrorModeBadRequest, err)
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
		respondResourceError(c, "PLAYBOOK", "更新变量失败", "Playbook不存在", integrationrepo.ErrPlaybookNotFound, resourceErrorModeInternal, err)
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

	page, pageSize := parsePagination(c, 20)
	logs, total, err := h.svc.GetScanLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		respondResourceError(c, "PLAYBOOK", "获取扫描日志失败", "Playbook不存在", integrationrepo.ErrPlaybookNotFound, resourceErrorModeInternal, err)
		return
	}
	response.List(c, logs, total, page, pageSize)
}
