package httpapi

import (
	"errors"

	authService "github.com/company/auto-healing/internal/modules/access/service/auth"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

func respondChangePasswordError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authService.ErrPasswordMismatch):
		response.BadRequest(c, ToBusinessError(err))
	case errors.Is(err, repository.ErrUserNotFound):
		response.NotFound(c, "用户不存在")
	default:
		respondInternalError(c, "AUTH", "修改密码失败", err)
	}
}

func respondUpdateProfileError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrUserNotFound) {
		response.NotFound(c, "用户不存在")
		return
	}
	respondInternalError(c, "AUTH", "更新个人信息失败", err)
}

func respondProfileAuditQueryError(c *gin.Context, publicMsg string, err error) {
	switch {
	case errors.Is(err, repository.ErrTenantContextRequired):
		response.Forbidden(c, "租户上下文缺失或无效")
	case errors.Is(err, errAuthTenantNotFound):
		response.Forbidden(c, errAuthTenantNotFound.Error())
	case errors.Is(err, errAuthTenantDisabled):
		response.Forbidden(c, errAuthTenantDisabled.Error())
	case errors.Is(err, errAuthTenantAccess):
		response.Forbidden(c, errAuthTenantAccess.Error())
	default:
		respondInternalError(c, "AUTH", publicMsg, err)
	}
}
