package handler

import (
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type resourceErrorMode = platformhttp.ResourceErrorMode

const (
	resourceErrorModeInternal   = platformhttp.ResourceErrorModeInternal
	resourceErrorModeBadRequest = platformhttp.ResourceErrorModeBadRequest
)

func respondResourceError(c *gin.Context, sub, publicMsg, notFoundMsg string, notFoundErr error, mode resourceErrorMode, err error) {
	platformhttp.RespondResourceError(c, sub, publicMsg, notFoundMsg, notFoundErr, mode, err)
}
