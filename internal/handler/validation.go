package handler

import (
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
)

// FormatValidationError 格式化验证错误，提取具体字段名称
func FormatValidationError(err error) string {
	return platformhttp.FormatValidationError(err)
}

// ToBusinessError 将技术错误转换为业务友好的错误信息
func ToBusinessError(err error) string {
	return platformhttp.ToBusinessError(err)
}
