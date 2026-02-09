// Package engine 执行引擎
//
// 本包提供执行引擎的统一接口和工厂函数。
// 具体执行器实现在 provider/ 子目录中。
//
// 使用示例:
//
//	executor := engine.NewExecutor("docker")
//	result, err := executor.Execute(ctx, req)
package engine

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/engine/provider/ansible"
)

// Executor 执行器接口
type Executor = ansible.Executor

// ExecuteRequest 执行请求
type ExecuteRequest = ansible.ExecuteRequest

// ExecuteResult 执行结果
type ExecuteResult = ansible.ExecuteResult

// AnsibleStats Ansible 执行统计
type AnsibleStats = ansible.AnsibleStats

// NewExecutor 创建执行器（工厂函数）
func NewExecutor(executorType string) Executor {
	switch executorType {
	case "docker":
		return ansible.NewDockerExecutor()
	case "local":
		return ansible.NewLocalExecutor()
	default:
		return ansible.NewDockerExecutor() // 默认使用 Docker
	}
}

// NewDockerExecutor 创建 Docker 执行器
func NewDockerExecutor() *ansible.DockerExecutor {
	return ansible.NewDockerExecutor()
}

// NewLocalExecutor 创建本地执行器
func NewLocalExecutor() *ansible.LocalExecutor {
	return ansible.NewLocalExecutor()
}

// Execute 便捷执行函数（使用默认执行器）
func Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResult, error) {
	executor := NewExecutor("docker")
	return executor.Execute(ctx, req)
}

// ExecuteWithTimeout 带超时的执行函数
func ExecuteWithTimeout(ctx context.Context, req *ExecuteRequest, timeout time.Duration) (*ExecuteResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return Execute(timeoutCtx, req)
}
