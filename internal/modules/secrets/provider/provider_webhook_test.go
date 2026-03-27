package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
)

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

	source := &secretsmodel.SecretsSource{
		Name:     "webhook-source",
		Type:     "webhook",
		AuthType: "password",
		Config: modeltypes.JSON{
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

func TestNewWebhookProviderRejectsMalformedURL(t *testing.T) {
	source := &secretsmodel.SecretsSource{
		Name:     "webhook-source",
		Type:     "webhook",
		AuthType: "password",
		Config: modeltypes.JSON{
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

func TestWebhookProviderGetSecretTreatsMissingResponsePathAsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"unexpected":{"password":"secret"}}}`))
	}))
	defer server.Close()

	source := &secretsmodel.SecretsSource{
		Name:     "webhook-source",
		Type:     "webhook",
		AuthType: "password",
		Config: modeltypes.JSON{
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
	_, err = provider.GetSecret(context.Background(), secretsmodel.SecretQuery{Hostname: "host-a"})
	if !errors.Is(err, ErrProviderInvalidResponse) {
		t.Fatalf("expected ErrProviderInvalidResponse, got %v", err)
	}
}

func TestBuildMappedSecretTreatsMissingCredentialAsInvalidResponse(t *testing.T) {
	_, err := buildMappedSecret("password", secretsmodel.FieldMapping{}, func(string) string {
		return ""
	})
	if !errors.Is(err, ErrProviderInvalidResponse) {
		t.Fatalf("expected ErrProviderInvalidResponse, got %v", err)
	}
}
