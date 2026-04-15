package provider

import (
	"net/url"
	"strings"
)

const weComWebhookHost = "qyapi.weixin.qq.com"

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

type weComMessage struct {
	MsgType  string         `json:"msgtype"`
	Markdown *weComMarkdown `json:"markdown,omitempty"`
	Text     *weComText     `json:"text,omitempty"`
}

type weComMarkdown struct {
	Content string `json:"content"`
}

type weComText struct {
	Content string `json:"content"`
}

func isWeComWebhook(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), weComWebhookHost) &&
		strings.HasPrefix(parsed.Path, "/cgi-bin/webhook/send")
}

func (p *DingTalkProvider) buildPayload(
	req *SendRequest,
	config *DingTalkConfig,
) interface{} {
	if isWeComWebhook(config.WebhookURL) {
		return buildWeComPayload(req, &WeComConfig{})
	}
	return p.buildMessage(req, config)
}

func (p *DingTalkProvider) buildMessage(
	req *SendRequest,
	config *DingTalkConfig,
) *DingTalkMessage {
	msg := &DingTalkMessage{
		At: &DingTalkAt{
			AtMobiles: config.AtMobiles,
			IsAtAll:   config.AtAll,
		},
	}
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
		return msg
	}

	msg.MsgType = "text"
	msg.Text = &DingTalkText{Content: textContent(req.Subject, req.Body)}
	return msg
}

func buildWeComPayload(req *SendRequest, _ *WeComConfig) *weComMessage {
	if req.Format == "markdown" {
		return &weComMessage{
			MsgType: "markdown",
			Markdown: &weComMarkdown{
				Content: markdownContent(req.Subject, req.Body),
			},
		}
	}
	return &weComMessage{
		MsgType: "text",
		Text: &weComText{
			Content: textContent(req.Subject, req.Body),
		},
	}
}

func markdownContent(subject, body string) string {
	if body != "" {
		return body
	}
	return subject
}

func textContent(subject, body string) string {
	if subject == "" {
		return body
	}
	if body == "" {
		return subject
	}
	return subject + "\n\n" + body
}
