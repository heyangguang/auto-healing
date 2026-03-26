package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type errorMetadataProvider interface {
	ErrorCode() string
	ErrorDetails() any
}

func ErrorWithMetadata(
	c *gin.Context,
	httpCode int,
	code int,
	msg string,
	errorCode string,
	details any,
) {
	c.JSON(httpCode, Response{
		Code:      code,
		Message:   msg,
		ErrorCode: errorCode,
		Details:   details,
	})
}

func ErrorFromErr(c *gin.Context, httpCode int, code int, err error) {
	errorCode, details := extractErrorMetadata(err)
	ErrorWithMetadata(c, httpCode, code, err.Error(), errorCode, details)
}

func BadRequestFromErr(c *gin.Context, err error) {
	ErrorFromErr(c, http.StatusBadRequest, CodeBadRequest, err)
}

func extractErrorMetadata(err error) (string, any) {
	var provider errorMetadataProvider
	if !errors.As(err, &provider) {
		return "", nil
	}
	return provider.ErrorCode(), provider.ErrorDetails()
}
