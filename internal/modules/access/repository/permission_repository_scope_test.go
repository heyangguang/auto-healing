package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestPermissionRepositoryListTenantWithFilterExcludesPlatformPermissions(t *testing.T) {
	db := newStateTestDB(t)
	createPermissionScopeSchema(t, db)
	repo := NewPermissionRepositoryWithDB(db)
	now := time.Now().UTC().Format(time.RFC3339)

	insertPermissionRow(t, db, uuid.New(), "platform:users:list", "List Users", "platform", "users", "list", now)
	insertPermissionRow(t, db, uuid.New(), "user:list", "List Tenant Users", "tenant", "users", "list", now)

	perms, err := repo.ListTenantWithFilter(context.Background(), PermissionFilter{})
	if err != nil {
		t.Fatalf("ListTenantWithFilter() error = %v", err)
	}
	if len(perms) != 1 || perms[0].Code != "user:list" {
		t.Fatalf("ListTenantWithFilter() codes = %v, want [user:list]", permissionCodes(perms))
	}
}

func TestPermissionRepositoryGetTenantPermissionCodesFiltersPlatformPermissions(t *testing.T) {
	db := newStateTestDB(t)
	createPermissionScopeSchema(t, db)
	repo := NewPermissionRepositoryWithDB(db)
	tenantID := uuid.New()
	userID := uuid.New()
	tenantRoleID := uuid.New()
	tenantPermID := uuid.New()
	platformPermID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	insertPermissionRow(t, db, tenantPermID, "user:list", "List Tenant Users", "tenant", "users", "list", now)
	insertPermissionRow(t, db, platformPermID, "platform:users:list", "List Users", "platform", "users", "list", now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tenantRoleID.String(), "ops-reviewer", "Ops Reviewer", false, "tenant", tenantID.String(), now, now)
	mustExec(t, db, `INSERT INTO role_permissions (id, role_id, permission_id, created_at) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), tenantRoleID.String(), tenantPermID.String(), now)
	mustExec(t, db, `INSERT INTO role_permissions (id, role_id, permission_id, created_at) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), tenantRoleID.String(), platformPermID.String(), now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), tenantID.String(), tenantRoleID.String(), now)

	codes, err := repo.GetTenantPermissionCodes(context.Background(), userID, tenantID)
	if err != nil {
		t.Fatalf("GetTenantPermissionCodes() error = %v", err)
	}
	if len(codes) != 1 || codes[0] != "user:list" {
		t.Fatalf("GetTenantPermissionCodes() = %v, want [user:list]", codes)
	}
}

func TestPermissionRepositoryGetUserPermissionsExcludesDisabledTenantRoles(t *testing.T) {
	db := newStateTestDB(t)
	createPermissionScopeSchema(t, db)
	repo := NewPermissionRepositoryWithDB(db)
	userID := uuid.New()
	activeTenantID := uuid.New()
	disabledTenantID := uuid.New()
	activeRoleID := uuid.New()
	disabledRoleID := uuid.New()
	activePermID := uuid.New()
	disabledPermID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	insertPermissionRow(t, db, activePermID, "user:list", "List Tenant Users", "tenant", "users", "list", now)
	insertPermissionRow(t, db, disabledPermID, "role:assign", "Assign Role", "tenant", "roles", "assign", now)
	mustExec(t, db, `INSERT INTO tenants (id, name, code, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		activeTenantID.String(), "Tenant A", "tenant-a", "active", now, now)
	mustExec(t, db, `INSERT INTO tenants (id, name, code, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		disabledTenantID.String(), "Tenant B", "tenant-b", "disabled", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		activeRoleID.String(), "active-operator", "Active Operator", false, "tenant", activeTenantID.String(), now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		disabledRoleID.String(), "disabled-admin", "Disabled Admin", false, "tenant", disabledTenantID.String(), now, now)
	mustExec(t, db, `INSERT INTO role_permissions (id, role_id, permission_id, created_at) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), activeRoleID.String(), activePermID.String(), now)
	mustExec(t, db, `INSERT INTO role_permissions (id, role_id, permission_id, created_at) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), disabledRoleID.String(), disabledPermID.String(), now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), activeTenantID.String(), activeRoleID.String(), now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), disabledTenantID.String(), disabledRoleID.String(), now)

	perms, err := repo.GetUserPermissions(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserPermissions() error = %v", err)
	}
	if len(perms) != 1 || perms[0].Code != "user:list" {
		t.Fatalf("GetUserPermissions() codes = %v, want [user:list]", permissionCodes(perms))
	}
}

func createPermissionScopeSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	createRolePermissionSchema(t, db)
	mustExec(t, db, `
		CREATE TABLE tenants (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE user_platform_roles (
			id TEXT PRIMARY KEY NOT NULL,
			user_id TEXT NOT NULL,
			role_id TEXT NOT NULL,
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
}

func insertPermissionRow(t *testing.T, db *gorm.DB, permissionID uuid.UUID, code, name, module, resource, action, createdAt string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO permissions (id, code, name, module, resource, action, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		permissionID.String(), code, name, module, resource, action, createdAt)
}
