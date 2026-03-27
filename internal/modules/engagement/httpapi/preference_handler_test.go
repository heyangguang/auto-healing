package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/middleware"
	respPkg "github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetPreferencesReturnsEmptyObjectWhenNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/preferences", nil)
	c.Set(middleware.UserIDKey, uuid.NewString())

	handler.GetPreferences(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GetPreferences() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp respPkg.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("response data type = %T, want map[string]interface{}", resp.Data)
	}
	preferences, ok := data["preferences"].(map[string]interface{})
	if !ok || len(preferences) != 0 {
		t.Fatalf("preferences = %#v, want empty object", data["preferences"])
	}
}

func TestGetPreferencesReturnsInternalErrorOnRepositoryFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPreferenceTestDB(t)
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/preferences", nil)
	c.Set(middleware.UserIDKey, uuid.NewString())

	handler.GetPreferences(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("GetPreferences() status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestGetPreferencesReturnsInternalErrorWhenStoredDataIsNull(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	userID := uuid.New()
	mustExecPreferenceTest(t, db, `INSERT INTO user_preferences (id, user_id, preferences) VALUES (?, ?, ?)`, uuid.NewString(), userID.String(), []byte("null"))
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/preferences", nil)
	c.Set(middleware.UserIDKey, userID.String())

	handler.GetPreferences(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("GetPreferences() status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestUpdatePreferencesRejectsNullObject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/preferences", bytes.NewBufferString(`{"preferences":null}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.UserIDKey, uuid.NewString())

	handler.UpdatePreferences(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("UpdatePreferences() status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestPatchPreferencesReturnsInternalErrorWhenStoredDataIsCorrupted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	userID := uuid.New()
	mustExecPreferenceTest(t, db, `INSERT INTO user_preferences (id, user_id, preferences) VALUES (?, ?, ?)`, uuid.NewString(), userID.String(), []byte("not-json"))
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPatch, "/preferences", bytes.NewBufferString(`{"preferences":{"theme":"dark"}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.UserIDKey, userID.String())

	handler.PatchPreferences(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("PatchPreferences() status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPatchPreferencesReturnsInternalErrorWhenStoredDataIsNull(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	userID := uuid.New()
	mustExecPreferenceTest(t, db, `INSERT INTO user_preferences (id, user_id, preferences) VALUES (?, ?, ?)`, uuid.NewString(), userID.String(), []byte("null"))
	handler := &PreferenceHandler{
		prefRepo: repository.NewUserPreferenceRepositoryWithDB(db),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPatch, "/preferences", bytes.NewBufferString(`{"preferences":{"theme":"dark"}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.UserIDKey, userID.String())

	handler.PatchPreferences(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("PatchPreferences() status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func newPreferenceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "preferences.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createUserPreferenceSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecPreferenceTest(t, db, `
		CREATE TABLE user_preferences (
			id TEXT PRIMARY KEY NOT NULL DEFAULT (
				lower(hex(randomblob(4))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(6)))
			),
			user_id TEXT NOT NULL,
			tenant_id TEXT,
			preferences BLOB NOT NULL DEFAULT '{}',
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecPreferenceTest(t, db, `CREATE UNIQUE INDEX idx_user_preferences_user_tenant ON user_preferences(user_id, tenant_id);`)
}

func mustExecPreferenceTest(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
