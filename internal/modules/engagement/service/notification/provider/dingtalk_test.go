package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDingTalkBuildMessageSupportsMarkdownAndText(t *testing.T) {
	provider := NewDingTalkProvider()
	config := &DingTalkConfig{AtMobiles: []string{"13800000000"}, AtAll: true}

	markdown := provider.buildMessage(&SendRequest{
		Subject: "告警",
		Body:    "## body",
		Format:  "markdown",
	}, config)
	if markdown.MsgType != "markdown" || markdown.Markdown == nil {
		t.Fatalf("markdown message = %#v, want markdown payload", markdown)
	}
	if markdown.Markdown.Title != "告警" {
		t.Fatalf("markdown title = %q, want %q", markdown.Markdown.Title, "告警")
	}

	text := provider.buildMessage(&SendRequest{
		Subject: "主题",
		Body:    "正文",
	}, config)
	if text.MsgType != "text" || text.Text == nil {
		t.Fatalf("text message = %#v, want text payload", text)
	}
	if !strings.Contains(text.Text.Content, "主题") || !strings.Contains(text.Text.Content, "正文") {
		t.Fatalf("text content = %q, want subject and body", text.Text.Content)
	}
	if text.At == nil || !text.At.IsAtAll || len(text.At.AtMobiles) != 1 {
		t.Fatalf("text at payload = %#v, want copied at config", text.At)
	}
}

func TestDingTalkBuildPayloadUsesWeComMarkdownShape(t *testing.T) {
	provider := NewDingTalkProvider()
	payload := provider.buildPayload(&SendRequest{
		Subject: "标题",
		Body:    "# 正文",
		Format:  "markdown",
	}, &DingTalkConfig{
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	})

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if raw["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %#v, want markdown", raw["msgtype"])
	}

	markdown, ok := raw["markdown"].(map[string]interface{})
	if !ok {
		t.Fatalf("markdown payload = %#v, want object", raw["markdown"])
	}
	if markdown["content"] != "# 正文" {
		t.Fatalf("markdown content = %#v, want body", markdown["content"])
	}
	if _, exists := markdown["text"]; exists {
		t.Fatalf("markdown payload = %#v, want no text field", markdown)
	}
	if _, exists := markdown["title"]; exists {
		t.Fatalf("markdown payload = %#v, want no title field", markdown)
	}
}

func TestDingTalkBuildPayloadUsesWeComTextShape(t *testing.T) {
	provider := NewDingTalkProvider()
	payload := provider.buildPayload(&SendRequest{
		Subject: "主题",
		Body:    "正文",
	}, &DingTalkConfig{
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	})

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	text, ok := raw["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("text payload = %#v, want object", raw["text"])
	}
	if text["content"] != "主题\n\n正文" {
		t.Fatalf("text content = %#v, want merged subject and body", text["content"])
	}
}

func TestDingTalkBuildSignedURL(t *testing.T) {
	provider := NewDingTalkProvider()

	plainURL, err := provider.buildSignedURL(&DingTalkConfig{WebhookURL: "https://example.com/hook"})
	if err != nil {
		t.Fatalf("buildSignedURL() error = %v", err)
	}
	if plainURL != "https://example.com/hook" {
		t.Fatalf("plain buildSignedURL() = %q", plainURL)
	}

	signedURL, err := provider.buildSignedURL(&DingTalkConfig{
		WebhookURL: "https://example.com/hook?foo=bar",
		Secret:     "top-secret",
	})
	if err != nil {
		t.Fatalf("buildSignedURL() with secret error = %v", err)
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Query().Get("foo") != "bar" {
		t.Fatalf("existing query lost: %q", parsed.RawQuery)
	}
	if parsed.Query().Get("timestamp") == "" || parsed.Query().Get("sign") == "" {
		t.Fatalf("signed query = %q, want timestamp and sign", parsed.RawQuery)
	}
}

func TestDingTalkProviderSendAndTest(t *testing.T) {
	var gotMessage DingTalkMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("request method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotMessage); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	provider := NewDingTalkProvider()
	provider.client = server.Client()

	req := &SendRequest{
		Subject: "主题",
		Body:    "正文",
		Format:  "markdown",
		Config:  map[string]interface{}{"webhook_url": server.URL},
	}

	resp, err := provider.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Send() response = %#v, want success", resp)
	}
	if gotMessage.MsgType != "markdown" || gotMessage.Markdown == nil {
		t.Fatalf("got message = %#v, want markdown message", gotMessage)
	}

	if err := provider.Test(context.Background(), map[string]interface{}{"webhook_url": server.URL}); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}

func TestDingTalkProviderSendReturnsRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"bad token"}`))
	}))
	defer server.Close()

	provider := NewDingTalkProvider()
	provider.client = server.Client()

	resp, err := provider.Send(context.Background(), &SendRequest{
		Body:   "正文",
		Config: map[string]interface{}{"webhook_url": server.URL},
	})
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if resp == nil || resp.Success {
		t.Fatalf("Send() response = %#v, want failed response", resp)
	}
}
