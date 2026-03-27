package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/crypto"
	authjwt "github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type authServiceTestBlacklistStore struct{}

func (authServiceTestBlacklistStore) Add(context.Context, string, time.Time) error { return nil }

func (authServiceTestBlacklistStore) Exists(context.Context, string) (bool, error) { return false, nil }

func TestLoginReturnsPersistenceErrorWhenFailedLoginUpdateFails(t *testing.T) {
	db := openAuthTestDB(t)
	createAuthLoginTablesMissingFailedLoginCount(t, db)
	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	mustExecAuthTest(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, uuid.NewString(), "login-user", "login@example.com", passwordHash, "active", false)
	svc := &Service{userRepo: accessrepo.NewUserRepositoryWithDB(db), db: db}

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
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })
	createAuthLoginTablesMissingSuccessfulLoginFields(t, db)
	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	mustExecAuthTest(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, uuid.NewString(), "login-user", "login@example.com", passwordHash, "active", false)
	svc := &Service{
		userRepo:   accessrepo.NewUserRepositoryWithDB(db),
		permRepo:   accessrepo.NewPermissionRepository(),
		tenantRepo: accessrepo.NewTenantRepositoryWithDB(db),
		jwtSvc: authjwt.NewService(authjwt.Config{
			Secret:          "login-test",
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: time.Hour,
			Issuer:          "login-test",
		}, authServiceTestBlacklistStore{}),
		db: db,
	}

	_, err = svc.Login(context.Background(), &LoginRequest{Username: "login-user", Password: "correct-password"}, "127.0.0.1")
	if err == nil {
		t.Fatal("Login() error = nil, want login persistence error")
	}
	if strings.Contains(err.Error(), ErrInvalidCredentials.Error()) {
		t.Fatalf("Login() error = %v, want infrastructure error", err)
	}
}

func TestLoginExpiredLockDoesNotStampSuccessfulLoginBeforePasswordCheck(t *testing.T) {
	db := openAuthTestDB(t)
	createAuthLoginTablesWithLoginState(t, db)
	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	userID := uuid.New()
	lastLoginAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	expiredLockAt := time.Now().Add(-time.Hour)
	mustExecAuthTest(t, db, `
		INSERT INTO users (
			id, username, email, password_hash, status, locked_until, last_login_at,
			failed_login_count, created_at, updated_at, is_platform_admin
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, userID.String(), "locked-user", "locked@example.com", passwordHash, "locked", expiredLockAt, lastLoginAt, 3, false)

	svc := &Service{userRepo: accessrepo.NewUserRepositoryWithDB(db), db: db}

	if _, err := svc.Login(context.Background(), &LoginRequest{
		Username: "locked-user",
		Password: "wrong-password",
	}, "127.0.0.1"); err != ErrInvalidCredentials {
		t.Fatalf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}

	var got struct {
		Status           string
		FailedLoginCount int
		LastLoginAt      *time.Time
	}
	if err := db.Table("users").
		Select("status, failed_login_count, last_login_at").
		Where("id = ?", userID.String()).
		Scan(&got).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("status = %q, want %q", got.Status, "active")
	}
	if got.FailedLoginCount != 1 {
		t.Fatalf("failed_login_count = %d, want %d", got.FailedLoginCount, 1)
	}
	if got.LastLoginAt == nil || !got.LastLoginAt.Equal(lastLoginAt) {
		t.Fatalf("last_login_at = %v, want %v", got.LastLoginAt, lastLoginAt)
	}
}

func TestLoginDoesNotStampSuccessfulLoginBeforeAccessResolution(t *testing.T) {
	db := openAuthTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })
	createAuthLoginTablesWithLoginState(t, db)

	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	userID := uuid.New()
	mustExecAuthTest(t, db, `
		INSERT INTO users (
			id, username, email, password_hash, status, failed_login_count, created_at, updated_at, is_platform_admin
		) VALUES (?, ?, ?, ?, 'active', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, userID.String(), "login-user", "login@example.com", passwordHash, false)
	mustExecAuthTest(t, db, `DROP TABLE permissions;`)

	svc := &Service{
		userRepo:   accessrepo.NewUserRepositoryWithDB(db),
		roleRepo:   accessrepo.NewRoleRepositoryWithDB(db),
		permRepo:   accessrepo.NewPermissionRepository(),
		tenantRepo: accessrepo.NewTenantRepositoryWithDB(db),
		db:         db,
	}

	if _, err := svc.Login(context.Background(), &LoginRequest{
		Username: "login-user",
		Password: "correct-password",
	}, "127.0.0.1"); err == nil {
		t.Fatal("Login() error = nil, want access resolution failure")
	}

	var got struct {
		LastLoginAt      *time.Time
		FailedLoginCount int
	}
	if err := db.Table("users").
		Select("last_login_at, failed_login_count").
		Where("id = ?", userID.String()).
		Scan(&got).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if got.LastLoginAt != nil {
		t.Fatalf("last_login_at = %v, want nil", got.LastLoginAt)
	}
	if got.FailedLoginCount != 0 {
		t.Fatalf("failed_login_count = %d, want %d", got.FailedLoginCount, 0)
	}
}

func createAuthLoginTablesMissingSuccessfulLoginFields(t *testing.T, db *gorm.DB) {
	t.Helper()
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
			locked_until DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			is_platform_admin BOOLEAN
		);
	`)
	createAuthLoginAccessTables(t, db)
}

func createAuthLoginTablesWithLoginState(t *testing.T, db *gorm.DB) {
	t.Helper()
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
			locked_until DATETIME,
			last_login_at DATETIME,
			last_login_ip TEXT,
			password_changed_at DATETIME,
			failed_login_count INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			is_platform_admin BOOLEAN
		);
	`)
	createAuthLoginAccessTables(t, db)
}

func createAuthLoginTablesMissingFailedLoginCount(t *testing.T, db *gorm.DB) {
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
	createAuthLoginAccessTables(t, db)
}

func createAuthLoginAccessTables(t *testing.T, db *gorm.DB) {
	t.Helper()
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
	mustExecAuthTest(t, db, `CREATE TABLE user_tenant_roles (user_id TEXT, tenant_id TEXT, role_id TEXT);`)
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
