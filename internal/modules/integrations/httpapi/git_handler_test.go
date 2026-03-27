package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/integrations/gitclient"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	gitSvc "github.com/company/auto-healing/internal/modules/integrations/service/git"
	playbookSvc "github.com/company/auto-healing/internal/modules/integrations/service/playbook"
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
	handler := NewGitRepoHandlerWithDeps(GitRepoHandlerDeps{Service: gitSvc.NewServiceWithDeps(gitSvc.ServiceDeps{
		Repo:         &integrationrepo.GitRepositoryRepository{},
		PlaybookRepo: &integrationrepo.PlaybookRepository{},
		PlaybookSvc:  func() *playbookSvc.Service { return &playbookSvc.Service{} },
		ReposDir:     t.TempDir(),
	})})
	t.Cleanup(handler.Shutdown)
	router.POST("/git/validate", handler.ValidateRepo)

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
	handler := NewGitRepoHandlerWithDeps(GitRepoHandlerDeps{Service: gitSvc.NewServiceWithDeps(gitSvc.ServiceDeps{
		Repo:         &integrationrepo.GitRepositoryRepository{},
		PlaybookRepo: &integrationrepo.PlaybookRepository{},
		PlaybookSvc:  func() *playbookSvc.Service { return &playbookSvc.Service{} },
		ReposDir:     t.TempDir(),
	})})
	t.Cleanup(handler.Shutdown)
	router.POST("/git/validate", handler.ValidateRepo)

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
	handler := NewGitRepoHandlerWithDeps(GitRepoHandlerDeps{Service: gitSvc.NewServiceWithDeps(gitSvc.ServiceDeps{
		Repo:         &integrationrepo.GitRepositoryRepository{},
		PlaybookRepo: &integrationrepo.PlaybookRepository{},
		PlaybookSvc:  func() *playbookSvc.Service { return &playbookSvc.Service{} },
		ReposDir:     t.TempDir(),
	})})
	t.Cleanup(handler.Shutdown)
	router.POST("/git", handler.CreateRepo)

	req := httptest.NewRequest(http.MethodPost, "/git", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}
