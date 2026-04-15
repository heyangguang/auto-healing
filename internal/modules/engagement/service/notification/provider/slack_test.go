package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildSlackPayloadUsesMarkdownAndOverrides(t *testing.T) {
	payload := buildSlackPayload(&SendRequest{
		Subject: "告警",
		Body:    "*磁盘使用率过高*",
		Format:  "markdown",
	}, &SlackConfig{
		Channel:   "#ops",
		Username:  "auto-healing",
		IconEmoji: ":robot_face:",
	})
	if payload.Text != "*磁盘使用率过高*" {
		t.Fatalf("payload.Text = %q, want markdown body", payload.Text)
	}
	if !payload.Mrkdwn {
		t.Fatal("payload.Mrkdwn = false, want true")
	}
	if payload.Channel != "#ops" || payload.Username != "auto-healing" || payload.IconEmoji != ":robot_face:" {
		t.Fatalf("payload overrides = %#v, want copied config", payload)
	}
}

func TestSlackProviderSendAndTest(t *testing.T) {
	var got slackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	provider := NewSlackProvider()
	provider.client = server.Client()

	resp, err := provider.Send(context.Background(), &SendRequest{
		Subject: "Slack 告警",
		Body:    "服务恢复完成",
		Config:  map[string]interface{}{"webhook_url": server.URL},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Send() response = %#v, want success", resp)
	}
	if got.Text != "Slack 告警\n\n服务恢复完成" {
		t.Fatalf("got.Text = %q, want merged subject and body", got.Text)
	}

	if err := provider.Test(context.Background(), map[string]interface{}{"webhook_url": server.URL}); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}
