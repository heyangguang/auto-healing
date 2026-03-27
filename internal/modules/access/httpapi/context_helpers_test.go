package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequiredAuthTenantContextRejectsStaleDefaultTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.New()
	userID := uuid.New()
	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertUser(t, db, userID, "tenant-user", false)

	router := gin.New()
	router.GET("/me", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID.String())
		c.Set(middleware.DefaultTenantIDKey, tenantID.String())
		c.Set(middleware.TenantIDsKey, []string{tenantID.String()})
	}, requiredAuthTenantContext(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestOptionalAuthTenantContextAllowsMissingTenantInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.New()
	userID := uuid.New()
	insertTenant(t, db, tenantID, "Tenant A", "tenant-a")
	insertUser(t, db, userID, "tenant-user", false)

	router := gin.New()
	router.POST("/logout", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID.String())
		c.Set(middleware.DefaultTenantIDKey, tenantID.String())
		c.Set(middleware.TenantIDsKey, []string{tenantID.String()})
	}, optionalAuthTenantContext(), func(c *gin.Context) {
		if _, ok := repository.TenantIDFromContextOK(c.Request.Context()); ok {
			t.Fatal("tenant context injected unexpectedly")
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/logout", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}
