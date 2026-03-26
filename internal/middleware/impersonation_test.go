package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestLoadImpersonationPermissionsUsesTTLCache(t *testing.T) {
	origLoader := impersonationPermsLoader
	origTTL := impersonationPermsTTL
	origPerms := append([]string(nil), impersonationPerms...)
	origLoadedAt := impersonationPermsLoadedAt
	defer func() {
		impersonationPermsLoader = origLoader
		impersonationPermsTTL = origTTL
		impersonationPerms = origPerms
		impersonationPermsLoadedAt = origLoadedAt
	}()

	impersonationPerms = nil
	impersonationPermsLoadedAt = time.Time{}
	impersonationPermsTTL = time.Hour

	var calls atomic.Int32
	impersonationPermsLoader = func(context.Context) ([]string, error) {
		switch calls.Add(1) {
		case 1:
			return []string{"first"}, nil
		default:
			return []string{"second"}, nil
		}
	}

	first, err := loadImpersonationPermissions(context.Background())
	if err != nil {
		t.Fatalf("first load error = %v", err)
	}
	second, err := loadImpersonationPermissions(context.Background())
	if err != nil {
		t.Fatalf("second load error = %v", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("loader called %d times, want 1", calls.Load())
	}
	if len(first) != 1 || first[0] != "first" || len(second) != 1 || second[0] != "first" {
		t.Fatalf("unexpected cached perms: first=%v second=%v", first, second)
	}
}

func TestLoadImpersonationPermissionsRefreshesAfterTTL(t *testing.T) {
	origLoader := impersonationPermsLoader
	origTTL := impersonationPermsTTL
	origPerms := append([]string(nil), impersonationPerms...)
	origLoadedAt := impersonationPermsLoadedAt
	defer func() {
		impersonationPermsLoader = origLoader
		impersonationPermsTTL = origTTL
		impersonationPerms = origPerms
		impersonationPermsLoadedAt = origLoadedAt
	}()

	impersonationPerms = nil
	impersonationPermsLoadedAt = time.Time{}
	impersonationPermsTTL = 5 * time.Millisecond

	var calls atomic.Int32
	impersonationPermsLoader = func(context.Context) ([]string, error) {
		if calls.Add(1) == 1 {
			return []string{"first"}, nil
		}
		return []string{"second"}, nil
	}

	if perms, err := loadImpersonationPermissions(context.Background()); err != nil || len(perms) != 1 || perms[0] != "first" {
		t.Fatalf("first perms = %v, want [first]", perms)
	}

	time.Sleep(10 * time.Millisecond)

	if perms, err := loadImpersonationPermissions(context.Background()); err != nil || len(perms) != 1 || perms[0] != "second" {
		t.Fatalf("refreshed perms = %v, want [second]", perms)
	}
	if calls.Load() != 2 {
		t.Fatalf("loader called %d times, want 2", calls.Load())
	}
}

func TestLoadImpersonationPermissionsPassesContextToLoader(t *testing.T) {
	origLoader := impersonationPermsLoader
	origPerms := append([]string(nil), impersonationPerms...)
	origLoadedAt := impersonationPermsLoadedAt
	defer func() {
		impersonationPermsLoader = origLoader
		impersonationPerms = origPerms
		impersonationPermsLoadedAt = origLoadedAt
	}()

	impersonationPerms = nil
	impersonationPermsLoadedAt = time.Time{}
	ctx := context.WithValue(context.Background(), "probe", "impersonation")
	impersonationPermsLoader = func(loaderCtx context.Context) ([]string, error) {
		if got := loaderCtx.Value("probe"); got != "impersonation" {
			t.Fatalf("loader ctx value = %v, want impersonation", got)
		}
		return []string{"ok"}, nil
	}

	if _, err := loadImpersonationPermissions(ctx); err != nil {
		t.Fatalf("loadImpersonationPermissions() error = %v", err)
	}
}

func TestApplyImpersonationContextAbortsWhenPermissionLoadFails(t *testing.T) {
	origLoader := impersonationPermsLoader
	origPerms := append([]string(nil), impersonationPerms...)
	origLoadedAt := impersonationPermsLoadedAt
	defer func() {
		impersonationPermsLoader = origLoader
		impersonationPerms = origPerms
		impersonationPermsLoadedAt = origLoadedAt
	}()

	impersonationPerms = nil
	impersonationPermsLoadedAt = time.Time{}
	impersonationPermsLoader = func(context.Context) ([]string, error) {
		return nil, errors.New("db down")
	}

	gin.SetMode(gin.TestMode)
	logger.Init(&config.LogConfig{})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)

	if ok := applyImpersonationContext(c, uuid.New(), uuid.New()); ok {
		t.Fatal("applyImpersonationContext() ok = true, want false")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if _, exists := c.Get(PermissionsKey); exists {
		t.Fatal("permissions should not be set when loader fails")
	}
}
