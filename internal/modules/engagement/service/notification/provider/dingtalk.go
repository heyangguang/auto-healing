package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DingTalkProvider 钉钉通知提供者
type DingTalkProvider struct {
	client *http.Client
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	WebhookURL string   `json:"webhook_url"`
	Secret     string   `json:"secret"`     // 加签密钥（可选）
	AtMobiles  []string `json:"at_mobiles"` // @ 指定手机号
	AtAll      bool     `json:"at_all"`     // @ 所有人
}

// DingTalkMessage 钉钉消息格式
type DingTalkMessage struct {
	MsgType  string            `json:"msgtype"`
	Markdown *DingTalkMarkdown `json:"markdown,omitempty"`
	Text     *DingTalkText     `json:"text,omitempty"`
	At       *DingTalkAt       `json:"at,omitempty"`
}

type DingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type DingTalkText struct {
	Content string `json:"content"`
}

type DingTalkAt struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	IsAtAll   bool     `json:"isAtAll"`
}

// NewDingTalkProvider 创建钉钉提供者
func NewDingTalkProvider() *DingTalkProvider {
	return &DingTalkProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Type 返回提供者类型
func (p *DingTalkProvider) Type() string {
	return "dingtalk"
}

// Send 发送通知
func (p *DingTalkProvider) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	// 构建消息
	msg := p.buildMessage(req, config)
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	// 构建签名 URL
	requestURL, err := p.buildSignedURL(config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	// 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	defer resp.Body.Close()

	// 解析响应
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	errcode, _ := result["errcode"].(float64)
	if errcode == 0 {
		return &SendResponse{
			Success:      true,
			ResponseData: result,
		}, nil
	}

	errmsg, _ := result["errmsg"].(string)
	errMsg := fmt.Sprintf("钉钉发送失败: %s (errcode: %.0f)", errmsg, errcode)
	return &SendResponse{Success: false, ErrorMessage: errMsg}, fmt.Errorf("%s", errMsg)
}

// Test 测试连接
func (p *DingTalkProvider) Test(ctx context.Context, configMap map[string]interface{}) error {
	config, err := p.parseConfig(configMap)
	if err != nil {
		return err
	}

	// 发送测试消息
	testMsg := &DingTalkMessage{
		MsgType: "text",
		Text: &DingTalkText{
			Content: "Auto-Healing 通知测试 - " + time.Now().Format("2006-01-02 15:04:05"),
		},
	}
	jsonData, _ := json.Marshal(testMsg)

	requestURL, err := p.buildSignedURL(config)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	errcode, _ := result["errcode"].(float64)
	if errcode == 0 {
		return nil
	}

	errmsg, _ := result["errmsg"].(string)
	return fmt.Errorf("测试失败: %s (errcode: %.0f)", errmsg, errcode)
}

// buildMessage 构建钉钉消息
func (p *DingTalkProvider) buildMessage(req *SendRequest, config *DingTalkConfig) *DingTalkMessage {
	msg := &DingTalkMessage{
		At: &DingTalkAt{
			AtMobiles: config.AtMobiles,
			IsAtAll:   config.AtAll,
		},
	}

	// 根据格式选择消息类型
	if req.Format == "markdown" {
		msg.MsgType = "markdown"
		title := req.Subject
		if title == "" {
			title = "Auto-Healing 通知"
		}
		msg.Markdown = &DingTalkMarkdown{
			Title: title,
			Text:  req.Body,
		}
	} else {
		msg.MsgType = "text"
		content := req.Body
		if req.Subject != "" {
			content = req.Subject + "\n\n" + content
		}
		msg.Text = &DingTalkText{
			Content: content,
		}
	}

	return msg
}

// buildSignedURL 构建签名 URL
func (p *DingTalkProvider) buildSignedURL(config *DingTalkConfig) (string, error) {
	if config.Secret == "" {
		return config.WebhookURL, nil
	}

	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, config.Secret)

	h := hmac.New(sha256.New, []byte(config.Secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 拼接签名参数
	parsedURL, err := url.Parse(config.WebhookURL)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set("timestamp", fmt.Sprintf("%d", timestamp))
	query.Set("sign", sign)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// parseConfig 解析配置
func (p *DingTalkProvider) parseConfig(configMap map[string]interface{}) (*DingTalkConfig, error) {
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var config DingTalkConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}

	if config.WebhookURL == "" {
		return nil, fmt.Errorf("钉钉 webhook_url 不能为空")
	}

	return &config, nil
}
