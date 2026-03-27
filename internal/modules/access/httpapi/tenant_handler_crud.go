package httpapi

import (
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListTenants 租户列表
func (h *TenantHandler) ListTenants(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	tenants, total, err := h.repo.List(
		c.Request.Context(),
		c.Query("keyword"),
		GetStringFilter(c, "name"),
		GetStringFilter(c, "code"),
		c.Query("status"),
		page,
		pageSize,
	)
	if err != nil {
		response.InternalError(c, "查询租户列表失败")
		return
	}
	response.List(c, tenants, total, page, pageSize)
}

// GetTenant 获取租户详情
func (h *TenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondTenantLookupError(c, err)
		return
	}
	response.Success(c, tenant)
}

// CreateTenant 创建租户
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：name 和 code 为必填")
		return
	}
	existing, err := h.repo.GetByCode(c.Request.Context(), req.Code)
	if err != nil && !errors.Is(err, repository.ErrTenantNotFound) {
		respondInternalError(c, "TENANT", "检查租户编码失败", err)
		return
	}
	if existing != nil {
		response.Conflict(c, "租户编码已存在: "+req.Code)
		return
	}

	tenant := &model.Tenant{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Icon:        req.Icon,
		Status:      model.TenantStatusActive,
	}
	if err := h.repo.Create(c.Request.Context(), tenant); err != nil {
		response.InternalError(c, "创建租户失败")
		return
	}
	response.Created(c, tenant)
}

// UpdateTenant 更新租户
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondTenantLookupError(c, err)
		return
	}

	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := applyTenantUpdate(tenant, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.repo.Update(c.Request.Context(), tenant); err != nil {
		response.InternalError(c, "更新租户失败")
		return
	}
	response.Success(c, tenant)
}

// DeleteTenant 删除租户
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的租户 ID")
		return
	}

	tenant, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondTenantLookupError(c, err)
		return
	}
	if tenant.Code == "default" {
		response.BadRequest(c, "默认租户不能被删除")
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除租户失败")
		return
	}
	response.Message(c, "租户已删除")
}

func applyTenantUpdate(tenant *model.Tenant, req *updateTenantRequest) error {
	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.Description != "" {
		tenant.Description = req.Description
	}
	if req.Icon != "" {
		tenant.Icon = req.Icon
	}

	if tenant.Code == "default" {
		if req.Status == model.TenantStatusDisabled {
			return fmt.Errorf("默认租户不能被禁用")
		}
		return nil
	}
	if req.Status != "" {
		tenant.Status = req.Status
	}
	return nil
}

func respondTenantLookupError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrTenantNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "租户不存在")
		return
	}
	respondInternalError(c, "TENANT", "查询租户失败", err)
}
