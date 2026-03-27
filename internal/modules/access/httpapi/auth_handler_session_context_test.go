package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestCurrentTenantOrNilPrefersHeaderTenantClaim(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	tenantA := uuid.New()
	tenantB := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("X-Tenant-ID", tenantB.String())
	c.Request = req
	c.Set(middleware.TenantIDsKey, []string{tenantA.String(), tenantB.String()})
	c.Set(middleware.DefaultTenantIDKey, tenantA.String())

	if got := currentTenantOrNil(c); got != tenantB {
		t.Fatalf("currentTenantOrNil() = %v, want %v", got, tenantB)
	}
}

func TestCurrentTenantOrNilFallsBackToDefaultTenantClaim(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	tenantID := uuid.New()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	c.Set(middleware.DefaultTenantIDKey, tenantID.String())

	if got := currentTenantOrNil(c); got != tenantID {
		t.Fatalf("currentTenantOrNil() = %v, want %v", got, tenantID)
	}
}

func TestRefreshTokenReturnsInternalErrorWhenUserInfoReloadFails(t *testing.T) {
	db := newAuthHandlerTestDB(t)
	createAuthHandlerSchema(t, db)

	userID := uuid.New()
	insertUser(t, db, userID, "tenant-user", false)
	mustExecAuthSQL(t, db, `DROP TABLE permissions;`)

	store := &logoutBlacklistRecorder{}
	jwtSvc := jwt.NewService(jwt.Config{
		Secret:          "test-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "handler-test",
	}, store)
	router := newAuthHandlerTestRouterWithJWTService(t, db, jwtSvc)
	pair, err := jwtSvc.GenerateTokenPair(userID.String(), "tenant-user", nil, nil)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	body := []byte(fmt.Sprintf(`{"refresh_token":%q}`, pair.RefreshToken))
	recorder := issueAuthRequest(t, router, http.MethodPost, "/api/v1/auth/refresh", "", nil, body)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if _, err := jwtSvc.ValidateRefreshToken(pair.RefreshToken); err != nil {
		t.Fatalf("ValidateRefreshToken() after failed refresh error = %v, want nil", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("blacklist Add calls = %d, want 0", len(store.added))
	}
}
