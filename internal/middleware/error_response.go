package middleware

import (
	"net/http"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func abortWithErrorCode(c *gin.Context, httpCode int, code int, msg, errorCode string, details any) {
	response.ErrorWithMetadata(c, httpCode, code, msg, errorCode, details)
	c.Abort()
}

func abortUnauthorized(c *gin.Context, msg, errorCode string) {
	abortWithErrorCode(c, http.StatusUnauthorized, response.CodeUnauthorized, msg, errorCode, nil)
}

func abortBadRequest(c *gin.Context, msg, errorCode string) {
	abortWithErrorCode(c, http.StatusBadRequest, response.CodeBadRequest, msg, errorCode, nil)
}

func abortForbidden(c *gin.Context, msg, errorCode string) {
	abortWithErrorCode(c, http.StatusForbidden, response.CodeForbidden, msg, errorCode, nil)
}

func abortInternalError(c *gin.Context, msg, errorCode string) {
	abortWithErrorCode(c, http.StatusInternalServerError, response.CodeInternal, msg, errorCode, nil)
}
