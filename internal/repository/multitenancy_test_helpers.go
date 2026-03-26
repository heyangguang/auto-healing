package repository

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func createDashboardSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE dashboard_configs (
			id TEXT PRIMARY KEY NOT NULL DEFAULT (
				lower(hex(randomblob(4))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(6)))
			),
			user_id TEXT NOT NULL,
			tenant_id TEXT,
			config TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `CREATE UNIQUE INDEX idx_dashboard_tenant_user ON dashboard_configs(user_id, tenant_id);`)
	mustExec(t, db, `CREATE UNIQUE INDEX idx_dashboard_configs_null_tenant_unique ON dashboard_configs(user_id) WHERE tenant_id IS NULL;`)
}

func createWorkspaceSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE system_workspaces (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			description TEXT,
			config TEXT NOT NULL DEFAULT '{}',
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE role_workspaces (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			role_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME
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

func createCMDBSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
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
			dependencies TEXT NOT NULL DEFAULT '[]',
			tags TEXT NOT NULL DEFAULT '{}',
			raw_data TEXT NOT NULL DEFAULT '{}',
			source_created_at DATETIME,
			source_updated_at DATETIME,
			maintenance_reason TEXT,
			maintenance_start_at DATETIME,
			maintenance_end_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func createDashboardUsersSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			display_name TEXT,
			status TEXT NOT NULL,
			last_login_at DATETIME,
			last_login_ip TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE roles (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			display_name TEXT NOT NULL,
			scope TEXT NOT NULL,
			tenant_id TEXT,
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
