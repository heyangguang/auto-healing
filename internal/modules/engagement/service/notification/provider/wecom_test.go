package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildWeComPayloadSupportsMarkdownAndText(t *testing.T) {
	markdown := buildWeComPayload(&SendRequest{
		Subject: "标题",
		Body:    "# 正文",
		Format:  "markdown",
	}, &WeComConfig{})
	if markdown.MsgType != "markdown" || markdown.Markdown == nil {
		t.Fatalf("markdown payload = %#v, want markdown message", markdown)
	}
	if markdown.Markdown.Content != "# 正文" {
		t.Fatalf("markdown content = %q, want body", markdown.Markdown.Content)
	}

	text := buildWeComPayload(&SendRequest{
		Subject: "主题",
		Body:    "正文",
	}, &WeComConfig{})
	if text.MsgType != "text" || text.Text == nil {
		t.Fatalf("text payload = %#v, want text message", text)
	}
	if text.Text.Content != "主题\n\n正文" {
		t.Fatalf("text content = %q, want merged subject and body", text.Text.Content)
	}
}

func TestWeComProviderSendAndTest(t *testing.T) {
	var gotMessage weComMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotMessage); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	provider := NewWeComProvider()
	provider.client = server.Client()

	resp, err := provider.Send(context.Background(), &SendRequest{
		Subject: "企业微信标题",
		Body:    "企业微信正文",
		Config: map[string]interface{}{
			"webhook_url": server.URL,
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Send() response = %#v, want success", resp)
	}
	if gotMessage.MsgType != "text" || gotMessage.Text == nil {
		t.Fatalf("got message = %#v, want text payload", gotMessage)
	}

	if err := provider.Test(context.Background(), map[string]interface{}{"webhook_url": server.URL}); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}
