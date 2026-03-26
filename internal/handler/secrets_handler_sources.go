package handler

import (
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListSources 获取密钥源列表
func (h *SecretsHandler) ListSources(c *gin.Context) {
	var isDefault *bool
	if raw := c.Query("is_default"); raw != "" {
		value := raw == "true"
		isDefault = &value
	}

	sources, err := h.svc.ListSources(c.Request.Context(), c.Query("type"), c.Query("status"), isDefault)
	if err != nil {
		response.InternalError(c, "获取密钥源列表失败")
		return
	}
	for i := range sources {
		sources[i].Config = maskConfig(sources[i].Config)
	}
	response.Success(c, sources)
}

// CreateSource 创建密钥源
func (h *SecretsHandler) CreateSource(c *gin.Context) {
	var req CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	source, err := h.svc.CreateSource(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	source.Config = maskConfig(source.Config)
	response.Created(c, source)
}

// GetSource 获取密钥源详情
func (h *SecretsHandler) GetSource(c *gin.Context) {
	source, ok := h.loadSourceByID(c)
	if !ok {
		return
	}
	source.Config = maskConfig(source.Config)
	response.Success(c, source)
}

func (h *SecretsHandler) loadSourceByID(c *gin.Context) (*model.SecretsSource, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return nil, false
	}

	source, err := h.svc.GetSource(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "密钥源不存在")
		return nil, false
	}
	return source, true
}

// UpdateSource 更新密钥源
func (h *SecretsHandler) UpdateSource(c *gin.Context) {
	id, ok := parseSecretsSourceID(c)
	if !ok {
		return
	}

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	source, err := h.svc.UpdateSource(c.Request.Context(), id, req.Config, req.IsDefault, req.Priority, req.Status)
	if err != nil {
		writeSourceAdminError(c, err, "更新密钥源失败")
		return
	}
	source.Config = maskConfig(source.Config)
	response.Success(c, source)
}

// DeleteSource 删除密钥源
func (h *SecretsHandler) DeleteSource(c *gin.Context) {
	id, ok := parseSecretsSourceID(c)
	if !ok {
		return
	}

	if err := h.svc.DeleteSource(c.Request.Context(), id); err != nil {
		writeSourceAdminError(c, err, "删除密钥源失败")
		return
	}
	response.Message(c, "删除成功")
}

// TestConnection 测试连接
func (h *SecretsHandler) TestConnection(c *gin.Context) {
	id, ok := parseSecretsSourceID(c)
	if !ok {
		return
	}
	if err := h.svc.TestConnection(c.Request.Context(), id); err != nil {
		response.BadRequest(c, "连接测试失败: "+err.Error())
		return
	}
	response.Message(c, "连接测试成功")
}

// Enable 启用密钥源
func (h *SecretsHandler) Enable(c *gin.Context) {
	id, ok := parseSecretsSourceID(c)
	if !ok {
		return
	}
	if err := h.svc.Enable(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Message(c, "密钥源已启用")
}

// Disable 禁用密钥源
func (h *SecretsHandler) Disable(c *gin.Context) {
	id, ok := parseSecretsSourceID(c)
	if !ok {
		return
	}
	if err := h.svc.Disable(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Message(c, "密钥源已禁用")
}

// GetStats 获取密钥源统计信息
func (h *SecretsHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "SECRETS", "获取密钥源统计信息失败", err)
		return
	}
	response.Success(c, stats)
}

func parseSecretsSourceID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return uuid.Nil, false
	}
	return id, true
}
