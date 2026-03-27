package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
	Details   any    `json:"details,omitempty"`
	Data      any    `json:"data,omitempty"`
	Total     *int64 `json:"total,omitempty"`
	Page      *int   `json:"page,omitempty"`
	PageSize  *int   `json:"page_size,omitempty"`
}

// 错误码定义
const (
	CodeSuccess      = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeForbidden    = 40300
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeInternal     = 50000
)

// Success 成功响应（单对象）
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// Created 创建成功响应 (201)
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// List 列表响应
func List(c *gin.Context, data any, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:     CodeSuccess,
		Message:  "success",
		Data:     data,
		Total:    &total,
		Page:     &page,
		PageSize: &pageSize,
	})
}

// Collection 集合响应（无分页，但有 total）
func Collection(c *gin.Context, data any, total int64) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Total:   &total,
	})
}

// Message 纯消息响应（如删除成功）
func Message(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: msg,
	})
}

// NoContent 无内容响应 (用于向后兼容，但推荐使用 Message)
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error 错误响应
func Error(c *gin.Context, httpCode int, code int, msg string) {
	c.JSON(httpCode, Response{
		Code:    code,
		Message: msg,
	})
}

// BadRequest 400 错误
func BadRequest(c *gin.Context, msg string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, msg)
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, msg string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, msg)
}

// Forbidden 403 错误
func Forbidden(c *gin.Context, msg string) {
	Error(c, http.StatusForbidden, CodeForbidden, msg)
}

// NotFound 404 错误
func NotFound(c *gin.Context, msg string) {
	Error(c, http.StatusNotFound, CodeNotFound, msg)
}

// Conflict 409 冲突错误
func Conflict(c *gin.Context, msg string) {
	Error(c, http.StatusConflict, CodeConflict, msg)
}

// InternalError 500 错误
func InternalError(c *gin.Context, msg string) {
	Error(c, http.StatusInternalServerError, CodeInternal, msg)
}
