package httpx

import (
	"errors"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ResourceErrorMode int

const (
	ResourceErrorModeInternal ResourceErrorMode = iota
	ResourceErrorModeBadRequest
)

func RespondResourceError(c *gin.Context, sub, publicMsg, notFoundMsg string, notFoundErr error, mode ResourceErrorMode, err error) {
	if errors.Is(err, notFoundErr) {
		response.NotFound(c, notFoundMsg)
		return
	}
	if mode == ResourceErrorModeBadRequest {
		response.BadRequest(c, err.Error())
		return
	}
	RespondInternalError(c, sub, publicMsg, err)
}
