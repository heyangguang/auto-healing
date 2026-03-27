package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type middlewareErrorResponse struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	ErrorCode string          `json:"error_code"`
	Details   json.RawMessage `json:"details"`
}

func TestRequirePermissionUsesStructuredErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/secure", RequirePermission("repository:validate"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if jsonContainsKey(t, recorder.Body.Bytes(), "error") {
		t.Fatalf("response should not contain legacy nested error object: %s", recorder.Body.String())
	}

	var resp middlewareErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodePermissionsContextMissing {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodePermissionsContextMissing)
	}
	if len(resp.Details) != 0 && string(resp.Details) != "null" {
		t.Fatalf("details = %s, want null/empty", string(resp.Details))
	}
}

func TestRequireAllPermissionsIncludesRequiredPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/secure", func(c *gin.Context) {
		c.Set(PermissionsKey, []string{"repository:create"})
	}, RequireAllPermissions("repository:create", "repository:validate"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	var resp middlewareErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodePermissionRequired {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodePermissionRequired)
	}
	var details PermissionDeniedDetails
	if err := decodeMiddlewareDetails(resp.Details, &details); err != nil {
		t.Fatalf("decode details: %v", err)
	}
	if details.Match != "all" {
		t.Fatalf("details.match = %q, want all", details.Match)
	}
	if len(details.RequiredPermissions) != 2 ||
		details.RequiredPermissions[0] != "repository:create" ||
		details.RequiredPermissions[1] != "repository:validate" {
		t.Fatalf("required_permissions = %#v, want [repository:create repository:validate]", details.RequiredPermissions)
	}
}

func TestRequirePermissionIncludesRequiredPermissionMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/secure", func(c *gin.Context) {
		c.Set(PermissionsKey, []string{"repository:create"})
	}, RequirePermission("repository:validate"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	var resp middlewareErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodePermissionRequired {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodePermissionRequired)
	}
	var details PermissionDeniedDetails
	if err := decodeMiddlewareDetails(resp.Details, &details); err != nil {
		t.Fatalf("decode details: %v", err)
	}
	if details.RequiredPermission != "repository:validate" {
		t.Fatalf("required_permission = %q, want repository:validate", details.RequiredPermission)
	}
	if details.Match != "all" {
		t.Fatalf("match = %q, want all", details.Match)
	}
}

func TestRequireRoleUsesStructuredErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/secure", func(c *gin.Context) {
		c.Set(RolesKey, []string{"viewer"})
	}, RequireRole("admin"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	var resp middlewareErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != ErrorCodeRoleRequired {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, ErrorCodeRoleRequired)
	}
	var details RoleRequiredDetails
	if err := decodeMiddlewareDetails(resp.Details, &details); err != nil {
		t.Fatalf("decode details: %v", err)
	}
	if details.RequiredRole != "admin" {
		t.Fatalf("details.required_role = %q, want admin", details.RequiredRole)
	}
}

func jsonContainsKey(t *testing.T, payload []byte, key string) bool {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	_, ok := decoded[key]
	return ok
}

func decodeMiddlewareError(recorder *httptest.ResponseRecorder, resp *middlewareErrorResponse) error {
	return json.Unmarshal(recorder.Body.Bytes(), resp)
}

func decodeMiddlewareDetails(payload json.RawMessage, target any) error {
	if len(payload) == 0 || string(payload) == "null" {
		return nil
	}
	return json.Unmarshal(payload, target)
}
