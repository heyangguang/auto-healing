package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const weComProviderLabel = "企业微信"

// WeComProvider 企业微信通知提供者
type WeComProvider struct {
	client *http.Client
}

// WeComConfig 企业微信配置
type WeComConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// NewWeComProvider 创建企业微信提供者
func NewWeComProvider() *WeComProvider {
	return &WeComProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Type 返回提供者类型
func (p *WeComProvider) Type() string {
	return "wecom"
}

// Send 发送通知
func (p *WeComProvider) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	httpReq, err := newJSONRequest(ctx, config.WebhookURL, buildWeComPayload(req, config))
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	defer resp.Body.Close()
	return buildRobotSendResponse(resp, weComProviderLabel)
}

// Test 测试连接
func (p *WeComProvider) Test(ctx context.Context, configMap map[string]interface{}) error {
	config, err := p.parseConfig(configMap)
	if err != nil {
		return err
	}
	testReq := &SendRequest{
		Body: "Auto-Healing 通知测试 - " + time.Now().Format("2006-01-02 15:04:05"),
	}
	httpReq, err := newJSONRequest(ctx, config.WebhookURL, buildWeComPayload(testReq, config))
	if err != nil {
		return err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	return validateRobotResponse(resp, weComProviderLabel)
}

func (p *WeComProvider) parseConfig(configMap map[string]interface{}) (*WeComConfig, error) {
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var config WeComConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("企业微信 webhook_url 不能为空")
	}
	return &config, nil
}
