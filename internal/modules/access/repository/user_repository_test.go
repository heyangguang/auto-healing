package repository

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/access/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func createUserRoleSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL DEFAULT '',
			display_name TEXT,
			phone TEXT,
			avatar_url TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			is_platform_admin BOOLEAN NOT NULL DEFAULT FALSE,
			last_login_at DATETIME,
			last_login_ip TEXT,
			password_changed_at DATETIME,
			failed_login_count INTEGER NOT NULL DEFAULT 0,
			locked_until DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE user_platform_roles (
			user_id TEXT NOT NULL,
			role_id TEXT NOT NULL
		);
	`)
}

func TestAssignRolesUpdatesPlatformAdminFlag(t *testing.T) {
	db := newSQLiteTestDB(t)
	createUserRoleSchema(t, db)

	repo := &UserRepository{db: db}
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mustExec(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, is_platform_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID.String(), "platform-user", "platform@example.com", "hash", "active", false, time.Now(), time.Now())

	roleID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if err := repo.AssignRoles(context.Background(), userID, []uuid.UUID{roleID}); err != nil {
		t.Fatalf("AssignRoles add role: %v", err)
	}

	var flag bool
	if err := db.Table("users").Select("is_platform_admin").Where("id = ?", userID.String()).Scan(&flag).Error; err != nil {
		t.Fatalf("read is_platform_admin: %v", err)
	}
	if !flag {
		t.Fatalf("is_platform_admin should be true after assigning platform role")
	}

	if err := repo.AssignRoles(context.Background(), userID, nil); err != nil {
		t.Fatalf("AssignRoles clear roles: %v", err)
	}
	if err := db.Table("users").Select("is_platform_admin").Where("id = ?", userID.String()).Scan(&flag).Error; err != nil {
		t.Fatalf("read is_platform_admin after clear: %v", err)
	}
	if flag {
		t.Fatalf("is_platform_admin should be false after clearing platform roles")
	}
}

func TestUpdatePlatformUserWithRoleReplacesExistingPlatformRole(t *testing.T) {
	db := newSQLiteTestDB(t)
	createUserRoleSchema(t, db)

	repo := &UserRepository{db: db}
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	oldRoleID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	newRoleID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	mustExec(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, is_platform_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID.String(), "platform-user", "platform@example.com", "hash", "active", false, time.Now(), time.Now())
	mustExec(t, db, `INSERT INTO user_platform_roles (user_id, role_id) VALUES (?, ?)`, userID.String(), oldRoleID.String())

	user := &model.User{
		ID:              userID,
		Username:        "platform-user",
		Email:           "platform@example.com",
		PasswordHash:    "hash",
		DisplayName:     "updated",
		Status:          "active",
		IsPlatformAdmin: false,
	}

	if err := repo.UpdatePlatformUserWithRole(context.Background(), user, &newRoleID); err != nil {
		t.Fatalf("UpdatePlatformUserWithRole: %v", err)
	}

	var roles []string
	if err := db.Table("user_platform_roles").Select("role_id").Where("user_id = ?", userID.String()).Order("role_id").Scan(&roles).Error; err != nil {
		t.Fatalf("read platform roles: %v", err)
	}
	if len(roles) != 1 || roles[0] != newRoleID.String() {
		t.Fatalf("platform roles = %v, want [%s]", roles, newRoleID)
	}

	var flag bool
	if err := db.Table("users").Select("is_platform_admin").Where("id = ?", userID.String()).Scan(&flag).Error; err != nil {
		t.Fatalf("read is_platform_admin: %v", err)
	}
	if !flag {
		t.Fatalf("is_platform_admin should be true after updating platform role")
	}
}
