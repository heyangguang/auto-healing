package httpapi

import (
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func FormatValidationError(err error) string {
	return platformhttp.FormatValidationError(err)
}

func respondInternalError(c *gin.Context, sub, publicMsg string, err error) {
	platformhttp.RespondInternalError(c, sub, publicMsg, err)
}
