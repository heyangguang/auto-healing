package provider

import (
	"context"
	"testing"
)

func TestEmailProviderParseConfigDefaults(t *testing.T) {
	provider := NewEmailProvider()

	config, err := provider.parseConfig(map[string]interface{}{
		"smtp_host": "smtp.example.com",
		"username":  "bot@example.com",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if config.SMTPPort != 587 {
		t.Fatalf("SMTPPort = %d, want 587", config.SMTPPort)
	}
	if config.FromAddress != "bot@example.com" {
		t.Fatalf("FromAddress = %q, want username fallback", config.FromAddress)
	}
}

func TestEmailProviderParseConfigRequiresHost(t *testing.T) {
	provider := NewEmailProvider()

	_, err := provider.parseConfig(map[string]interface{}{"username": "bot@example.com"})
	if err == nil {
		t.Fatal("parseConfig() error = nil, want error")
	}
}

func TestEmailProviderSendRejectsEmptyRecipients(t *testing.T) {
	provider := NewEmailProvider()

	resp, err := provider.Send(context.Background(), &SendRequest{
		Config: map[string]interface{}{
			"smtp_host":    "smtp.example.com",
			"smtp_port":    2525,
			"username":     "bot@example.com",
			"password":     "secret",
			"from_address": "bot@example.com",
		},
	})
	if err == nil {
		t.Fatal("Send() error = nil, want error")
	}
	if resp == nil || resp.Success {
		t.Fatalf("Send() response = %#v, want failed response", resp)
	}
}
