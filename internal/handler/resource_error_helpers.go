package handler

import (
	"errors"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type resourceErrorMode int

const (
	resourceErrorModeInternal resourceErrorMode = iota
	resourceErrorModeBadRequest
)

func respondResourceError(c *gin.Context, sub, publicMsg, notFoundMsg string, notFoundErr error, mode resourceErrorMode, err error) {
	if errors.Is(err, notFoundErr) {
		response.NotFound(c, notFoundMsg)
		return
	}
	if mode == resourceErrorModeBadRequest {
		response.BadRequest(c, err.Error())
		return
	}
	respondInternalError(c, sub, publicMsg, err)
}
