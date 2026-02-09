// Package provider 密钥提供者实现
package provider

import (
	"errors"
)

// 共享错误定义
var (
	ErrSecretNotFound   = errors.New("密钥未找到")
	ErrConnectionFailed = errors.New("连接失败")
)
