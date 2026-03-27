package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequireServerDBPanicsOnNil(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if strings.Contains(recovered.(string), "server router requires explicit db") {
				return
			}
			t.Fatalf("panic = %v, want explicit db panic", recovered)
		}
		t.Fatal("panic = nil, want explicit db panic")
	}()
	requireServerDB(nil, "server router")
}

func TestNewRouterWithDBRegistersHealthAndAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newServerTestConfig()
	config.SetGlobalConfig(cfg)
	router := newRouterWithDB(cfg, openServerTestDB(t))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("/health body = %q, want status ok", recorder.Body.String())
	}

	routes := router.Routes()
	assertServerRouteExists(t, routes, "POST", "/api/v1/auth/login")
	assertServerRoutePrefixExists(t, routes, "/api/v1/common/")
	assertServerRoutePrefixExists(t, routes, "/api/v1/platform/")
	assertServerRoutePrefixExists(t, routes, "/api/v1/tenant/")
}

func TestNewRouterWithDBNullPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newServerTestConfig()
	config.SetGlobalConfig(cfg)

	defer func() {
		if recovered := recover(); recovered != nil {
			if strings.Contains(recovered.(string), "server router requires explicit db") {
				return
			}
			t.Fatalf("panic = %v, want explicit db panic", recovered)
		}
		t.Fatal("panic = nil, want explicit db panic")
	}()

	_ = newRouterWithDB(cfg, nil)
}

func newServerTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		JWT: config.JWTConfig{
			Secret:                "server-router-test-secret",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLHours:  24,
			Issuer:                "server-router-test",
		},
	}
}

func openServerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "server-router.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE sys_dictionaries (
			id TEXT PRIMARY KEY,
			dict_type TEXT NOT NULL,
			dict_key TEXT NOT NULL,
			label TEXT NOT NULL,
			sort_order INTEGER DEFAULT 0,
			is_active BOOLEAN DEFAULT 1
		)
	`).Error; err != nil {
		t.Fatalf("create sys_dictionaries: %v", err)
	}
	return db
}

func assertServerRouteExists(t *testing.T, routes gin.RoutesInfo, method, path string) {
	t.Helper()

	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("route %s %s not registered", method, path)
}

func assertServerRoutePrefixExists(t *testing.T, routes gin.RoutesInfo, prefix string) {
	t.Helper()

	for _, route := range routes {
		if strings.HasPrefix(route.Path, prefix) {
			return
		}
	}
	t.Fatalf("route prefix %s not registered", prefix)
}
