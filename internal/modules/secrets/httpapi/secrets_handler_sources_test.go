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
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateSourceReturnsConflictOnDuplicateName(t *testing.T) {
	db := newSecretsHandlerTestDB(t)
	createSecretsHandlerSourceTable(t, db)
	installSecretsHandlerDB(t, db)

	tenantID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsHandler(t, db, insertSecretsHandlerSourceSQL,
		uuid.NewString(), tenantID.String(), "dup-source", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, false, 1, "active", now, now)

	body := bytes.NewBufferString(`{"name":"dup-source","type":"webhook","auth_type":"password","config":{"url":"http://example.com","method":"GET","query_key":"hostname"}}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/secrets/sources", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(platformrepo.WithTenantID(req.Context(), tenantID))
	c.Request = req

	h := &SecretsHandler{svc: secretsSvc.NewServiceWithDB(db)}
	h.CreateSource(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	assertResponseMessage(t, w.Body.Bytes(), "密钥源名称已存在")
}

func TestGetSourceReturnsNotFound(t *testing.T) {
	db := newSecretsHandlerTestDB(t)
	createSecretsHandlerSourceTable(t, db)
	installSecretsHandlerDB(t, db)

	tenantID := uuid.New()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/secrets/sources/"+uuid.NewString(), nil)
	req = req.WithContext(platformrepo.WithTenantID(req.Context(), tenantID))
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: uuid.NewString()}}

	h := &SecretsHandler{svc: secretsSvc.NewServiceWithDB(db)}
	h.GetSource(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
	assertResponseMessage(t, w.Body.Bytes(), "密钥源不存在")
}

func TestEnableReturnsBadGatewayWhenProviderUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := newSecretsHandlerTestDB(t)
	createSecretsHandlerSourceTable(t, db)
	installSecretsHandlerDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	config := `{"url":"` + server.URL + `","method":"GET","query_key":"hostname"}`
	mustExecSecretsHandler(t, db, insertSecretsHandlerSourceSQL,
		sourceID.String(), tenantID.String(), "inactive-source", "webhook", "password",
		config, false, 1, "inactive", now, now)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/secrets/sources/"+sourceID.String()+"/enable", nil)
	req = req.WithContext(platformrepo.WithTenantID(req.Context(), tenantID))
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: sourceID.String()}}

	h := &SecretsHandler{svc: secretsSvc.NewServiceWithDB(db)}
	h.Enable(c)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusBadGateway, w.Body.String())
	}
	assertResponseMessage(t, w.Body.Bytes(), "密钥提供方不可用")
}

func TestListSourcesRejectsInvalidIsDefaultQuery(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/secrets/sources?is_default=foo", nil)

	h := &SecretsHandler{}
	h.ListSources(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	assertResponseMessage(t, w.Body.Bytes(), "is_default 参数必须为 true 或 false")
}

func newSecretsHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "secrets-handler.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createSecretsHandlerSourceTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecSecretsHandler(t, db, `
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

func installSecretsHandlerDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })
}

func mustExecSecretsHandler(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func assertResponseMessage(t *testing.T, body []byte, want string) {
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

const insertSecretsHandlerSourceSQL = `
	INSERT INTO secrets_sources (
		id, tenant_id, name, type, auth_type, config, is_default, priority, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
