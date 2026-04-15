package provider

import (
	"context"
)

// Provider 通知提供者接口
type Provider interface {
	// Type 返回提供者类型
	Type() string

	// Send 发送通知
	Send(ctx context.Context, req *SendRequest) (*SendResponse, error)

	// Test 测试连接
	Test(ctx context.Context, config map[string]interface{}) error
}

// SendRequest 发送请求
type SendRequest struct {
	Recipients []string               // 接收者列表
	Subject    string                 // 主题（可选，仅邮件等需要）
	Body       string                 // 正文
	Format     string                 // text, markdown, html
	Config     map[string]interface{} // 渠道配置
}

// SendResponse 发送响应
type SendResponse struct {
	Success           bool                   // 是否成功
	ExternalMessageID string                 // 外部消息 ID
	ResponseData      map[string]interface{} // 原始响应数据
	ErrorMessage      string                 // 错误信息
}

// Registry 提供者注册表
type Registry struct {
	providers map[string]Provider
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
	}
	// 注册默认提供者
	r.Register(NewWebhookProvider())
	r.Register(NewDingTalkProvider())
	r.Register(NewWeComProvider())
	r.Register(NewSlackProvider())
	r.Register(NewTeamsProvider())
	r.Register(NewEmailProvider())
	return r
}

// Register 注册提供者
func (r *Registry) Register(p Provider) {
	r.providers[p.Type()] = p
}

// Get 获取提供者
func (r *Registry) Get(providerType string) (Provider, bool) {
	p, ok := r.providers[providerType]
	return p, ok
}

// List 列出所有提供者类型
func (r *Registry) List() []string {
	types := make([]string, 0, len(r.providers))
	for t := range r.providers {
		types = append(types, t)
	}
	return types
}
