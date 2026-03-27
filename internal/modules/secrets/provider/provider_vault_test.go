package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
)

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

	source := &secretsmodel.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: modeltypes.JSON{
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

			source := &secretsmodel.SecretsSource{
				Name:     "vault-source",
				Type:     "vault",
				AuthType: "password",
				Config: modeltypes.JSON{
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

	source := &secretsmodel.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: modeltypes.JSON{
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
	source := &secretsmodel.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: modeltypes.JSON{
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

func TestNewVaultProviderRejectsMalformedAddress(t *testing.T) {
	source := &secretsmodel.SecretsSource{
		Name:     "vault-source",
		Type:     "vault",
		AuthType: "password",
		Config: modeltypes.JSON{
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
