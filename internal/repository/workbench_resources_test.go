package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestGetResourceOverviewDoesNotExposePlaybooksWithoutPlaybookPermission(t *testing.T) {
	db := newSQLiteTestDB(t)
	createWorkbenchResourceSchema(t, db)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mustExec(t, db, `INSERT INTO playbooks (id, tenant_id, status) VALUES (?, ?, ?)`, uuid.NewString(), tenantID.String(), "draft")
	mustExec(t, db, `INSERT INTO cmdb_items (id, tenant_id, status, type) VALUES (?, ?, ?, ?)`, uuid.NewString(), tenantID.String(), "offline", "host")

	repo := &WorkbenchRepository{db: db}
	overview, err := repo.GetResourceOverview(WithTenantID(context.Background(), tenantID), []string{"plugin:list"})
	if err != nil {
		t.Fatalf("GetResourceOverview() error = %v", err)
	}
	if overview.Hosts.Total == 0 {
		t.Fatalf("hosts overview not populated: %#v", overview)
	}
	if overview.Playbooks.Total != 0 || overview.Playbooks.NeedsReview != nil {
		t.Fatalf("playbook overview leaked without playbook:list: %#v", overview.Playbooks)
	}
}

func createWorkbenchResourceSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE playbooks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT
		);
	`)
	mustExec(t, db, `
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT,
			type TEXT
		);
	`)
	mustExec(t, db, `
		CREATE TABLE secrets_sources (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			type TEXT
		);
	`)
}
