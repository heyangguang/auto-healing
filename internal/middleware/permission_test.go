package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type middlewareErrorResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	ErrorCode string         `json:"error_code"`
	Details   map[string]any `json:"details"`
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
	if resp.Details["match"] != "all" {
		t.Fatalf("details.match = %#v, want all", resp.Details["match"])
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
	if resp.Details["required_role"] != "admin" {
		t.Fatalf("details.required_role = %#v, want admin", resp.Details["required_role"])
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
