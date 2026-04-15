package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultTeamsThemeColor = "6264A7"

// TeamsProvider Microsoft Teams 通知提供者
type TeamsProvider struct {
	client *http.Client
}

// TeamsConfig Microsoft Teams 配置
type TeamsConfig struct {
	WebhookURL string `json:"webhook_url"`
	ThemeColor string `json:"theme_color,omitempty"`
}

type teamsMessageCard struct {
	Type       string `json:"@type"`
	Context    string `json:"@context"`
	Summary    string `json:"summary"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text"`
	ThemeColor string `json:"themeColor,omitempty"`
}

// NewTeamsProvider 创建 Teams 提供者
func NewTeamsProvider() *TeamsProvider {
	return &TeamsProvider{client: webhookHTTPClient(10)}
}

// Type 返回提供者类型
func (p *TeamsProvider) Type() string {
	return "teams"
}

// Send 发送通知
func (p *TeamsProvider) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	httpReq, err := newJSONRequest(ctx, config.WebhookURL, buildTeamsPayload(req, config))
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	defer resp.Body.Close()
	return buildWebhookSendResult(resp)
}

// Test 测试连接
func (p *TeamsProvider) Test(ctx context.Context, configMap map[string]interface{}) error {
	config, err := p.parseConfig(configMap)
	if err != nil {
		return err
	}
	testReq := &SendRequest{
		Subject: "Auto-Healing Teams 通知测试",
		Body:    "连接测试成功",
	}
	httpReq, err := newJSONRequest(ctx, config.WebhookURL, buildTeamsPayload(testReq, config))
	if err != nil {
		return err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	_, err = buildWebhookSendResult(resp)
	return err
}

func (p *TeamsProvider) parseConfig(configMap map[string]interface{}) (*TeamsConfig, error) {
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var config TeamsConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("Teams webhook_url 不能为空")
	}
	if config.ThemeColor == "" {
		config.ThemeColor = defaultTeamsThemeColor
	}
	return &config, nil
}

func buildTeamsPayload(req *SendRequest, config *TeamsConfig) *teamsMessageCard {
	title := teamsTitle(req.Subject)
	return &teamsMessageCard{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		Summary:    title,
		Title:      title,
		Text:       teamsBody(req),
		ThemeColor: config.ThemeColor,
	}
}

func teamsTitle(subject string) string {
	if subject != "" {
		return subject
	}
	return "Auto-Healing 通知"
}

func teamsBody(req *SendRequest) string {
	if req.Body != "" {
		return req.Body
	}
	return teamsTitle(req.Subject)
}
