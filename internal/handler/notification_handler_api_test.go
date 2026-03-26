package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/notification"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateChannelRouteReturnsConflictOnDuplicateName(t *testing.T) {
	db := newNotificationHandlerTestDB(t)
	createNotificationHandlerChannelSchema(t, db)
	handler := &NotificationHandler{
		svc: notification.NewService(db, "test", "http://localhost", "v1"),
	}

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
	})
	router.POST("/tenant/notification-channels", handler.CreateChannel)

	body := `{"name":"dup-channel","type":"webhook","config":{}}`
	first := issueNotificationHandlerJSON(t, router, http.MethodPost, "/tenant/notification-channels", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want %d; body=%s", first.Code, http.StatusCreated, first.Body.String())
	}

	second := issueNotificationHandlerJSON(t, router, http.MethodPost, "/tenant/notification-channels", body)
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, want %d; body=%s", second.Code, http.StatusConflict, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "已存在") {
		t.Fatalf("duplicate create body = %s, want conflict message", second.Body.String())
	}
}

func TestSendNotificationRouteReturnsBadRequestForInactiveChannel(t *testing.T) {
	db := newNotificationHandlerTestDB(t)
	createNotificationHandlerChannelSchema(t, db)
	tenantID := uuid.New()
	channelID := uuid.New()
	insertNotificationHandlerChannel(t, db, channelID, tenantID, false)
	handler := &NotificationHandler{
		svc: notification.NewService(db, "test", "http://localhost", "v1"),
	}

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: tenantID.String(),
	})
	router.POST("/tenant/notifications/send", handler.SendNotification)

	recorder := issueNotificationHandlerJSON(
		t,
		router,
		http.MethodPost,
		"/tenant/notifications/send",
		fmt.Sprintf(`{"channel_ids":["%s"],"body":"hello"}`, channelID),
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("send status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "inactive") && !strings.Contains(recorder.Body.String(), "停用") {
		t.Fatalf("send body = %s, want inactive channel message", recorder.Body.String())
	}
}

func newNotificationHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "notification-handler.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createNotificationHandlerChannelSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecNotificationHandlerSQL(t, db, `
		CREATE TABLE notification_channels (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT,
			config TEXT NOT NULL DEFAULT '{}',
			retry_config TEXT,
			recipients TEXT NOT NULL DEFAULT '[]',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			rate_limit_per_minute INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecNotificationHandlerSQL(t, db, `CREATE UNIQUE INDEX idx_channel_tenant_name ON notification_channels(tenant_id, name);`)
}

func insertNotificationHandlerChannel(t *testing.T, db *gorm.DB, channelID, tenantID uuid.UUID, active bool) {
	t.Helper()
	mustExecNotificationHandlerSQL(
		t,
		db,
		`INSERT INTO notification_channels (id, tenant_id, name, type, config, recipients, is_active, is_default) VALUES (?, ?, ?, ?, '{}', '[]', ?, false)`,
		channelID.String(),
		tenantID.String(),
		fmt.Sprintf("channel-%s", channelID.String()[:8]),
		"webhook",
		active,
	)
}

func issueNotificationHandlerJSON(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	return recorder
}

func mustExecNotificationHandlerSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
