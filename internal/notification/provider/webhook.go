package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookProvider Webhook 通知提供者
type WebhookProvider struct {
	client *http.Client
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Username       string            `json:"username,omitempty"` // Basic Auth 用户名
	Password       string            `json:"password,omitempty"` // Basic Auth 密码
}

// NewWebhookProvider 创建 Webhook 提供者
func NewWebhookProvider() *WebhookProvider {
	return &WebhookProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Type 返回提供者类型
func (p *WebhookProvider) Type() string {
	return "webhook"
}

// Send 发送通知
func (p *WebhookProvider) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	// 设置超时
	if config.TimeoutSeconds > 0 {
		p.client.Timeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	// 构建请求体
	payload := map[string]interface{}{
		"subject":    req.Subject,
		"body":       req.Body,
		"format":     req.Format,
		"recipients": req.Recipients,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	// 创建请求
	method := config.Method
	if method == "" {
		method = "POST"
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, config.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	// 设置头部
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range config.Headers {
		httpReq.Header.Set(k, v)
	}

	// Basic Auth
	if config.Username != "" {
		httpReq.SetBasicAuth(config.Username, config.Password)
	}

	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &SendResponse{
			Success:      true,
			ResponseData: map[string]interface{}{"status_code": resp.StatusCode, "body": string(body)},
		}, nil
	}

	errMsg := fmt.Sprintf("webhook 返回错误状态码: %d, body: %s", resp.StatusCode, string(body))
	return &SendResponse{Success: false, ErrorMessage: errMsg}, fmt.Errorf("%s", errMsg)
}

// Test 测试连接
func (p *WebhookProvider) Test(ctx context.Context, configMap map[string]interface{}) error {
	config, err := p.parseConfig(configMap)
	if err != nil {
		return err
	}

	// 发送测试请求
	testPayload := map[string]interface{}{
		"test":      true,
		"message":   "Auto-Healing 通知测试",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	jsonData, _ := json.Marshal(testPayload)

	method := config.Method
	if method == "" {
		method = "POST"
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, config.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range config.Headers {
		httpReq.Header.Set(k, v)
	}

	// Basic Auth
	if config.Username != "" {
		httpReq.SetBasicAuth(config.Username, config.Password)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("测试失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
}

// parseConfig 解析配置
func (p *WebhookProvider) parseConfig(configMap map[string]interface{}) (*WebhookConfig, error) {
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var config WebhookConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}

	if config.URL == "" {
		return nil, fmt.Errorf("webhook url 不能为空")
	}

	return &config, nil
}
