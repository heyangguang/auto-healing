package repository

import (
	"context"
	"testing"
	"time"

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
			status TEXT NOT NULL DEFAULT 'active',
			is_platform_admin BOOLEAN NOT NULL DEFAULT FALSE,
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
