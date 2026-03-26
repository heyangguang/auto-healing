package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestRoleRepositoryAssignPermissionsRejectsPlatformRoleInTenantContext(t *testing.T) {
	db := newStateTestDB(t)
	createRolePermissionSchema(t, db)
	repo := NewRoleRepositoryWithDB(db)
	tenantID := uuid.New()
	platformRoleID := uuid.New()
	permissionID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
		platformRoleID.String(), "platform_admin", "Platform Admin", true, "platform", now, now)
	mustExec(t, db, `INSERT INTO permissions (id, code, name, module, resource, action, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		permissionID.String(), "tenant:roles:assign", "Assign", "tenant", "roles", "assign", now)

	err := repo.AssignPermissions(WithTenantID(context.Background(), tenantID), platformRoleID, []uuid.UUID{permissionID})
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("AssignPermissions() error = %v, want %v", err, ErrRoleNotFound)
	}
}

func TestRoleRepositoryAssignPermissionsRejectsSystemTenantRole(t *testing.T) {
	db := newStateTestDB(t)
	createRolePermissionSchema(t, db)
	repo := NewRoleRepositoryWithDB(db)
	tenantID := uuid.New()
	roleID := uuid.New()
	permissionID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`,
		roleID.String(), "viewer", "Viewer", true, "tenant", now, now)
	mustExec(t, db, `INSERT INTO permissions (id, code, name, module, resource, action, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		permissionID.String(), "tenant:roles:view", "View", "tenant", "roles", "view", now)

	err := repo.AssignPermissions(WithTenantID(context.Background(), tenantID), roleID, []uuid.UUID{permissionID})
	if err == nil || err.Error() != "系统内置角色的权限不允许修改" {
		t.Fatalf("AssignPermissions() error = %v, want system-role mutation error", err)
	}
}

func TestRoleRepositoryAssignPermissionsAllowsTenantCustomRole(t *testing.T) {
	db := newStateTestDB(t)
	createRolePermissionSchema(t, db)
	repo := NewRoleRepositoryWithDB(db)
	tenantID := uuid.New()
	roleID := uuid.New()
	permissionA := uuid.New()
	permissionB := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO roles (id, name, display_name, is_system, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		roleID.String(), "ops-reviewer", "Ops Reviewer", false, "tenant", tenantID.String(), now, now)
	mustExec(t, db, `INSERT INTO permissions (id, code, name, module, resource, action, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		permissionA.String(), "tenant:roles:view", "View", "tenant", "roles", "view", now)
	mustExec(t, db, `INSERT INTO permissions (id, code, name, module, resource, action, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		permissionB.String(), "tenant:roles:assign", "Assign", "tenant", "roles", "assign", now)

	if err := repo.AssignPermissions(WithTenantID(context.Background(), tenantID), roleID, []uuid.UUID{permissionA, permissionB}); err != nil {
		t.Fatalf("AssignPermissions() error = %v", err)
	}

	var count int64
	if err := db.Table("role_permissions").Where("role_id = ?", roleID.String()).Count(&count).Error; err != nil {
		t.Fatalf("count role_permissions error = %v", err)
	}
	if count != 2 {
		t.Fatalf("role_permissions count = %d, want 2", count)
	}
}

func createRolePermissionSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
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
}
