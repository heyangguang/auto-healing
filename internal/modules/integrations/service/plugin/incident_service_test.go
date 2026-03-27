package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	sharedrepo "github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBatchResetScanRequiresScope(t *testing.T) {
	svc := &IncidentService{}
	if _, err := svc.BatchResetScan(nil, nil, ""); err == nil {
		t.Fatal("expected empty batch reset request to be rejected")
	}
}

func TestWriteBackIncidentCloseIgnoresMissingPlugin(t *testing.T) {
	db := openIncidentServiceTestDB(t, `
		CREATE TABLE plugins (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	pluginID := uuid.New()
	svc := NewIncidentService()
	ctx := sharedrepo.WithTenantID(context.Background(), uuid.New())
	incident := &model.Incident{PluginID: &pluginID}

	sourceUpdated, err := svc.writeBackIncidentClose(ctx, uuid.New(), incident, "", "", "", "resolved")
	if err != nil {
		t.Fatalf("writeBackIncidentClose() unexpected error: %v", err)
	}
	if sourceUpdated {
		t.Fatal("writeBackIncidentClose() should not report source updated when plugin is missing")
	}
}

func TestWriteBackIncidentCloseReturnsPluginLookupError(t *testing.T) {
	db := openIncidentServiceTestDB(t)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	pluginID := uuid.New()
	svc := NewIncidentService()
	ctx := sharedrepo.WithTenantID(context.Background(), uuid.New())
	incident := &model.Incident{PluginID: &pluginID}

	_, err := svc.writeBackIncidentClose(ctx, uuid.New(), incident, "", "", "", "resolved")
	if err == nil {
		t.Fatal("writeBackIncidentClose() expected plugin lookup error")
	}
	if errors.Is(err, integrationrepo.ErrPluginNotFound) {
		t.Fatalf("writeBackIncidentClose() should wrap unexpected lookup failures, got %v", err)
	}
}

func openIncidentServiceTestDB(t *testing.T, statements ...string) *gorm.DB {
	t.Helper()

	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("exec sql failed: %v", err)
		}
	}
	return db
}
