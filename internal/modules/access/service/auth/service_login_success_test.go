package auth

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/crypto"
	authjwt "github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestLoginReturnsTokenPairAndResetsLoginState(t *testing.T) {
	db := openAuthTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })
	createAuthLoginTablesWithLoginState(t, db)
	createAuthLoginTenantTables(t, db)

	passwordHash, err := crypto.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password error = %v", err)
	}
	userID := uuid.New()
	tenantID := uuid.New()
	insertAuthLoginUser(t, db, userID, passwordHash)
	insertAuthLoginTenant(t, db, tenantID)
	insertAuthLoginTenantMembership(t, db, userID, tenantID)

	jwtSvc := authjwt.NewService(authjwt.Config{
		Secret:          "login-success-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "login-success-test",
	}, authServiceTestBlacklistStore{})
	svc := &Service{
		userRepo:   accessrepo.NewUserRepositoryWithDB(db),
		roleRepo:   accessrepo.NewRoleRepositoryWithDB(db),
		permRepo:   accessrepo.NewPermissionRepositoryWithDB(db),
		tenantRepo: accessrepo.NewTenantRepositoryWithDB(db),
		jwtSvc:     jwtSvc,
		db:         db,
	}

	resp, err := svc.Login(context.Background(), &LoginRequest{
		Username: "login-user",
		Password: "correct-password",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("tokens = (%q, %q), want non-empty", resp.AccessToken, resp.RefreshToken)
	}
	if resp.CurrentTenantID != tenantID.String() {
		t.Fatalf("current_tenant_id = %q, want %q", resp.CurrentTenantID, tenantID.String())
	}
	if len(resp.Tenants) != 1 || resp.Tenants[0].ID != tenantID.String() {
		t.Fatalf("tenants = %+v, want tenant %q", resp.Tenants, tenantID)
	}

	claims, err := jwtSvc.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.DefaultTenantID != tenantID.String() {
		t.Fatalf("default_tenant_id = %q, want %q", claims.DefaultTenantID, tenantID.String())
	}

	var got struct {
		Status           string
		FailedLoginCount int
		LastLoginIP      string
		LastLoginAt      *time.Time
	}
	if err := db.Table("users").
		Select("status, failed_login_count, last_login_ip, last_login_at").
		Where("id = ?", userID.String()).
		Scan(&got).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if got.Status != "active" || got.FailedLoginCount != 0 || got.LastLoginIP != "127.0.0.1" || got.LastLoginAt == nil {
		t.Fatalf("login state = %+v, want active/reset state with last login stamp", got)
	}
}

func createAuthLoginTenantTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecAuthTest(t, db, `
		CREATE TABLE tenants (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			description TEXT,
			icon TEXT,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func insertAuthLoginUser(t *testing.T, db *gorm.DB, userID uuid.UUID, passwordHash string) {
	t.Helper()
	mustExecAuthTest(t, db, `
		INSERT INTO users (
			id, username, email, password_hash, display_name, status, failed_login_count,
			created_at, updated_at, is_platform_admin
		) VALUES (?, ?, ?, ?, ?, 'active', 3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`, userID.String(), "login-user", "login@example.com", passwordHash, "Login User", false)
}

func insertAuthLoginTenant(t *testing.T, db *gorm.DB, tenantID uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	mustExecAuthTest(t, db, `
		INSERT INTO tenants (id, name, code, description, icon, status, created_at, updated_at)
		VALUES (?, ?, ?, '', '', 'active', ?, ?)
	`, tenantID.String(), "Tenant A", "tenant-a", now, now)
}

func insertAuthLoginTenantMembership(t *testing.T, db *gorm.DB, userID, tenantID uuid.UUID) {
	t.Helper()
	mustExecAuthTest(t, db, `
		INSERT INTO user_tenant_roles (user_id, tenant_id, role_id)
		VALUES (?, ?, ?)
	`, userID.String(), tenantID.String(), uuid.NewString())
}
