package handler

import (
	"fmt"
	"net/http"
	"testing"

	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
)

func TestClassifySecretQueryError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "secret not found",
			err:        secretsSvc.ErrSecretNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    "密钥未找到",
		},
		{
			name:       "provider invalid config",
			err:        fmt.Errorf("bad config: %w", secretsSvc.ErrSecretsProviderInvalidConfig),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "密钥源配置无效",
		},
		{
			name:       "provider unavailable",
			err:        fmt.Errorf("request failed: %w", secretsSvc.ErrSecretsProviderConnectionFailed),
			wantStatus: http.StatusBadGateway,
			wantMsg:    "密钥提供方不可用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotMsg, ok := classifySecretQueryError(tt.err)
			if !ok {
				t.Fatalf("classifySecretQueryError(%v) returned ok=false", tt.err)
			}
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %d, want %d", gotStatus, tt.wantStatus)
			}
			if gotMsg != tt.wantMsg {
				t.Fatalf("message = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestPublicSecretQueryErrorMessageFallback(t *testing.T) {
	if got := publicSecretQueryErrorMessage(fmt.Errorf("boom")); got != "查询密钥失败" {
		t.Fatalf("message = %q, want %q", got, "查询密钥失败")
	}
}
