package repositoryx

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type tenantScopedTestModel struct {
	ID       uuid.UUID  `gorm:"column:id;primaryKey"`
	TenantID *uuid.UUID `gorm:"column:tenant_id"`
	Name     string     `gorm:"column:name"`
}

func (tenantScopedTestModel) TableName() string {
	return "tenant_scoped_test_models"
}

func TestTenantDBRequiresTenantContext(t *testing.T) {
	db := newRepositoryXTestDB(t)

	tx := TenantDB(db, context.Background())
	if !errors.Is(tx.Error, ErrTenantContextRequired) {
		t.Fatalf("TenantDB() error = %v, want %v", tx.Error, ErrTenantContextRequired)
	}
}

func TestTenantIDFromContextMissingReturnsNil(t *testing.T) {
	if got := TenantIDFromContext(context.Background()); got != uuid.Nil {
		t.Fatalf("TenantIDFromContext() = %v, want %v", got, uuid.Nil)
	}
}

func TestFillTenantIDRequiresTenantContext(t *testing.T) {
	var tenantID *uuid.UUID
	err := FillTenantID(context.Background(), &tenantID)
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("FillTenantID() error = %v, want %v", err, ErrTenantContextRequired)
	}
}

func TestFillTenantIDUsesExplicitTenantContext(t *testing.T) {
	expected := uuid.New()
	ctx := WithTenantID(context.Background(), expected)

	var tenantID *uuid.UUID
	if err := FillTenantID(ctx, &tenantID); err != nil {
		t.Fatalf("FillTenantID() error = %v", err)
	}
	if tenantID == nil || *tenantID != expected {
		t.Fatalf("FillTenantID() tenantID = %v, want %v", tenantID, expected)
	}
}

func TestUpdateTenantScopedModelRequiresTenantContext(t *testing.T) {
	db := newRepositoryXTestDB(t)
	mustExecRepositoryX(t, db, `
		CREATE TABLE tenant_scoped_test_models (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)

	tenantID := uuid.New()
	entity := &tenantScopedTestModel{ID: uuid.New(), TenantID: &tenantID, Name: "updated"}

	err := UpdateTenantScopedModel(db, context.Background(), entity.ID, entity)
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("UpdateTenantScopedModel() error = %v, want %v", err, ErrTenantContextRequired)
	}
}

func TestUpdateTenantScopedModelUsesTenantScope(t *testing.T) {
	db := newRepositoryXTestDB(t)
	mustExecRepositoryX(t, db, `
		CREATE TABLE tenant_scoped_test_models (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT
		);
	`)

	tenantA := uuid.New()
	tenantB := uuid.New()
	entityID := uuid.New()
	mustExecRepositoryX(t, db, `INSERT INTO tenant_scoped_test_models (id, tenant_id, name) VALUES (?, ?, ?)`, entityID.String(), tenantA.String(), "before")

	ctxA := WithTenantID(context.Background(), tenantA)
	ctxB := WithTenantID(context.Background(), tenantB)
	update := &tenantScopedTestModel{ID: entityID, TenantID: &tenantA, Name: "after"}

	if err := UpdateTenantScopedModel(db, ctxB, entityID, update); err != nil {
		t.Fatalf("UpdateTenantScopedModel() wrong tenant error = %v", err)
	}

	var afterWrongTenant tenantScopedTestModel
	if err := db.WithContext(context.Background()).First(&afterWrongTenant, "id = ?", entityID).Error; err != nil {
		t.Fatalf("query after wrong tenant update error = %v", err)
	}
	if afterWrongTenant.Name != "before" {
		t.Fatalf("wrong-tenant update changed row name to %q, want %q", afterWrongTenant.Name, "before")
	}

	if err := UpdateTenantScopedModel(db, ctxA, entityID, update); err != nil {
		t.Fatalf("UpdateTenantScopedModel() correct tenant error = %v", err)
	}

	var afterCorrectTenant tenantScopedTestModel
	if err := db.WithContext(context.Background()).First(&afterCorrectTenant, "id = ?", entityID).Error; err != nil {
		t.Fatalf("query after correct tenant update error = %v", err)
	}
	if afterCorrectTenant.Name != "after" {
		t.Fatalf("correct-tenant update name = %q, want %q", afterCorrectTenant.Name, "after")
	}
}

func newRepositoryXTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "repositoryx.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExecRepositoryX(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
