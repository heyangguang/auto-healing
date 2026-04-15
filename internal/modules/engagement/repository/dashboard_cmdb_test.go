package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestGetCMDBSectionUsesFreshQueryState(t *testing.T) {
	db := newSQLiteTestDB(t)
	createCMDBDashboardSchema(t, db)

	repo := NewDashboardRepositoryWithDB(db)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	section, err := repo.GetCMDBSection(WithTenantID(context.Background(), tenantID))
	if err != nil {
		t.Fatalf("GetCMDBSection() error = %v", err)
	}
	if section.Total != 0 || section.Active != 0 || section.Maintenance != 0 || section.Offline != 0 {
		t.Fatalf("unexpected non-zero cmdb counts: %#v", section)
	}
}

func createCMDBDashboardSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			type TEXT,
			status TEXT,
			ip_address TEXT,
			environment TEXT,
			os TEXT,
			department TEXT,
			manufacturer TEXT,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE cmdb_maintenance_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			cmdb_item_name TEXT,
			action TEXT,
			reason TEXT,
			created_at DATETIME
		);
	`)
}
