package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type middlewareBlacklistStore struct{}

func (middlewareBlacklistStore) Add(context.Context, string, time.Time) error { return nil }

func (middlewareBlacklistStore) Exists(context.Context, string) (bool, error) { return false, nil }

type middlewareErrorBlacklistStore struct{}

func (middlewareErrorBlacklistStore) Add(context.Context, string, time.Time) error { return nil }

func (middlewareErrorBlacklistStore) Exists(context.Context, string) (bool, error) {
	return false, errors.New("redis down")
}

func TestAllowQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "site message events", path: "/api/v1/tenant/site-messages/events", want: true},
		{name: "execution stream", path: "/api/v1/tenant/execution-runs/123/stream", want: true},
		{name: "healing events", path: "/api/v1/tenant/healing/instances/123/events", want: true},
		{name: "normal api", path: "/api/v1/common/search", want: false},
		{name: "auth refresh", path: "/api/v1/auth/refresh", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest("GET", "http://example.com"+tt.path+"?token=abc", nil)
			c.Request = req

			if got := allowQueryToken(c); got != tt.want {
				t.Fatalf("allowQueryToken(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestEnsureActiveUserReturnsInternalErrorWhenLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	db := newMiddlewareTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/secure", nil)

	if ensureActiveUser(c, uuid.NewString()) {
		t.Fatal("ensureActiveUser() = true, want false")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeAccountLookup {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeAccountLookup)
	}
}

func TestEnsureActiveUserReturnsUnauthorizedForInactiveUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	db := newMiddlewareTestDB(t)
	createUserLookupSchema(t, db)
	userID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, 'inactive-user', 'inactive@example.com', 'hashed', 'inactive', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, false)
	`, userID.String())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/secure", nil)

	if ensureActiveUser(c, userID.String()) {
		t.Fatal("ensureActiveUser() = true, want false")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var resp middlewareErrorResponse
	if err := decodeMiddlewareError(recorder, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeAccountDisabled {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeAccountDisabled)
	}
}

func TestJWTAuthRejectsRefreshTokenOnProtectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	db := newMiddlewareTestDB(t)
	createUserLookupSchema(t, db)
	userID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, 'active-user', 'active@example.com', 'hashed', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, false)
	`, userID.String())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "middleware-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "middleware-test",
	}, middlewareBlacklistStore{})
	pair, err := jwtSvc.GenerateTokenPair(userID.String(), "active-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	router := gin.New()
	router.GET("/secure", JWTAuth(jwtSvc), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set(AuthorizationHeader, BearerPrefix+pair.RefreshToken)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuthReturnsInternalErrorWhenBlacklistLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	db := newMiddlewareTestDB(t)
	createUserLookupSchema(t, db)
	userID := uuid.New()
	mustExecMiddlewareSQL(t, db, `
		INSERT INTO users (id, username, email, password_hash, status, created_at, updated_at, is_platform_admin)
		VALUES (?, 'active-user', 'active@example.com', 'hashed', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, false)
	`, userID.String())
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "middleware-test",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "middleware-test",
	}, middlewareErrorBlacklistStore{})
	pair, err := jwtSvc.GenerateTokenPair(userID.String(), "active-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	router := gin.New()
	router.GET("/secure", JWTAuth(jwtSvc), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set(AuthorizationHeader, BearerPrefix+pair.AccessToken)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func newMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "middleware.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createUserLookupSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecMiddlewareSQL(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			is_platform_admin BOOLEAN NOT NULL DEFAULT FALSE
		);
	`)
	mustExecMiddlewareSQL(t, db, `CREATE TABLE roles (id TEXT PRIMARY KEY NOT NULL, name TEXT, display_name TEXT, description TEXT, is_system BOOLEAN, scope TEXT, tenant_id TEXT, created_at DATETIME, updated_at DATETIME);`)
	mustExecMiddlewareSQL(t, db, `CREATE TABLE permissions (id TEXT PRIMARY KEY NOT NULL, code TEXT, name TEXT, description TEXT, module TEXT, resource TEXT, action TEXT, created_at DATETIME);`)
	mustExecMiddlewareSQL(t, db, `CREATE TABLE user_platform_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, role_id TEXT, created_at DATETIME);`)
	mustExecMiddlewareSQL(t, db, `CREATE TABLE user_tenant_roles (id TEXT PRIMARY KEY NOT NULL, user_id TEXT, tenant_id TEXT, role_id TEXT, created_at DATETIME);`)
	mustExecMiddlewareSQL(t, db, `CREATE TABLE role_permissions (id TEXT PRIMARY KEY NOT NULL, role_id TEXT, permission_id TEXT, created_at DATETIME);`)
}

func mustExecMiddlewareSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql: %v\nsql=%s", err, sql)
	}
}
