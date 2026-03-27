package cmdb

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCMDBRepositoryCreateListUpdateAndStats(t *testing.T) {
	db := openCMDBRepositoryTestDB(t)
	createCMDBRepositorySchema(t, db)

	repo := NewCMDBItemRepositoryWithDB(db)
	tenantA := uuid.New()
	tenantB := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantA)
	now := time.Now().UTC()

	itemA := &platformmodel.CMDBItem{
		ID:               uuid.New(),
		ExternalID:       "ext-a",
		Name:             "host-a",
		Type:             "server",
		Status:           "active",
		IPAddress:        "10.0.0.1",
		Hostname:         "host-a",
		Environment:      "production",
		SourcePluginName: "plugin-a",
		RawData:          map[string]any{"origin": "test"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repo.Create(ctx, itemA); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	mustExecCMDBSQL(t, db, `
		INSERT INTO cmdb_items (id, tenant_id, external_id, name, type, status, ip_address, hostname, environment, source_plugin_name, raw_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), tenantB.String(), "ext-b", "host-b", "server", "maintenance", "10.0.0.2", "host-b", "staging", "plugin-b", "{}", now, now)

	got, err := repo.GetByID(ctx, itemA.ID)
	if err != nil || got.ID != itemA.ID {
		t.Fatalf("GetByID() = (%+v, %v), want itemA", got, err)
	}

	items, total, err := repo.List(ctx, 1, 10, nil, "server", "active", "", "", query.StringFilter{}, nil, "updated_at", "desc")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != itemA.ID {
		t.Fatalf("List() = total %d items %+v, want tenant-scoped itemA", total, items)
	}

	itemA.Name = "host-a-updated"
	itemA.Status = "maintenance"
	if err := repo.Update(ctx, itemA); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	stats, err := repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats["total"].(int64) != 1 {
		t.Fatalf("GetStats().total = %v, want 1", stats["total"])
	}
}

func TestCMDBRepositoryUpsertPreservesMaintenanceAndListsLogs(t *testing.T) {
	db := openCMDBRepositoryTestDB(t)
	createCMDBRepositorySchema(t, db)

	repo := NewCMDBItemRepositoryWithDB(db)
	tenantID := uuid.New()
	pluginID := uuid.New()
	itemID := uuid.New()
	logID := uuid.New()
	now := time.Now().UTC()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	mustExecCMDBSQL(t, db, `
		INSERT INTO cmdb_items (id, tenant_id, plugin_id, external_id, name, type, status, ip_address, hostname, environment, source_plugin_name, raw_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, itemID.String(), tenantID.String(), pluginID.String(), "ext-a", "host-a", "server", "maintenance", "10.0.0.1", "host-a", "production", "plugin-a", "{}", now, now)

	isNew, err := repo.UpsertByExternalID(ctx, &platformmodel.CMDBItem{
		ID:               uuid.New(),
		PluginID:         &pluginID,
		ExternalID:       "ext-a",
		Name:             "host-a-renamed",
		Type:             "server",
		Status:           "active",
		IPAddress:        "10.0.0.9",
		Hostname:         "host-a",
		Environment:      "production",
		SourcePluginName: "plugin-a",
		RawData:          map[string]any{"sync": true},
	})
	if err != nil {
		t.Fatalf("UpsertByExternalID() error = %v", err)
	}
	if isNew {
		t.Fatal("UpsertByExternalID() isNew = true, want false")
	}

	var stored platformmodel.CMDBItem
	if err := db.First(&stored, "id = ?", itemID.String()).Error; err != nil {
		t.Fatalf("reload cmdb item: %v", err)
	}
	if stored.Status != "maintenance" || stored.Name != "host-a-renamed" || stored.IPAddress != "10.0.0.9" {
		t.Fatalf("stored cmdb item = %+v, want preserved maintenance status with updated payload", stored)
	}

	if err := repo.CreateMaintenanceLog(ctx, &platformmodel.CMDBMaintenanceLog{
		ID:           logID,
		CMDBItemID:   itemID,
		CMDBItemName: stored.Name,
		Action:       "enter",
		Reason:       "planned",
		Operator:     "tester",
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("CreateMaintenanceLog() error = %v", err)
	}
	logs, total, err := repo.ListMaintenanceLogs(ctx, itemID, 1, 10)
	if err != nil {
		t.Fatalf("ListMaintenanceLogs() error = %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].ID != logID {
		t.Fatalf("ListMaintenanceLogs() = total %d logs %+v, want one maintenance log", total, logs)
	}
}

func openCMDBRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createCMDBRepositorySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecCMDBSQL(t, db, `
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			plugin_id TEXT,
			source_plugin_name TEXT,
			external_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT,
			status TEXT,
			ip_address TEXT,
			hostname TEXT,
			os TEXT,
			os_version TEXT,
			cpu TEXT,
			memory TEXT,
			disk TEXT,
			location TEXT,
			owner TEXT,
			environment TEXT,
			manufacturer TEXT,
			model TEXT,
			serial_number TEXT,
			department TEXT,
			dependencies TEXT,
			tags TEXT,
			raw_data TEXT NOT NULL,
			source_created_at DATETIME,
			source_updated_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			maintenance_reason TEXT,
			maintenance_start_at DATETIME,
			maintenance_end_at DATETIME
		);
		CREATE TABLE cmdb_maintenance_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			cmdb_item_id TEXT NOT NULL,
			cmdb_item_name TEXT,
			action TEXT NOT NULL,
			reason TEXT,
			scheduled_end_at DATETIME,
			actual_end_at DATETIME,
			exit_type TEXT,
			operator TEXT,
			created_at DATETIME
		);
	`)
}

func mustExecCMDBSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
