package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SlackProvider Slack 通知提供者
type SlackProvider struct {
	client *http.Client
}

// SlackConfig Slack 配置
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel,omitempty"`
	Username   string `json:"username,omitempty"`
	IconEmoji  string `json:"icon_emoji,omitempty"`
	IconURL    string `json:"icon_url,omitempty"`
}

type slackMessage struct {
	Text      string `json:"text"`
	Mrkdwn    bool   `json:"mrkdwn,omitempty"`
	Channel   string `json:"channel,omitempty"`
	Username  string `json:"username,omitempty"`
	IconEmoji string `json:"icon_emoji,omitempty"`
	IconURL   string `json:"icon_url,omitempty"`
}

// NewSlackProvider 创建 Slack 提供者
func NewSlackProvider() *SlackProvider {
	return &SlackProvider{client: webhookHTTPClient(10)}
}

// Type 返回提供者类型
func (p *SlackProvider) Type() string {
	return "slack"
}

// Send 发送通知
func (p *SlackProvider) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	httpReq, err := newJSONRequest(ctx, config.WebhookURL, buildSlackPayload(req, config))
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
func (p *SlackProvider) Test(ctx context.Context, configMap map[string]interface{}) error {
	config, err := p.parseConfig(configMap)
	if err != nil {
		return err
	}
	httpReq, err := newJSONRequest(ctx, config.WebhookURL, buildSlackTestPayload(config))
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

func (p *SlackProvider) parseConfig(configMap map[string]interface{}) (*SlackConfig, error) {
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var config SlackConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("Slack webhook_url 不能为空")
	}
	return &config, nil
}

func buildSlackPayload(req *SendRequest, config *SlackConfig) *slackMessage {
	return &slackMessage{
		Text:      slackText(req),
		Mrkdwn:    req.Format == "markdown",
		Channel:   config.Channel,
		Username:  config.Username,
		IconEmoji: config.IconEmoji,
		IconURL:   config.IconURL,
	}
}

func buildSlackTestPayload(config *SlackConfig) *slackMessage {
	return &slackMessage{
		Text:      "Auto-Healing Slack 通知测试",
		Channel:   config.Channel,
		Username:  config.Username,
		IconEmoji: config.IconEmoji,
		IconURL:   config.IconURL,
	}
}

func slackText(req *SendRequest) string {
	if req.Format == "markdown" {
		return markdownContent(req.Subject, req.Body)
	}
	return textContent(req.Subject, req.Body)
}
