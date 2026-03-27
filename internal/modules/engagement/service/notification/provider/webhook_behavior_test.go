package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookProviderBuildsRequestWithHeadersAndAuth(t *testing.T) {
	provider := NewWebhookProvider()
	config, err := provider.parseConfig(map[string]interface{}{
		"url":      "https://example.com/hook",
		"headers":  map[string]string{"X-Token": "abc"},
		"username": "user",
		"password": "pass",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	httpReq, err := provider.newWebhookRequest(context.Background(), config, webhookPayload(&SendRequest{
		Subject:    "主题",
		Body:       "正文",
		Format:     "markdown",
		Recipients: []string{"a@example.com"},
	}))
	if err != nil {
		t.Fatalf("newWebhookRequest() error = %v", err)
	}
	if httpReq.Method != http.MethodPost {
		t.Fatalf("request method = %s, want POST", httpReq.Method)
	}
	if httpReq.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", httpReq.Header.Get("Content-Type"))
	}
	if httpReq.Header.Get("X-Token") != "abc" {
		t.Fatalf("X-Token = %q, want abc", httpReq.Header.Get("X-Token"))
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if httpReq.Header.Get("Authorization") != wantAuth {
		t.Fatalf("Authorization = %q, want %q", httpReq.Header.Get("Authorization"), wantAuth)
	}
}

func TestBuildWebhookSendResult(t *testing.T) {
	successResp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	success, err := buildWebhookSendResult(successResp)
	if err != nil {
		t.Fatalf("buildWebhookSendResult(success) error = %v", err)
	}
	if !success.Success {
		t.Fatalf("success response = %#v, want success", success)
	}

	failResp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader("boom")),
	}
	failed, err := buildWebhookSendResult(failResp)
	if err == nil {
		t.Fatal("buildWebhookSendResult(failed) error = nil, want error")
	}
	if failed == nil || failed.Success {
		t.Fatalf("failed response = %#v, want failed response", failed)
	}
}

func TestWebhookProviderSendAndTest(t *testing.T) {
	var gotMethod string
	var gotPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	provider := NewWebhookProvider()
	resp, err := provider.Send(context.Background(), &SendRequest{
		Subject:    "主题",
		Body:       "正文",
		Format:     "markdown",
		Recipients: []string{"a@example.com"},
		Config: map[string]interface{}{
			"url":    server.URL,
			"method": http.MethodPut,
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !resp.Success {
		t.Fatalf("Send() response = %#v, want success", resp)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("request method = %s, want %s", gotMethod, http.MethodPut)
	}
	if gotPayload["subject"] != "主题" || gotPayload["body"] != "正文" {
		t.Fatalf("payload = %#v, want subject/body", gotPayload)
	}

	if err := provider.Test(context.Background(), map[string]interface{}{"url": server.URL}); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}

func TestWebhookProviderHandlesConfigAndRemoteErrors(t *testing.T) {
	provider := NewWebhookProvider()
	if _, err := provider.parseConfig(map[string]interface{}{}); err == nil {
		t.Fatal("parseConfig() error = nil, want error")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid"))
	}))
	defer server.Close()

	resp, err := provider.Send(context.Background(), &SendRequest{
		Body:   "正文",
		Config: map[string]interface{}{"url": server.URL},
	})
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if resp == nil || resp.Success {
		t.Fatalf("Send() response = %#v, want failed response", resp)
	}

	if err := provider.Test(context.Background(), map[string]interface{}{"url": server.URL}); err == nil {
		t.Fatal("Test() error = nil, want error")
	}
}
