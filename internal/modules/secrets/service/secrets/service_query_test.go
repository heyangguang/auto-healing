package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestResolveSecretsSourceNoDefault(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	svc := NewService()
	_, err := svc.resolveSecretsSource(platformrepo.WithTenantID(context.Background(), uuid.New()), "")
	if !errors.Is(err, ErrDefaultSecretsSourceUnavailable) {
		t.Fatalf("resolveSecretsSource() error = %v, want %v", err, ErrDefaultSecretsSourceUnavailable)
	}
}

func TestResolveSecretsSourceRejectsInvalidID(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	svc := NewService()
	_, err := svc.resolveSecretsSource(platformrepo.WithTenantID(context.Background(), uuid.New()), "bad-id")
	if !errors.Is(err, ErrSecretsSourceInvalidID) {
		t.Fatalf("resolveSecretsSource() error = %v, want %v", err, ErrSecretsSourceInvalidID)
	}
}

func TestResolveSecretsSourceRejectsInactiveSource(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		sourceID.String(), tenantID.String(), "inactive-source", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, false, 1, "inactive", now, now)

	svc := NewService()
	_, err := svc.resolveSecretsSource(platformrepo.WithTenantID(context.Background(), tenantID), sourceID.String())
	if !errors.Is(err, ErrSecretsSourceInactive) {
		t.Fatalf("resolveSecretsSource() error = %v, want %v", err, ErrSecretsSourceInactive)
	}
}

func TestQuerySecretUsesDefaultSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"username":"alice","password":"secret"}}`))
	}))
	defer server.Close()

	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	config := `{"url":"` + server.URL + `","method":"GET","query_key":"hostname"}`
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		sourceID.String(), tenantID.String(), "default-source", "webhook", "password",
		config, true, 1, "active", now, now)

	svc := NewService()
	secret, err := svc.QuerySecret(platformrepo.WithTenantID(context.Background(), tenantID), secretsmodel.SecretQuery{Hostname: "host-a"})
	if err != nil {
		t.Fatalf("QuerySecret() error = %v", err)
	}
	if secret.Username != "alice" || secret.Password != "secret" {
		t.Fatalf("QuerySecret() = %#v", secret)
	}
}

func TestQuerySecretReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	config := `{"url":"` + server.URL + `","method":"GET","query_key":"hostname"}`
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		sourceID.String(), tenantID.String(), "default-source", "webhook", "password",
		config, true, 1, "active", now, now)

	svc := NewService()
	_, err := svc.QuerySecret(platformrepo.WithTenantID(context.Background(), tenantID), secretsmodel.SecretQuery{Hostname: "host-a"})
	if !errors.Is(err, ErrSecretsProviderRequestFailed) {
		t.Fatalf("QuerySecret() error = %v, want %v", err, ErrSecretsProviderRequestFailed)
	}
}

func TestQuerySecretRequiresHostnameOrIPAddress(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	svc := NewService()
	_, err := svc.QuerySecret(platformrepo.WithTenantID(context.Background(), uuid.New()), secretsmodel.SecretQuery{})
	if !errors.Is(err, ErrSecretsQueryTargetRequired) {
		t.Fatalf("QuerySecret() error = %v, want %v", err, ErrSecretsQueryTargetRequired)
	}
}
