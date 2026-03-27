package auth

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/google/uuid"
)

func TestGetCurrentUserReturnsErrorWhenPermissionQueryFails(t *testing.T) {
	db := openAuthTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })
	mustExecAuthTest(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT,
			phone TEXT,
			avatar_url TEXT,
			status TEXT,
			last_login_at DATETIME,
			last_login_ip TEXT,
			password_changed_at DATETIME,
			failed_login_count INTEGER,
			locked_until DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			is_platform_admin BOOLEAN
		);
	`)
	mustExecAuthTest(t, db, `
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
	mustExecAuthTest(t, db, `CREATE TABLE user_platform_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, role_id TEXT, created_at DATETIME);`)
	mustExecAuthTest(t, db, `CREATE TABLE user_tenant_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, tenant_id TEXT, role_id TEXT, created_at DATETIME);`)
	user := &model.User{
		ID:           uuid.New(),
		Username:     "profile-user",
		Email:        "profile@example.com",
		PasswordHash: "hashed",
		Status:       "active",
	}
	if err := accessrepo.NewUserRepositoryWithDB(db).Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := &Service{
		userRepo:   accessrepo.NewUserRepositoryWithDB(db),
		roleRepo:   accessrepo.NewRoleRepositoryWithDB(db),
		permRepo:   accessrepo.NewPermissionRepositoryWithDB(db),
		tenantRepo: accessrepo.NewTenantRepositoryWithDB(db),
	}

	if _, err := svc.GetCurrentUser(context.Background(), user.ID); err == nil {
		t.Fatal("GetCurrentUser() error = nil, want permission query failure")
	}
}
