package repository

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestGetUserTenantsReturnsStableOrder(t *testing.T) {
	db := newSQLiteTestDB(t)
	createTenantSchema(t, db)

	repo := NewTenantRepositoryWithDB(db)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mustExec(t, db, `INSERT INTO tenants (id, name, code, status) VALUES (?, ?, ?, ?)`, tenantB.String(), "Tenant B", "b", model.TenantStatusActive)
	mustExec(t, db, `INSERT INTO tenants (id, name, code, status) VALUES (?, ?, ?, ?)`, tenantA.String(), "Tenant A", "a", model.TenantStatusActive)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), userID.String(), tenantB.String(), uuid.NewString(), time.Now().UTC().Format(time.RFC3339))
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), userID.String(), tenantA.String(), uuid.NewString(), time.Now().UTC().Format(time.RFC3339))

	tenants, err := repo.GetUserTenants(context.Background(), userID, "")
	if err != nil {
		t.Fatalf("GetUserTenants: %v", err)
	}
	if len(tenants) != 2 {
		t.Fatalf("tenant count = %d, want 2", len(tenants))
	}
	if tenants[0].ID != tenantA || tenants[1].ID != tenantB {
		t.Fatalf("tenant order = [%s, %s], want [%s, %s]", tenants[0].ID, tenants[1].ID, tenantA, tenantB)
	}
}

func createTenantSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE tenants (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE user_tenant_roles (
			id TEXT PRIMARY KEY NOT NULL,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			role_id TEXT NOT NULL,
			created_at DATETIME
		);
	`)
}
