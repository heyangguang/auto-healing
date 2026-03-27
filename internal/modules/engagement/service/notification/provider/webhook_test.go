package provider

import (
	"testing"
	"time"
)

func TestWebhookHTTPClientCreatesIndependentClients(t *testing.T) {
	defaultClient := webhookHTTPClient(0)
	customClient := webhookHTTPClient(5)

	if defaultClient == customClient {
		t.Fatal("webhookHTTPClient() returned shared client instance")
	}
	if defaultClient.Timeout != 30*time.Second {
		t.Fatalf("default timeout = %v, want %v", defaultClient.Timeout, 30*time.Second)
	}
	if customClient.Timeout != 5*time.Second {
		t.Fatalf("custom timeout = %v, want %v", customClient.Timeout, 5*time.Second)
	}
}
