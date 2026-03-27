package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestQuerySecretReturnsBadGatewayOnProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := newSecretsQueryHandlerTestDB(t)
	createSecretsQueryHandlerTables(t, db)
	installSecretsQueryHandlerDB(t, db)

	tenantID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsQueryHandler(t, db, insertSecretsQuerySourceSQL,
		uuid.NewString(), tenantID.String(), "default-source", "webhook", "password",
		`{"url":"`+server.URL+`","method":"GET","query_key":"hostname"}`, true, 1, "active", now, now)

	body := bytes.NewBufferString(`{"hostname":"host-a"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/secrets/query", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(platformrepo.WithTenantID(req.Context(), tenantID))
	c.Request = req
	c.Set(middleware.PermissionsKey, []string{"secrets:query"})

	h := &SecretsHandler{svc: secretsSvc.NewService()}
	h.QuerySecret(c)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusBadGateway, w.Body.String())
	}
	assertSecretsQueryResponseMessage(t, w.Body.Bytes(), "密钥提供方不可用")
}

func TestTestQueryMasksProviderFailureInResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := newSecretsQueryHandlerTestDB(t)
	createSecretsQueryHandlerTables(t, db)
	installSecretsQueryHandlerDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsQueryHandler(t, db, insertSecretsQuerySourceSQL,
		sourceID.String(), tenantID.String(), "source-a", "webhook", "password",
		`{"url":"`+server.URL+`","method":"GET","query_key":"hostname"}`, false, 1, "active", now, now)

	body := bytes.NewBufferString(`{"hostname":"host-a"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/secrets/sources/"+sourceID.String()+"/test-query", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(platformrepo.WithTenantID(req.Context(), tenantID))
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: sourceID.String()}}

	h := &SecretsHandler{svc: secretsSvc.NewService()}
	h.TestQuery(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Data struct {
			Results []struct {
				Message string `json:"message"`
				Success bool   `json:"success"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(resp.Data.Results))
	}
	if resp.Data.Results[0].Success {
		t.Fatalf("expected success=false")
	}
	if resp.Data.Results[0].Message != "密钥提供方不可用" {
		t.Fatalf("message = %q, want %q", resp.Data.Results[0].Message, "密钥提供方不可用")
	}
}

func TestQuerySecretRejectsMissingHostnameAndIPAddress(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/secrets/query", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(platformrepo.WithTenantID(req.Context(), uuid.New()))
	c.Request = req
	c.Set(middleware.PermissionsKey, []string{"secrets:query"})

	h := &SecretsHandler{svc: secretsSvc.NewService()}
	h.QuerySecret(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	assertSecretsQueryResponseMessage(t, w.Body.Bytes(), "请提供 hostname 或 ip_address")
}

func newSecretsQueryHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "secrets-query-handler.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createSecretsQueryHandlerTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecSecretsQueryHandler(t, db, `
		CREATE TABLE secrets_sources (
			id TEXT PRIMARY KEY NOT NULL DEFAULT (
				lower(hex(randomblob(4))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(6)))
			),
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			config TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			priority INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'inactive',
			last_test_at DATETIME,
			last_test_result BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE (tenant_id, name)
		);
	`)
}

func installSecretsQueryHandlerDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })
}

func mustExecSecretsQueryHandler(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func assertSecretsQueryResponseMessage(t *testing.T, body []byte, want string) {
	t.Helper()
	var resp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Message != want {
		t.Fatalf("message = %q, want %q", resp.Message, want)
	}
}

const insertSecretsQuerySourceSQL = `
	INSERT INTO secrets_sources (
		id, tenant_id, name, type, auth_type, config, is_default, priority, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
