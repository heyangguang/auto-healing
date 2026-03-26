package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestLoginReturnsPersistenceErrorWhenFailedLoginUpdateFails(t *testing.T) {
	db := openAuthTestDB(t)
	createAuthLoginTables(t, db)
	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	mustExecAuthTest(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, uuid.NewString(), "login-user", "login@example.com", passwordHash, "active", false)
	svc := &Service{userRepo: repository.NewUserRepositoryWithDB(db)}

	_, err = svc.Login(context.Background(), &LoginRequest{Username: "login-user", Password: "wrong-password"}, "127.0.0.1")
	if err == nil {
		t.Fatal("Login() error = nil, want failed_login_count persistence error")
	}
	if strings.Contains(err.Error(), ErrInvalidCredentials.Error()) {
		t.Fatalf("Login() error = %v, want infrastructure error", err)
	}
}

func TestLoginReturnsPersistenceErrorWhenSuccessfulLoginUpdateFails(t *testing.T) {
	db := openAuthTestDB(t)
	createAuthLoginTables(t, db)
	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	mustExecAuthTest(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, uuid.NewString(), "login-user", "login@example.com", passwordHash, "active", false)
	svc := &Service{userRepo: repository.NewUserRepositoryWithDB(db)}

	_, err = svc.Login(context.Background(), &LoginRequest{Username: "login-user", Password: "correct-password"}, "127.0.0.1")
	if err == nil {
		t.Fatal("Login() error = nil, want login persistence error")
	}
	if strings.Contains(err.Error(), ErrInvalidCredentials.Error()) {
		t.Fatalf("Login() error = %v, want infrastructure error", err)
	}
}

func createAuthLoginTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecAuthTest(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			status TEXT,
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
	mustExecAuthTest(t, db, `CREATE TABLE user_platform_roles (user_id TEXT, role_id TEXT);`)
	mustExecAuthTest(t, db, `
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
	mustExecAuthTest(t, db, `CREATE TABLE role_permissions (role_id TEXT, permission_id TEXT);`)
}
