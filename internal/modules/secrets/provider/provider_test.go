package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestNewFileProviderRejectsNonSSHAuthType(t *testing.T) {
	source := &model.SecretsSource{
		Name:     "file-source",
		Type:     "file",
		AuthType: "password",
		Config: model.JSON{
			"key_path": "/etc/auto-healing/secrets/id_rsa",
		},
	}

	_, err := NewFileProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestFileProviderRejectsSymlinkEscape(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()
	originalPrefixes := allowedPathPrefixes
	allowedPathPrefixes = []string{allowedDir + string(os.PathSeparator)}
	t.Cleanup(func() {
		allowedPathPrefixes = originalPrefixes
	})

	targetPath := filepath.Join(outsideDir, "id_rsa")
	if err := os.WriteFile(targetPath, []byte("private-key"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	symlinkPath := filepath.Join(allowedDir, "link")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	source := &model.SecretsSource{
		Name:     "file-source",
		Type:     "file",
		AuthType: "ssh_key",
		Config: model.JSON{
			"key_path": symlinkPath,
		},
	}

	_, err := NewFileProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestFileProviderRejectsDirectoryPathOnCreate(t *testing.T) {
	allowedDir := t.TempDir()
	originalPrefixes := allowedPathPrefixes
	allowedPathPrefixes = []string{allowedDir + string(os.PathSeparator)}
	t.Cleanup(func() {
		allowedPathPrefixes = originalPrefixes
	})

	source := &model.SecretsSource{
		Name:     "file-source",
		Type:     "file",
		AuthType: "ssh_key",
		Config: model.JSON{
			"key_path": allowedDir,
		},
	}

	_, err := NewFileProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestWebhookProviderTestConnectionFallsBackToConfiguredMethod(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	source := &model.SecretsSource{
		Name:     "webhook-source",
		Type:     "webhook",
		AuthType: "password",
		Config: model.JSON{
			"url":       server.URL,
			"method":    http.MethodGet,
			"query_key": "hostname",
		},
	}

	provider, err := NewWebhookProvider(source)
	if err != nil {
		t.Fatalf("NewWebhookProvider() error = %v", err)
	}
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}

	if got, want := strings.Join(methods, ","), "HEAD,GET"; got != want {
		t.Fatalf("methods = %q, want %q", got, want)
	}
}

func TestVaultProviderTestConnectionIncludesNamespaceHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("X-Vault-Namespace"); got != "team-a" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if got := r.Header.Get("X-Vault-Token"); got != "vault-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	source := &model.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: model.JSON{
			"address":     server.URL,
			"secret_path": "kv/data/demo",
			"namespace":   "team-a",
			"query_key":   "hostname",
			"auth": map[string]interface{}{
				"type":  "token",
				"token": "vault-token",
			},
		},
	}

	provider, err := NewVaultProvider(source)
	if err != nil {
		t.Fatalf("NewVaultProvider() error = %v", err)
	}
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestVaultProviderTestConnectionAcceptsHealthyStandbyStatuses(t *testing.T) {
	statuses := []int{472, 473}
	for _, statusCode := range statuses {
		t.Run(fmt.Sprintf("status-%d", statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/health" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(statusCode)
			}))
			defer server.Close()

			source := &model.SecretsSource{
				Name:     "vault-source",
				Type:     "vault",
				AuthType: "password",
				Config: model.JSON{
					"address":     server.URL,
					"secret_path": "kv/data/demo",
					"query_key":   "hostname",
					"auth": map[string]interface{}{
						"type":  "token",
						"token": "vault-token",
					},
				},
			}

			provider, err := NewVaultProvider(source)
			if err != nil {
				t.Fatalf("NewVaultProvider() error = %v", err)
			}
			if err := provider.TestConnection(context.Background()); err != nil {
				t.Fatalf("TestConnection() error = %v", err)
			}
		})
	}
}

func TestVaultProviderAppRoleLoginIncludesNamespaceHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/approle/login":
			if got := r.Header.Get("X-Vault-Namespace"); got != "team-a" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"auth":{"client_token":"vault-token"}}`))
		case "/v1/sys/health":
			if got := r.Header.Get("X-Vault-Namespace"); got != "team-a" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if got := r.Header.Get("X-Vault-Token"); got != "vault-token" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	source := &model.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: model.JSON{
			"address":     server.URL,
			"secret_path": "kv/data/demo",
			"namespace":   "team-a",
			"query_key":   "hostname",
			"auth": map[string]interface{}{
				"type":      "approle",
				"role_id":   "role-id",
				"secret_id": "secret-id",
			},
		},
	}

	provider, err := NewVaultProvider(source)
	if err != nil {
		t.Fatalf("NewVaultProvider() error = %v", err)
	}
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestNewVaultProviderRejectsInvalidQueryKey(t *testing.T) {
	source := &model.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: model.JSON{
			"address":     "https://vault.example.com",
			"secret_path": "kv/data/demo",
			"query_key":   "mac",
			"auth": map[string]interface{}{
				"type":  "token",
				"token": "vault-token",
			},
		},
	}

	_, err := NewVaultProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestNewWebhookProviderRejectsMalformedURL(t *testing.T) {
	source := &model.SecretsSource{
		Name:     "webhook-source",
		Type:     "webhook",
		AuthType: "password",
		Config: model.JSON{
			"url":       "://bad-url",
			"method":    http.MethodGet,
			"query_key": "hostname",
		},
	}

	_, err := NewWebhookProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestNewVaultProviderRejectsMalformedAddress(t *testing.T) {
	source := &model.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: model.JSON{
			"address":     "://bad-url",
			"secret_path": "kv/data/demo",
			"query_key":   "hostname",
			"auth": map[string]interface{}{
				"type":  "token",
				"token": "vault-token",
			},
		},
	}

	_, err := NewVaultProvider(source)
	if !errors.Is(err, ErrProviderInvalidConfig) {
		t.Fatalf("expected ErrProviderInvalidConfig, got %v", err)
	}
}

func TestWebhookProviderGetSecretTreatsMissingResponsePathAsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"unexpected":{"password":"secret"}}}`))
	}))
	defer server.Close()

	source := &model.SecretsSource{
		Name:     "webhook-source",
		Type:     "webhook",
		AuthType: "password",
		Config: model.JSON{
			"url":                server.URL,
			"method":             http.MethodGet,
			"query_key":          "hostname",
			"response_data_path": "data.credentials",
		},
	}

	provider, err := NewWebhookProvider(source)
	if err != nil {
		t.Fatalf("NewWebhookProvider() error = %v", err)
	}
	_, err = provider.GetSecret(context.Background(), model.SecretQuery{Hostname: "host-a"})
	if !errors.Is(err, ErrProviderInvalidResponse) {
		t.Fatalf("expected ErrProviderInvalidResponse, got %v", err)
	}
}

func TestBuildMappedSecretTreatsMissingCredentialAsInvalidResponse(t *testing.T) {
	_, err := buildMappedSecret("password", model.FieldMapping{}, func(string) string {
		return ""
	})
	if !errors.Is(err, ErrProviderInvalidResponse) {
		t.Fatalf("expected ErrProviderInvalidResponse, got %v", err)
	}
}
