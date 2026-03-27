package incident

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildIncidentSyncUpdatesExcludesRuntimeFields(t *testing.T) {
	now := time.Now()
	incident := &platformmodel.Incident{
		PluginID:         ptrUUID(uuid.New()),
		SourcePluginName: "plugin",
		ExternalID:       "ext-1",
		Title:            "title",
		Status:           "open",
		SourceCreatedAt:  &now,
		SourceUpdatedAt:  &now,
	}

	updates := buildIncidentSyncUpdates(incident)
	for _, key := range []string{"scanned", "matched_rule_id", "healing_flow_instance_id", "healing_status", "tenant_id"} {
		if _, exists := updates[key]; exists {
			t.Fatalf("updates should not override runtime field %q", key)
		}
	}
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}

func TestIncidentGetByIDReturnsNotFoundSentinel(t *testing.T) {
	db := openIncidentRepoTestDB(t)
	mustExecIncidentSQL(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	ctx := WithTenantID(context.Background(), uuid.New())
	_, err := NewIncidentRepositoryWithDB(db).GetByID(ctx, uuid.New())
	if !errors.Is(err, ErrIncidentNotFound) {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrIncidentNotFound)
	}
}

func openIncidentRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExecIncidentSQL(t *testing.T, db *gorm.DB, sql string, args ...interface{}) {
	t.Helper()

	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
