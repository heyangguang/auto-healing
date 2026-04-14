package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/database"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
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
	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())
	incident := &platformmodel.Incident{PluginID: &pluginID}

	req, err := svc.buildIncidentWritebackRequest(ctx, incident, CloseIncidentParams{CloseStatus: "resolved"})
	if err != nil {
		t.Fatalf("buildIncidentWritebackRequest() unexpected error: %v", err)
	}
	if req != nil {
		t.Fatal("buildIncidentWritebackRequest() should return nil when plugin is missing")
	}
}

func TestWriteBackIncidentCloseReturnsPluginLookupError(t *testing.T) {
	db := openIncidentServiceTestDB(t)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	pluginID := uuid.New()
	svc := NewIncidentServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())
	incident := &platformmodel.Incident{PluginID: &pluginID}

	_, err := svc.buildIncidentWritebackRequest(ctx, incident, CloseIncidentParams{CloseStatus: "resolved"})
	if err == nil {
		t.Fatal("buildIncidentWritebackRequest() expected plugin lookup error")
	}
	if errors.Is(err, integrationrepo.ErrPluginNotFound) {
		t.Fatalf("buildIncidentWritebackRequest() should wrap unexpected lookup failures, got %v", err)
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
