package handler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type authMeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID              uuid.UUID `json:"id"`
		Username        string    `json:"username"`
		Roles           []string  `json:"roles"`
		Permissions     []string  `json:"permissions"`
		IsPlatformAdmin bool      `json:"is_platform_admin"`
	} `json:"data"`
}

func newAuthHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth-handler.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExecAuthSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

var authHandlerSchemaStatements = []string{
	`CREATE TABLE users (
		id TEXT PRIMARY KEY NOT NULL,
		username TEXT NOT NULL,
		email TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		display_name TEXT,
		phone TEXT,
		avatar_url TEXT,
		status TEXT NOT NULL,
		last_login_at DATETIME,
		last_login_ip TEXT,
		password_changed_at DATETIME,
		failed_login_count INTEGER DEFAULT 0,
		locked_until DATETIME,
		created_at DATETIME,
		updated_at DATETIME,
		is_platform_admin BOOLEAN NOT NULL DEFAULT FALSE
	);`,
	`CREATE TABLE roles (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		display_name TEXT NOT NULL,
		description TEXT,
		is_system BOOLEAN NOT NULL DEFAULT FALSE,
		scope TEXT NOT NULL DEFAULT 'tenant',
		tenant_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);`,
	`CREATE TABLE permissions (
		id TEXT PRIMARY KEY NOT NULL,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		module TEXT NOT NULL,
		resource TEXT NOT NULL,
		action TEXT NOT NULL,
		created_at DATETIME
	);`,
	`CREATE TABLE user_platform_roles (
		id TEXT PRIMARY KEY NOT NULL,
		user_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		created_at DATETIME
	);`,
	`CREATE TABLE user_tenant_roles (
		id TEXT PRIMARY KEY NOT NULL,
		user_id TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		created_at DATETIME
	);`,
	`CREATE TABLE role_permissions (
		id TEXT PRIMARY KEY NOT NULL,
		role_id TEXT NOT NULL,
		permission_id TEXT NOT NULL,
		created_at DATETIME
	);`,
	`CREATE TABLE tenants (
		id TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		code TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME
	);`,
	`CREATE TABLE impersonation_requests (
		id TEXT PRIMARY KEY NOT NULL,
		requester_id TEXT NOT NULL,
		requester_name TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		tenant_name TEXT NOT NULL,
		reason TEXT,
		duration_minutes INTEGER NOT NULL,
		status TEXT NOT NULL,
		approved_by TEXT,
		approved_at DATETIME,
		session_started_at DATETIME,
		session_expires_at DATETIME,
		completed_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);`,
	`CREATE TABLE audit_logs (
		id TEXT PRIMARY KEY NOT NULL,
		tenant_id TEXT,
		user_id TEXT,
		username TEXT,
		ip_address TEXT,
		user_agent TEXT,
		category TEXT NOT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT,
		resource_name TEXT,
		request_method TEXT,
		request_path TEXT,
		request_body TEXT,
		response_status INTEGER,
		changes TEXT,
		status TEXT NOT NULL,
		error_message TEXT,
		created_at DATETIME
	);`,
	`CREATE TABLE platform_audit_logs (
		id TEXT PRIMARY KEY NOT NULL,
		user_id TEXT,
		username TEXT,
		ip_address TEXT,
		user_agent TEXT,
		category TEXT NOT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT,
		resource_name TEXT,
		request_method TEXT,
		request_path TEXT,
		request_body TEXT,
		response_status INTEGER,
		changes TEXT,
		status TEXT NOT NULL,
		error_message TEXT,
		created_at DATETIME
	);`,
}

func createAuthHandlerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, statement := range authHandlerSchemaStatements {
		mustExecAuthSQL(t, db, statement)
	}
}

func insertUser(t *testing.T, db *gorm.DB, id uuid.UUID, username string, isPlatformAdmin bool) {
	t.Helper()
	now := time.Now().UTC()
	mustExecAuthSQL(t, db, `
		INSERT INTO users (
			id, username, email, password_hash, display_name, status, password_changed_at, failed_login_count,
			created_at, updated_at, is_platform_admin
		) VALUES (?, ?, ?, ?, ?, 'active', ?, 0, ?, ?, ?)
	`, id.String(), username, username+"@example.com", "hashed-password", username, now, now, now, isPlatformAdmin)
}

func insertTenant(t *testing.T, db *gorm.DB, id uuid.UUID, name, code string) {
	t.Helper()
	now := time.Now().UTC()
	mustExecAuthSQL(t, db, `
		INSERT INTO tenants (id, name, code, description, icon, status, created_at, updated_at)
		VALUES (?, ?, ?, '', '', ?, ?, ?)
	`, id.String(), name, code, model.TenantStatusActive, now, now)
}

func insertRole(t *testing.T, db *gorm.DB, id uuid.UUID, name, scope string) {
	t.Helper()
	now := time.Now().UTC()
	mustExecAuthSQL(t, db, `
		INSERT INTO roles (id, name, display_name, description, is_system, scope, created_at, updated_at)
		VALUES (?, ?, ?, '', 1, ?, ?, ?)
	`, id.String(), name, name, scope, now, now)
}

func insertPermission(t *testing.T, db *gorm.DB, id uuid.UUID, code string) {
	t.Helper()
	module, resource, action, createdAt := permissionSegments(code)
	mustExecAuthSQL(t, db, `
		INSERT INTO permissions (id, code, name, description, module, resource, action, created_at)
		VALUES (?, ?, ?, '', ?, ?, ?, ?)
	`, id.String(), code, code, module, resource, action, createdAt)
}

func permissionSegments(code string) (module string, resource string, action string, createdAt time.Time) {
	parts := splitPermissionCode(code)
	return parts[0], parts[1], parts[2], time.Now().UTC()
}

func splitPermissionCode(code string) [3]string {
	parts := [3]string{"misc", "misc", "read"}
	segments := [3]string{}
	n := 0
	start := 0
	for i := 0; i <= len(code); i++ {
		if i == len(code) || code[i] == ':' {
			if n < len(segments) {
				segments[n] = code[start:i]
			}
			n++
			start = i + 1
		}
	}
	switch n {
	case 1:
		parts[0] = segments[0]
	case 2:
		parts[0], parts[2] = segments[0], segments[1]
	default:
		parts[0], parts[1], parts[2] = segments[0], segments[1], segments[2]
	}
	return parts
}
