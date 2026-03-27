package httpapi

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequireModulesDBPanicsOnNil(t *testing.T) {
	assertPanicContains(t, "http routes require explicit db", func() {
		requireModulesDB(nil)
	})
}

func TestNewModulesWithDBBuildsDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newHTTPAPITestConfig()
	config.SetGlobalConfig(cfg)
	modules := newModulesWithDB(cfg, openHTTPAPITestDB(t))

	if modules.access == nil || modules.automation == nil || modules.engagement == nil {
		t.Fatal("newModulesWithDB() returned nil module")
	}
	if modules.routes.access.Dependencies().Middleware.DB == nil {
		t.Fatal("access middleware deps DB = nil")
	}
	if modules.routes.access.Dependencies().Auth == nil {
		t.Fatal("access auth handler = nil")
	}
}

func TestSetupRoutesWithDBRegistersRouteGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newHTTPAPITestConfig()
	config.SetGlobalConfig(cfg)
	router := gin.New()

	SetupRoutesWithDB(router, cfg, openHTTPAPITestDB(t))

	routes := router.Routes()
	assertRouteExists(t, routes, "POST", "/api/v1/auth/login")
	assertRoutePrefixExists(t, routes, "/api/v1/common/")
	assertRoutePrefixExists(t, routes, "/api/v1/platform/")
	assertRoutePrefixExists(t, routes, "/api/v1/tenant/")
}

func TestSetupRoutesWithDBNullPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newHTTPAPITestConfig()
	config.SetGlobalConfig(cfg)
	router := gin.New()

	assertPanicContains(t, "http routes require explicit db", func() {
		SetupRoutesWithDB(router, cfg, nil)
	})
}

func newHTTPAPITestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		JWT: config.JWTConfig{
			Secret:                "httpapi-test-secret",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLHours:  24,
			Issuer:                "httpapi-test",
		},
	}
}

func openHTTPAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "httpapi-router.db")), &gorm.Config{})
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

func assertRouteExists(t *testing.T, routes gin.RoutesInfo, method, path string) {
	t.Helper()

	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	t.Fatalf("route %s %s not registered", method, path)
}

func assertRoutePrefixExists(t *testing.T, routes gin.RoutesInfo, prefix string) {
	t.Helper()

	for _, route := range routes {
		if strings.HasPrefix(route.Path, prefix) {
			return
		}
	}
	t.Fatalf("route prefix %s not registered", prefix)
}

func assertPanicContains(t *testing.T, want string, fn func()) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			if strings.Contains(recovered.(string), want) {
				return
			}
			t.Fatalf("panic = %v, want substring %q", recovered, want)
		}
		t.Fatalf("panic = nil, want substring %q", want)
	}()
	fn()
}
