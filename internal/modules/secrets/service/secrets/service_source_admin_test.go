package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApplySourceAdminChangesRejectsNegativePriority(t *testing.T) {
	source := &secretsmodel.SecretsSource{}
	priority := -1

	_, err := applySourceAdminChanges(source, nil, &priority, "")
	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want invalid input")
	}
	if !errors.Is(err, ErrSecretsSourceInvalidInput) {
		t.Fatalf("applySourceAdminChanges() error = %v, want invalid input", err)
	}
}

func TestApplySourceAdminChangesRejectsInvalidStatus(t *testing.T) {
	source := &secretsmodel.SecretsSource{}

	_, err := applySourceAdminChanges(source, nil, nil, "enabled")
	if err == nil {
		t.Fatal("applySourceAdminChanges() error = nil, want invalid input")
	}
	if !errors.Is(err, ErrSecretsSourceInvalidInput) {
		t.Fatalf("applySourceAdminChanges() error = %v, want invalid input", err)
	}
}

func TestApplySourceAdminChangesReturnsRequestedDefaultAndClearsStoredDefault(t *testing.T) {
	source := &secretsmodel.SecretsSource{
		IsDefault: false,
		Priority:  1,
		Status:    "inactive",
	}
	isDefault := true
	priority := 3

	requestedDefault, err := applySourceAdminChanges(source, &isDefault, &priority, "active")
	if err != nil {
		t.Fatalf("applySourceAdminChanges() error = %v", err)
	}
	if !requestedDefault {
		t.Fatal("requestedDefault = false, want true")
	}
	if source.IsDefault {
		t.Fatal("source.IsDefault = true, want false before repo.SetDefault")
	}
	if source.Priority != 3 {
		t.Fatalf("source.Priority = %d, want 3", source.Priority)
	}
	if source.Status != "active" {
		t.Fatalf("source.Status = %q, want active", source.Status)
	}
}

func TestApplySourceAdminChangesLeavesDefaultUnchangedWhenUnset(t *testing.T) {
	source := &secretsmodel.SecretsSource{
		IsDefault: true,
		Priority:  2,
		Status:    "active",
	}

	requestedDefault, err := applySourceAdminChanges(source, nil, nil, "")
	if err != nil {
		t.Fatalf("applySourceAdminChanges() error = %v", err)
	}
	if requestedDefault {
		t.Fatal("requestedDefault = true, want false")
	}
	if !source.IsDefault {
		t.Fatal("source.IsDefault = false, want true")
	}
}

func TestApplySourceAdminChangesRejectsInactiveDefault(t *testing.T) {
	source := &secretsmodel.SecretsSource{Status: "active"}
	inactive := "inactive"
	setDefault := true

	_, err := applySourceAdminChanges(source, &setDefault, nil, inactive)
	if err == nil {
		t.Fatalf("applySourceAdminChanges() expected error")
	}
	if source.Status != inactive {
		t.Fatalf("source.Status = %q, want %q", source.Status, inactive)
	}
}

func TestDeleteSourceReturnsNotFoundForMissingSource(t *testing.T) {
	db := newSecretsServiceTestDB(t)
	createSecretsSourceServiceTable(t, db)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	svc := NewService()
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

	svc := NewService()
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

	svc := NewService()
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

	svc := NewService()
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

	svc := NewService()
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
	svc := NewService()
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

	svc := NewService()
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

func installSecretsServiceDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })
}

const insertSecretsServiceSourceSQL = `
	INSERT INTO secrets_sources (
		id, tenant_id, name, type, auth_type, config, is_default, priority, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func mustExecSecretsService(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func newSecretsServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "secrets-service.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createSecretsSourceServiceTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE secrets_sources (
			id TEXT PRIMARY KEY NOT NULL DEFAULT (
				lower(hex(randomblob(4))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(6)))
			),
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			config TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			priority INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'inactive',
			last_test_at DATETIME,
			last_test_result BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		);
	`).Error; err != nil {
		t.Fatalf("create secrets_sources: %v", err)
	}
}

func createSecretsServiceReferenceTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecSecretsService(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			secrets_source_ids TEXT
		);
	`)
	mustExecSecretsService(t, db, `
		CREATE TABLE execution_schedules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			secrets_source_ids TEXT
		);
	`)
}
