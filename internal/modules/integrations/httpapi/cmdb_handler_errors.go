package httpapi

import (
	"errors"

	pluginservice "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

const cmdbNotFoundMessage = "配置项不存在"

func respondCMDBItemError(c *gin.Context, publicMsg string, err error) {
	if errors.Is(err, repository.ErrCMDBItemNotFound) {
		response.NotFound(c, cmdbNotFoundMessage)
		return
	}
	respondInternalError(c, "CMDB", publicMsg, err)
}

func respondCMDBMaintenanceError(c *gin.Context, publicMsg string, err error) {
	switch {
	case errors.Is(err, repository.ErrCMDBItemNotFound):
		response.NotFound(c, cmdbNotFoundMessage)
	case errors.Is(err, pluginservice.ErrCMDBOfflineMaintenanceForbidden):
		response.BadRequest(c, err.Error())
	default:
		respondInternalError(c, "CMDB", publicMsg, err)
	}
}

func cmdbLookupFailureMessage(err error) string {
	if errors.Is(err, repository.ErrCMDBItemNotFound) {
		return cmdbNotFoundMessage
	}
	return "查询配置项失败: " + err.Error()
}
