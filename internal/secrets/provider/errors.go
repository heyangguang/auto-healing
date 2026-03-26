// Package provider 密钥提供者实现
package provider

import (
	"errors"
	"fmt"
	"net/http"
)

// 共享错误定义
var (
	ErrSecretNotFound          = errors.New("密钥未找到")
	ErrConnectionFailed        = errors.New("连接失败")
	ErrProviderAuthFailed      = errors.New("提供方认证失败")
	ErrProviderRequestFailed   = errors.New("提供方请求失败")
	ErrProviderInvalidConfig   = errors.New("提供方配置无效")
	ErrProviderInvalidResponse = errors.New("提供方响应无效")
)

type providerError struct {
	kind  error
	msg   string
	cause error
}

func (e *providerError) Error() string {
	return e.msg
}

func (e *providerError) Unwrap() error {
	if e.cause == nil {
		return e.kind
	}
	return errors.Join(e.kind, e.cause)
}

func newProviderError(kind error, msg string, cause error) error {
	return &providerError{kind: kind, msg: msg, cause: cause}
}

func newConfigError(msg string) error {
	return newProviderError(ErrProviderInvalidConfig, msg, nil)
}

func newAuthError(msg string, cause error) error {
	return newProviderError(ErrProviderAuthFailed, msg, cause)
}

func newConnectionError(msg string, cause error) error {
	return newProviderError(ErrConnectionFailed, msg, cause)
}

func newRequestError(msg string, cause error) error {
	return newProviderError(ErrProviderRequestFailed, msg, cause)
}

func newInvalidResponseError(msg string, cause error) error {
	return newProviderError(ErrProviderInvalidResponse, msg, cause)
}

func newHTTPStatusError(system string, statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return newAuthError(fmt.Sprintf("%s 认证失败 (HTTP %d)", system, statusCode), nil)
	default:
		return newRequestError(fmt.Sprintf("%s 返回错误 (HTTP %d)", system, statusCode), nil)
	}
}
