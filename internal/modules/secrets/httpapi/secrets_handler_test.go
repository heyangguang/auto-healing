package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/middleware"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	"github.com/gin-gonic/gin"
)

func TestQuerySecretRequiresPluginUpdatePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/secrets/query", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.PermissionsKey, []string{"playbook:execute"})

	h := &SecretsHandler{}
	h.QuerySecret(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}

	var resp struct {
		ErrorCode string         `json:"error_code"`
		Details   map[string]any `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != middleware.ErrorCodePermissionRequired {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, middleware.ErrorCodePermissionRequired)
	}
	if resp.Details["required_permission"] != "secrets:query" {
		t.Fatalf("details.required_permission = %#v, want secrets:query", resp.Details["required_permission"])
	}
}

func TestClassifySecretsError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "invalid id",
			err:        fmt.Errorf("%w: bad uuid", secretsSvc.ErrSecretsSourceInvalidID),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid secrets source id: bad uuid",
		},
		{
			name:       "invalid input",
			err:        fmt.Errorf("%w: 无效的状态: banana", secretsSvc.ErrSecretsSourceInvalidInput),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid secrets source input: 无效的状态: banana",
		},
		{
			name:       "in use",
			err:        fmt.Errorf("%w: still referenced", secretsSvc.ErrSecretsSourceInUse),
			wantStatus: http.StatusConflict,
			wantMsg:    "secrets source in use: still referenced",
		},
		{
			name:       "not found",
			err:        secretsSvc.ErrSecretsSourceNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    "密钥源不存在",
		},
		{
			name:       "provider unavailable",
			err:        fmt.Errorf("wrapped: %w", secretsSvc.ErrSecretsProviderConnectionFailed),
			wantStatus: http.StatusBadGateway,
			wantMsg:    "密钥提供方不可用",
		},
		{
			name:       "already active",
			err:        fmt.Errorf("%w: source-a", secretsSvc.ErrSecretsSourceAlreadyActive),
			wantStatus: http.StatusConflict,
			wantMsg:    "secrets source already active: source-a",
		},
		{
			name:       "duplicate name",
			err:        fmt.Errorf("UNIQUE constraint failed: secrets_sources.name"),
			wantStatus: http.StatusConflict,
			wantMsg:    "密钥源名称已存在",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotMsg := classifySourceAdminError(tt.err)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %d, want %d", gotStatus, tt.wantStatus)
			}
			if gotMsg != tt.wantMsg {
				t.Fatalf("message = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}
