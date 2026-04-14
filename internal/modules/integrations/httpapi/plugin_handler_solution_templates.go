package httpapi

import (
	"errors"

	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *PluginHandler) ListSolutionTemplates(c *gin.Context) {
	templates, err := h.solutionTemplateSvc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "获取解决方案模板列表失败")
		return
	}
	response.Success(c, templates)
}

func (h *PluginHandler) CreateSolutionTemplate(c *gin.Context) {
	var req CreateSolutionTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	template := req.ToModel()
	if err := h.solutionTemplateSvc.Create(c.Request.Context(), template); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, template)
}

func (h *PluginHandler) GetSolutionTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的解决方案模板ID")
		return
	}
	template, err := h.solutionTemplateSvc.Get(c.Request.Context(), id)
	if err != nil {
		respondSolutionTemplateError(c, err)
		return
	}
	response.Success(c, template)
}

func (h *PluginHandler) UpdateSolutionTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的解决方案模板ID")
		return
	}
	template, err := h.solutionTemplateSvc.Get(c.Request.Context(), id)
	if err != nil {
		respondSolutionTemplateError(c, err)
		return
	}
	var req UpdateSolutionTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	req.ApplyTo(template)
	if err := h.solutionTemplateSvc.Update(c.Request.Context(), template); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, template)
}

func (h *PluginHandler) DeleteSolutionTemplate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的解决方案模板ID")
		return
	}
	if err := h.solutionTemplateSvc.Delete(c.Request.Context(), id); err != nil {
		respondSolutionTemplateError(c, err)
		return
	}
	response.Message(c, "删除成功")
}

func respondSolutionTemplateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, integrationrepo.ErrIncidentSolutionTemplateNotFound):
		response.NotFound(c, "解决方案模板不存在")
	default:
		response.BadRequest(c, err.Error())
	}
}
