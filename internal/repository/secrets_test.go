package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSecretsSourceRepositoryCreateOverridesTenantID(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	ctxTenant := uuid.New()
	source := &model.SecretsSource{
		ID:       uuid.New(),
		TenantID: uuidPtr(uuid.New()),
		Name:     "source-a",
		Type:     "file",
		AuthType: "ssh_key",
		Config:   model.JSON{"key_path": "/etc/auto-healing/secrets/id_a"},
		Status:   "active",
	}

	if err := repo.Create(WithTenantID(context.Background(), ctxTenant), source); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if source.TenantID == nil || *source.TenantID != ctxTenant {
		t.Fatalf("source.TenantID = %v, want %v", source.TenantID, ctxTenant)
	}

	var tenantID string
	if err := db.Table("secrets_sources").Select("tenant_id").Where("id = ?", source.ID.String()).Scan(&tenantID).Error; err != nil {
		t.Fatalf("query tenant_id: %v", err)
	}
	if tenantID != ctxTenant.String() {
		t.Fatalf("stored tenant_id = %q, want %q", tenantID, ctxTenant.String())
	}
}

func TestSecretsSourceRepositoryEnsureActiveDefaultPromotesStableCandidate(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	tenantID := uuid.New()
	ctx := WithTenantID(context.Background(), tenantID)
	olderID := uuid.New()
	newerID := uuid.New()
	now := time.Now().UTC()

	mustExec(t, db, insertSecretsSourceSQL,
		olderID.String(), tenantID.String(), "older", "file", "ssh_key", `{}`, false, 10, "active",
		now.Add(-time.Hour).Format(time.RFC3339), now.Format(time.RFC3339))
	mustExec(t, db, insertSecretsSourceSQL,
		newerID.String(), tenantID.String(), "newer", "file", "ssh_key", `{}`, false, 10, "active",
		now.Format(time.RFC3339), now.Format(time.RFC3339))

	if err := repo.EnsureActiveDefault(ctx); err != nil {
		t.Fatalf("EnsureActiveDefault() error = %v", err)
	}

	var defaultID string
	if err := db.Table("secrets_sources").Select("id").Where("tenant_id = ? AND is_default = ?", tenantID.String(), true).Scan(&defaultID).Error; err != nil {
		t.Fatalf("query default source: %v", err)
	}
	if defaultID != olderID.String() {
		t.Fatalf("default id = %q, want %q", defaultID, olderID.String())
	}
}

func TestSecretsSourceRepositoryEnsureActiveDefaultClearsInactiveDefault(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	tenantID := uuid.New()
	ctx := WithTenantID(context.Background(), tenantID)
	inactiveID := uuid.New()
	activeID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, insertSecretsSourceSQL,
		inactiveID.String(), tenantID.String(), "inactive-default", "file", "ssh_key", `{}`, true, 1, "inactive", now, now)
	mustExec(t, db, insertSecretsSourceSQL,
		activeID.String(), tenantID.String(), "active", "file", "ssh_key", `{}`, false, 5, "active", now, now)

	if err := repo.EnsureActiveDefault(ctx); err != nil {
		t.Fatalf("EnsureActiveDefault() error = %v", err)
	}

	type row struct {
		ID        string
		IsDefault bool
	}
	var rows []row
	if err := db.Table("secrets_sources").Select("id, is_default").Where("tenant_id = ?", tenantID.String()).Scan(&rows).Error; err != nil {
		t.Fatalf("query rows: %v", err)
	}

	defaults := make(map[string]bool, len(rows))
	for _, item := range rows {
		defaults[item.ID] = item.IsDefault
	}
	if defaults[inactiveID.String()] {
		t.Fatalf("inactive source remained default")
	}
	if !defaults[activeID.String()] {
		t.Fatalf("active source was not promoted to default")
	}
}

func TestSecretsSourceRepositoryDeleteReturnsNotFound(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	err := repo.Delete(WithTenantID(context.Background(), uuid.New()), uuid.New())
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Delete() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

func TestSecretsSourceRepositorySetDefaultReturnsNotFound(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	err := repo.SetDefault(WithTenantID(context.Background(), uuid.New()), uuid.New())
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("SetDefault() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

func TestSecretsSourceRepositoryEnsureActiveDefaultClearsDefaultWhenNoActiveSource(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	tenantID := uuid.New()
	ctx := WithTenantID(context.Background(), tenantID)
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExec(t, db, insertSecretsSourceSQL,
		sourceID.String(), tenantID.String(), "inactive-default", "file", "ssh_key", `{}`, true, 1, "inactive", now, now)

	if err := repo.EnsureActiveDefault(ctx); err != nil {
		t.Fatalf("EnsureActiveDefault() error = %v", err)
	}

	var isDefault bool
	if err := db.Table("secrets_sources").Select("is_default").Where("id = ?", sourceID.String()).Scan(&isDefault).Error; err != nil {
		t.Fatalf("query is_default: %v", err)
	}
	if isDefault {
		t.Fatalf("inactive default was not cleared")
	}
}

func TestSecretsSourceRepositoryUpdatePreservesTestFields(t *testing.T) {
	db := newStateTestDB(t)
	createSecretsSourcesTable(t, db)

	repo := &SecretsSourceRepository{db: db}
	tenantID := uuid.New()
	sourceID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	mustExec(t, db, `
		INSERT INTO secrets_sources (
			id, tenant_id, name, type, auth_type, config, is_default, priority, status, last_test_at, last_test_result, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sourceID.String(), tenantID.String(), "source-a", "webhook", "password", `{"url":"http://example.com","method":"GET","query_key":"hostname"}`,
		false, 1, "active", now, true, now, now)

	source := &model.SecretsSource{
		ID:        sourceID,
		TenantID:  uuidPtr(tenantID),
		Name:      "source-a",
		Type:      "webhook",
		AuthType:  "password",
		Config:    model.JSON{"url": "http://example.com", "method": "POST", "query_key": "hostname"},
		IsDefault: true,
		Priority:  2,
		Status:    "inactive",
	}

	if err := repo.Update(WithTenantID(context.Background(), tenantID), source); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	var row struct {
		LastTestAt     string
		LastTestResult bool
		IsDefault      bool
		Priority       int
		Status         string
	}
	if err := db.Table("secrets_sources").
		Select("last_test_at, last_test_result, is_default, priority, status").
		Where("id = ?", sourceID.String()).
		Scan(&row).Error; err != nil {
		t.Fatalf("query row: %v", err)
	}
	if row.LastTestAt == "" || !row.LastTestResult {
		t.Fatalf("test fields were overwritten: %#v", row)
	}
	if !row.IsDefault || row.Priority != 2 || row.Status != "inactive" {
		t.Fatalf("admin fields not updated: %#v", row)
	}
}

const insertSecretsSourceSQL = `
	INSERT INTO secrets_sources (
		id, tenant_id, name, type, auth_type, config, is_default, priority, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func createSecretsSourcesTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE secrets_sources (
			id TEXT PRIMARY KEY NOT NULL,
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
	`)
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
