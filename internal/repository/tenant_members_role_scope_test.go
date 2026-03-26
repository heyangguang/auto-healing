package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

func TestTenantRepositoryGetMemberReturnsErrUserNotFoundWhenMissing(t *testing.T) {
	db := newStateTestDB(t)
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
	_, err := repo.GetMember(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("GetMember() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestTenantRepositoryGetUserAllRolesExcludesDisabledTenants(t *testing.T) {
	db := newStateTestDB(t)
	createTenantRoleAggregationSchema(t, db)
	repo := &TenantRepository{db: db}
	userID := uuid.New()
	activeTenantID := uuid.New()
	disabledTenantID := uuid.New()
	activeRoleID := uuid.New()
	disabledRoleID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO tenants (id, name, code, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		activeTenantID.String(), "Tenant A", "tenant-a", "active", now, now)
	mustExec(t, db, `INSERT INTO tenants (id, name, code, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		disabledTenantID.String(), "Tenant B", "tenant-b", "disabled", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		activeRoleID.String(), "active-operator", "Active Operator", false, "tenant", activeTenantID.String(), now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		disabledRoleID.String(), "disabled-admin", "Disabled Admin", false, "tenant", disabledTenantID.String(), now, now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), activeTenantID.String(), activeRoleID.String(), now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), disabledTenantID.String(), disabledRoleID.String(), now)

	roles, err := repo.GetUserAllRoles(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserAllRoles() error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != activeRoleID {
		t.Fatalf("GetUserAllRoles() = %+v, want only active tenant role %s", roles, activeRoleID)
	}
}

func TestTenantRepositoryListMembersReturnsErrorWhenUserLoadFails(t *testing.T) {
	db := newStateTestDB(t)
	createTenantMemberListSchemaWithoutUsers(t, db)
	repo := &TenantRepository{db: db}
	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO tenants (id, name, code, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		tenantID.String(), "Tenant A", "tenant-a", "active", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
		roleID.String(), "viewer", "Viewer", true, "tenant", now, now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), tenantID.String(), roleID.String(), now)

	_, err := repo.ListMembers(context.Background(), tenantID)
	if err == nil {
		t.Fatalf("ListMembers() error = nil, want user load failure")
	}
}

func TestTenantRepositoryListMembersReturnsErrorWhenUserAssociationMissing(t *testing.T) {
	db := newStateTestDB(t)
	createTenantMemberListSchemaWithUsersTable(t, db)
	repo := &TenantRepository{db: db}
	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO tenants (id, name, code, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		tenantID.String(), "Tenant A", "tenant-a", "active", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
		roleID.String(), "viewer", "Viewer", true, "tenant", now, now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), tenantID.String(), roleID.String(), now)

	_, err := repo.ListMembers(context.Background(), tenantID)
	if !errors.Is(err, ErrTenantMemberAssociationCorrupted) {
		t.Fatalf("ListMembers() error = %v, want %v", err, ErrTenantMemberAssociationCorrupted)
	}
}

func TestTenantRepositoryUpdateMemberRoleReturnsErrUserNotFoundWhenRowMissing(t *testing.T) {
	db := newStateTestDB(t)
	createTenantRoleAggregationSchema(t, db)
	repo := &TenantRepository{db: db}
	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		roleID.String(), "ops-reviewer", "Ops Reviewer", false, "tenant", tenantID.String(), now, now)

	err := repo.UpdateMemberRole(context.Background(), userID, tenantID, roleID)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("UpdateMemberRole() error = %v, want %v", err, ErrUserNotFound)
	}
}

func createTenantRoleAggregationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
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
}

func createTenantMemberListSchemaWithoutUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	createTenantRoleAggregationSchema(t, db)
}

func createTenantMemberListSchemaWithUsersTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	createTenantRoleAggregationSchema(t, db)
	mustExec(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			display_name TEXT,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}
