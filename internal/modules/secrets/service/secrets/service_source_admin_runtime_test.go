package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestDeleteSourceReturnsNotFoundForMissingSource(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())

	err := svc.DeleteSource(ctx, uuid.New())
	if !errors.Is(err, ErrSecretsSourceNotFound) {
		t.Fatalf("DeleteSource() error = %v, want %v", err, ErrSecretsSourceNotFound)
	}
}

func TestCreateSourceRequestedDefaultBecomesDefault(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	svc := NewServiceWithDB(db)
	tenantID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	source, err := svc.CreateSource(ctx, &secretsmodel.SecretsSource{
		Name:      "default-source",
		Type:      "webhook",
		AuthType:  "password",
		Config:    modeltypes.JSON{"url": "http://example.com", "method": "GET", "query_key": "hostname"},
		IsDefault: true,
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("CreateSource() error = %v", err)
	}
	if !source.IsDefault {
		t.Fatalf("CreateSource() returned IsDefault=false")
	}

	var isDefault bool
	if err := db.Table("secrets_sources").Select("is_default").Where("id = ?", source.ID.String()).Scan(&isDefault).Error; err != nil {
		t.Fatalf("query is_default: %v", err)
	}
	if !isDefault {
		t.Fatalf("stored source is not default")
	}
}

func TestDisableSourcePromotesNextActiveDefault(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	defaultID := uuid.New()
	nextID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		defaultID.String(), tenantID.String(), "default-source", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, true, 1, "active", now, now)
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		nextID.String(), tenantID.String(), "next-source", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, false, 2, "active", now, now)

	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	if err := svc.Disable(ctx, defaultID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	type row struct {
		ID        string
		Status    string
		IsDefault bool
	}
	var rows []row
	if err := db.Table("secrets_sources").Select("id, status, is_default").Where("tenant_id = ?", tenantID.String()).Scan(&rows).Error; err != nil {
		t.Fatalf("query rows: %v", err)
	}
	for _, item := range rows {
		if item.ID == defaultID.String() && (item.Status != "inactive" || item.IsDefault) {
			t.Fatalf("disabled source = %#v", item)
		}
		if item.ID == nextID.String() && !item.IsDefault {
			t.Fatalf("next source was not promoted: %#v", item)
		}
	}
}

func TestEnableSourceAssignsDefaultWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
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
		sourceID.String(), tenantID.String(), "inactive-source", "webhook", "password",
		config, false, 1, "inactive", now, now)

	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	if err := svc.Enable(ctx, sourceID); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	var row struct {
		Status    string
		IsDefault bool
	}
	if err := db.Table("secrets_sources").Select("status, is_default").Where("id = ?", sourceID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("query source: %v", err)
	}
	if row.Status != "active" || !row.IsDefault {
		t.Fatalf("enabled source = %#v", row)
	}
}

func TestUpdateSourceRejectsReferencedConfigChange(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	createSecretsServiceReferenceTables(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		sourceID.String(), tenantID.String(), "source-a", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, true, 1, "active", now, now)
	mustExecSecretsService(t, db, `
		INSERT INTO execution_tasks (id, tenant_id, secrets_source_ids) VALUES (?, ?, ?)
	`, uuid.NewString(), tenantID.String(), `["`+sourceID.String()+`"]`)

	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	_, err := svc.UpdateSource(ctx, sourceID, modeltypes.JSON{
		"url":       "http://changed.example.com",
		"method":    "GET",
		"query_key": "hostname",
	}, nil, nil, "")
	if !errors.Is(err, ErrSecretsSourceInUse) {
		t.Fatalf("UpdateSource() error = %v, want %v", err, ErrSecretsSourceInUse)
	}
}

func TestUpdateSourceRejectsInactiveDefaultAtServiceLevel(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		sourceID.String(), tenantID.String(), "source-a", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, false, 1, "active", now, now)

	setDefault := true
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	_, err := svc.UpdateSource(ctx, sourceID, nil, &setDefault, nil, "inactive")
	if !errors.Is(err, ErrDefaultSourceMustBeActive) {
		t.Fatalf("UpdateSource() error = %v, want %v", err, ErrDefaultSourceMustBeActive)
	}
}

func TestDisablePreservesConcurrentAdminFields(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)
	installSecretsServiceDB(t, db)

	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExecSecretsService(t, db, insertSecretsServiceSourceSQL,
		sourceID.String(), tenantID.String(), "source-a", "webhook", "password",
		`{"url":"http://example.com","method":"GET","query_key":"hostname"}`, true, 7, "active", now, now)

	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	if err := svc.Disable(ctx, sourceID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	var row struct {
		Priority int
		Status   string
		Config   string
	}
	if err := db.Table("secrets_sources").Select("priority, status, config").Where("id = ?", sourceID.String()).Scan(&row).Error; err != nil {
		t.Fatalf("query row: %v", err)
	}
	if row.Priority != 7 {
		t.Fatalf("priority = %d, want 7", row.Priority)
	}
	if row.Status != "inactive" {
		t.Fatalf("status = %q, want inactive", row.Status)
	}
	if row.Config == "" {
		t.Fatalf("config unexpectedly empty")
	}
}
