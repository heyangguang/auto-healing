package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTenantRepositoryUpdateMemberRoleRejectsPlatformRole(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE roles (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			display_name TEXT,
			description TEXT,
			is_system BOOLEAN,
			scope TEXT,
			tenant_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE permissions (
			id TEXT PRIMARY KEY NOT NULL,
			code TEXT,
			name TEXT,
			description TEXT,
			module TEXT,
			resource TEXT,
			action TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE role_permissions (
			id TEXT PRIMARY KEY NOT NULL,
			role_id TEXT,
			permission_id TEXT,
			created_at DATETIME
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

	repo := &TenantRepository{db: db}
	tenantID := uuid.New()
	userID := uuid.New()
	tenantRoleID := uuid.New()
	platformRoleID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
		tenantRoleID.String(), "viewer", "Viewer", true, "tenant", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
		platformRoleID.String(), "platform_admin", "Platform Admin", true, "platform", now, now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), tenantID.String(), tenantRoleID.String(), now)

	err := repo.UpdateMemberRole(context.Background(), userID, tenantID, platformRoleID)
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("UpdateMemberRole() error = %v, want %v", err, ErrRoleNotFound)
	}

	var roleID string
	if err := db.Table("user_tenant_roles").Select("role_id").Where("user_id = ? AND tenant_id = ?", userID.String(), tenantID.String()).Scan(&roleID).Error; err != nil {
		t.Fatalf("query role_id error = %v", err)
	}
	if roleID != tenantRoleID.String() {
		t.Fatalf("role_id = %s, want %s", roleID, tenantRoleID.String())
	}
}
