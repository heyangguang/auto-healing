package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSanitizeAuditJSONMasksNestedSensitiveFields(t *testing.T) {
	payload := model.JSON{
		"config": map[string]interface{}{
			"auth": map[string]interface{}{
				"token":       "secret-token",
				"password":    "secret-password",
				"private_key": "secret-key",
			},
			"nested_json": `{"secret_id":"abc","safe":"ok"}`,
		},
		"safe": "value",
	}

	masked := sanitizeAuditJSON(payload)

	config, ok := masked["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("config type = %T, want map[string]interface{}", masked["config"])
	}
	auth := config["auth"].(map[string]interface{})
	if auth["token"] != "***" || auth["password"] != "***" || auth["private_key"] != "***" {
		t.Fatalf("nested auth was not masked: %#v", auth)
	}

	nestedJSON, ok := config["nested_json"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested_json type = %T, want parsed masked map", config["nested_json"])
	}
	if nestedJSON["secret_id"] != "***" {
		t.Fatalf("secret_id = %#v, want masked", nestedJSON["secret_id"])
	}
	if nestedJSON["safe"] != "ok" {
		t.Fatalf("safe = %#v, want ok", nestedJSON["safe"])
	}
	if masked["safe"] != "value" {
		t.Fatalf("safe field changed unexpectedly: %#v", masked["safe"])
	}
}

func TestReadAuditRequestBodyCapsCapturedBytesWithoutBreakingRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := bytes.Repeat([]byte("a"), maxAuditRequestBodyBytes+2048)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins", bytes.NewReader(body))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	captured := readAuditRequestBody(c)
	if len(captured) != maxAuditRequestBodyBytes {
		t.Fatalf("captured length = %d, want %d", len(captured), maxAuditRequestBodyBytes)
	}

	replayed, err := io.ReadAll(c.Request.Body)
	if err != nil {
		t.Fatalf("ReadAll(request body): %v", err)
	}
	if !bytes.Equal(replayed, body) {
		t.Fatal("request body was not preserved for downstream handlers")
	}
}

func TestResolveResourceNameReturnsQueryError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:audit-resource-name-error?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	resourceID := uuid.New()
	_, err = resolveResourceName(db, "/api/v1/plugins/"+resourceID.String(), &resourceID, "", nil, uuid.Nil)
	if err == nil {
		t.Fatal("resolveResourceName() error = nil, want query error")
	}
}

func TestResolveResourceNameReturnsTenantLookupError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:audit-tenant-name-error?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	body := model.JSON{"tenant_id": uuid.NewString()}
	_, err = resolveResourceName(db, "/api/v1/platform/impersonation/requests", nil, "", body, uuid.Nil)
	if err == nil {
		t.Fatal("resolveResourceName() error = nil, want tenant lookup error")
	}
}
