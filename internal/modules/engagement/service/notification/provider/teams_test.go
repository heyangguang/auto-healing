package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildTeamsPayloadUsesMessageCard(t *testing.T) {
	payload := buildTeamsPayload(&SendRequest{
		Subject: "恢复完成",
		Body:    "主机已恢复上线",
	}, &TeamsConfig{ThemeColor: "112233"})
	if payload.Type != "MessageCard" {
		t.Fatalf("payload.Type = %q, want MessageCard", payload.Type)
	}
	if payload.Context != "https://schema.org/extensions" {
		t.Fatalf("payload.Context = %q, want schema context", payload.Context)
	}
	if payload.Title != "恢复完成" || payload.Text != "主机已恢复上线" {
		t.Fatalf("payload = %#v, want title/body copied", payload)
	}
	if payload.ThemeColor != "112233" {
		t.Fatalf("payload.ThemeColor = %q, want custom theme color", payload.ThemeColor)
	}
}

func TestTeamsProviderSendAndTest(t *testing.T) {
	var got teamsMessageCard
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte("1"))
	}))
	defer server.Close()

	provider := NewTeamsProvider()
	provider.client = server.Client()

	resp, err := provider.Send(context.Background(), &SendRequest{
		Subject: "Teams 告警",
		Body:    "自动修复成功",
		Config:  map[string]interface{}{"webhook_url": server.URL},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Send() response = %#v, want success", resp)
	}
	if got.Title != "Teams 告警" || got.Text != "自动修复成功" {
		t.Fatalf("got payload = %#v, want title and body", got)
	}

	if err := provider.Test(context.Background(), map[string]interface{}{"webhook_url": server.URL}); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}
