package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/google/uuid"
)

func TestPreferenceRoutesRespectTenantIsolation(t *testing.T) {
	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	userID := uuid.NewString()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	handler := &PreferenceHandler{
		prefRepo: engagementrepo.NewUserPreferenceRepositoryWithDB(db),
	}

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          userID,
		defaultTenantID: tenantA,
	})
	router.PUT("/common/user/preferences", handler.UpdatePreferences)
	router.GET("/common/user/preferences", handler.GetPreferences)

	putJSON(t, router, http.MethodPut, "/common/user/preferences", `{"preferences":{"theme":"alpha"}}`, "")
	putJSON(t, router, http.MethodPut, "/common/user/preferences", `{"preferences":{"theme":"beta"}}`, tenantB)

	respA := getJSON(t, router, "/common/user/preferences", "")
	respB := getJSON(t, router, "/common/user/preferences", tenantB)

	dataA := respA.Data.(map[string]interface{})
	dataB := respB.Data.(map[string]interface{})
	prefA := dataA["preferences"].(map[string]interface{})
	prefB := dataB["preferences"].(map[string]interface{})
	if prefA["theme"] != "alpha" {
		t.Fatalf("tenant A theme = %#v, want alpha", prefA["theme"])
	}
	if prefB["theme"] != "beta" {
		t.Fatalf("tenant B theme = %#v, want beta", prefB["theme"])
	}
}

func TestPreferenceUpdateRouteRejectsNullPreferences(t *testing.T) {
	db := newPreferenceTestDB(t)
	createUserPreferenceSchema(t, db)
	handler := &PreferenceHandler{
		prefRepo: engagementrepo.NewUserPreferenceRepositoryWithDB(db),
	}

	router := newOwnedScopeTestRouter(ownedScopeTestContext{
		userID:          uuid.NewString(),
		defaultTenantID: uuid.NewString(),
	})
	router.PUT("/common/user/preferences", handler.UpdatePreferences)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/common/user/preferences", bytes.NewBufferString(`{"preferences":null}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func putJSON(t *testing.T, router http.Handler, method, path, body, tenantID string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func getJSON(t *testing.T, router http.Handler, path, tenantID string) struct {
	Data any `json:"data"`
} {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d; body=%s", path, recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var resp struct {
		Data any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}
