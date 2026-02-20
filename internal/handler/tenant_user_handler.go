package handler

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	authService "github.com/company/auto-healing/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantUserHandler 租户级用户管理处理器
type TenantUserHandler struct {
	authSvc *authService.Service
}

// NewTenantUserHandler 创建租户级用户处理器
func NewTenantUserHandler(authSvc *authService.Service) *TenantUserHandler {
	return &TenantUserHandler{
		authSvc: authSvc,
	}
}

// CreateTenantUser 租户级创建用户
// 从 TenantMiddleware 获取当前租户 ID，自动将用户关联到当前租户
func (h *TenantUserHandler) CreateTenantUser(c *gin.Context) {
	var req authService.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	// 从 TenantMiddleware 获取当前租户 ID
	tenantIDStr, exists := c.Get(middleware.TenantIDKey)
	if !exists {
		response.InternalError(c, "无法获取租户上下文")
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		response.InternalError(c, "租户ID格式错误")
		return
	}

	// 自动关联到当前租户
	req.TenantID = &tenantID

	user, err := h.authSvc.Register(c.Request.Context(), &req)
	if err != nil {
		response.BadRequest(c, ToBusinessError(err))
		return
	}

	response.Created(c, user)
}
