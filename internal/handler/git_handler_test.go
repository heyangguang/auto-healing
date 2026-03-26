package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/git"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

type gitValidateErrorResponse struct {
	Code      int               `json:"code"`
	Message   string            `json:"message"`
	ErrorCode string            `json:"error_code"`
	Details   map[string]string `json:"details"`
}

func TestValidateRepoReturnsStructuredKnownHostsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("AUTO_HEALING_KNOWN_HOSTS", "")
	t.Setenv("HOME", t.TempDir())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.PermissionsKey, []string{"repository:validate"})
	})
	router.POST("/git/validate", NewGitRepoHandler().ValidateRepo)

	body := `{"url":"git@github.com:example/repo.git","auth_type":"ssh_key","auth_config":{"private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\\nfake\\n-----END OPENSSH PRIVATE KEY-----"}}`
	req := httptest.NewRequest(http.MethodPost, "/git/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var resp gitValidateErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != 40000 {
		t.Fatalf("unexpected code: %d", resp.Code)
	}
	if resp.ErrorCode != git.ErrorCodeKnownHostsRequired {
		t.Fatalf("unexpected error_code: %q", resp.ErrorCode)
	}
	if resp.Details["env_var"] != "AUTO_HEALING_KNOWN_HOSTS" {
		t.Fatalf("missing env_var details: %#v", resp.Details)
	}
	if resp.Details["default_path"] == "" {
		t.Fatalf("missing default_path details: %#v", resp.Details)
	}
}

func TestValidateRepoRequiresRepositoryValidatePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/git/validate", NewGitRepoHandler().ValidateRepo)

	req := httptest.NewRequest(http.MethodPost, "/git/validate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var resp struct {
		ErrorCode string         `json:"error_code"`
		Details   map[string]any `json:"details"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ErrorCode != middleware.ErrorCodePermissionRequired {
		t.Fatalf("error_code = %q, want %q", resp.ErrorCode, middleware.ErrorCodePermissionRequired)
	}
	if resp.Details["required_permission"] != "repository:validate" {
		t.Fatalf("details.required_permission = %#v, want repository:validate", resp.Details["required_permission"])
	}
}

func TestCreateRepoRequiresRepositoryValidatePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/git", NewGitRepoHandler().CreateRepo)

	req := httptest.NewRequest(http.MethodPost, "/git", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}
