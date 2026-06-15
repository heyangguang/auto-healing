package plugin

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreatePluginRejectsMissingURL(t *testing.T) {
	db := openPluginValidationDB(t)
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())

	_, err := svc.CreatePlugin(ctx, &model.Plugin{
		ID:                  uuid.New(),
		Name:                "bad-plugin",
		Type:                "itsm",
		Version:             "1.0.0",
		Config:              model.JSON{"auth_type": "none"},
		FieldMapping:        model.JSON{},
		SyncEnabled:         false,
		SyncIntervalMinutes: 5,
		MaxFailures:         5,
	})
	if err == nil {
		t.Fatal("CreatePlugin() error = nil, want missing URL rejection")
	}
}

func TestCreatePluginRejectsIncompleteAuthConfig(t *testing.T) {
	db := openPluginValidationDB(t)
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())

	_, err := svc.CreatePlugin(ctx, &model.Plugin{
		ID:                  uuid.New(),
		Name:                "bad-auth",
		Type:                "cmdb",
		Version:             "1.0.0",
		Config:              model.JSON{"url": "http://127.0.0.1:18085/cmdb", "auth_type": "bearer"},
		FieldMapping:        model.JSON{},
		SyncEnabled:         false,
		SyncIntervalMinutes: 5,
		MaxFailures:         5,
	})
	if err == nil {
		t.Fatal("CreatePlugin() error = nil, want incomplete auth rejection")
	}
}

func TestCreatePluginRejectsInvalidSyncFilter(t *testing.T) {
	db := openPluginValidationDB(t)
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())

	_, err := svc.CreatePlugin(ctx, &model.Plugin{
		ID:                  uuid.New(),
		Name:                "bad-filter",
		Type:                "itsm",
		Version:             "1.0.0",
		Config:              model.JSON{"url": "http://127.0.0.1:18085/incidents"},
		FieldMapping:        model.JSON{},
		SyncFilter:          model.JSON{"field": "status", "operator": "bad_operator", "value": "new"},
		SyncEnabled:         false,
		SyncIntervalMinutes: 5,
		MaxFailures:         5,
	})
	if err == nil {
		t.Fatal("CreatePlugin() error = nil, want invalid sync filter rejection")
	}
}

func TestCreatePluginAcceptsValidHTTPConfig(t *testing.T) {
	db := openPluginValidationDB(t)
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())

	plugin, err := svc.CreatePlugin(ctx, &model.Plugin{
		ID:      uuid.New(),
		Name:    "good-plugin",
		Type:    "itsm",
		Version: "1.0.0",
		Config: model.JSON{
			"url":                   "http://127.0.0.1:18085/incidents",
			"auth_type":             "api_key",
			"api_key":               "secret",
			"api_key_header":        "X-Test-Key",
			"response_data_path":    "data.items",
			"close_incident_url":    "http://127.0.0.1:18085/incidents/{external_id}/close",
			"close_incident_method": "POST",
		},
		FieldMapping: model.JSON{
			"incident_mapping": map[string]interface{}{"external_id": "id", "title": "title"},
		},
		SyncFilter:          model.JSON{"field": "status", "operator": "equals", "value": "new"},
		SyncEnabled:         true,
		SyncIntervalMinutes: 5,
		MaxFailures:         5,
	})
	if err != nil {
		t.Fatalf("CreatePlugin() error = %v", err)
	}
	if plugin.ID == uuid.Nil {
		t.Fatal("plugin ID should be set")
	}
}

func openPluginValidationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "plugin-validation.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mustExecPluginValidationSQL(t, db, `
		CREATE TABLE plugins (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT,
			version TEXT NOT NULL,
			config TEXT NOT NULL,
			field_mapping TEXT,
			sync_filter TEXT,
			sync_enabled BOOLEAN,
			sync_interval_minutes INTEGER,
			last_sync_at DATETIME,
			next_sync_at DATETIME,
			max_failures INTEGER,
			consecutive_failures INTEGER,
			pause_reason TEXT,
			status TEXT,
			error_message TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	return db
}

func mustExecPluginValidationSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}
