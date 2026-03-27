package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCommandBlacklistServiceCreateForcesTenantRule(t *testing.T) {
	db := openCommandBlacklistServiceDB(t)
	createCommandBlacklistServiceSchema(t, db)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := NewCommandBlacklistService()
	rule := &model.CommandBlacklist{
		Name:     "tenant rule",
		Pattern:  "rm -rf /",
		IsSystem: true,
	}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if err := svc.Create(platformrepo.WithTenantID(context.Background(), tenantID), rule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rule.IsSystem {
		t.Fatal("Create() should force IsSystem=false")
	}

	var saved model.CommandBlacklist
	if err := db.First(&saved, "id = ?", rule.ID).Error; err != nil {
		t.Fatalf("load saved rule: %v", err)
	}
	if saved.IsSystem {
		t.Fatal("saved rule should remain tenant-scoped")
	}
}

func TestApplyCommandBlacklistUpdatePreservesOmittedOptionalFields(t *testing.T) {
	rule := &model.CommandBlacklist{
		Name:        "old",
		Category:    "system",
		Description: "keep",
	}

	applyCommandBlacklistUpdate(rule, &model.CommandBlacklist{Name: "new"})

	if rule.Name != "new" {
		t.Fatalf("name = %q, want new", rule.Name)
	}
	if rule.Category != "system" {
		t.Fatalf("category = %q, want system", rule.Category)
	}
	if rule.Description != "keep" {
		t.Fatalf("description = %q, want keep", rule.Description)
	}
}

func TestSimulateFilesUsesPerFileLineNumbers(t *testing.T) {
	results, err := (&CommandBlacklistService{repo: opsrepo.NewCommandBlacklistRepository()}).Simulate(&SimulateRequest{
		Pattern:   "rm",
		MatchType: "contains",
		Files: []SimulateFileReq{
			{Path: "a.sh", Content: "rm -rf /\nnoop"},
			{Path: "b.sh", Content: "rm -rf /"},
		},
	})
	if err != nil {
		t.Fatalf("Simulate() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[2].File != "b.sh" || results[2].Line != 1 {
		t.Fatalf("third result = %#v, want file b.sh line 1", results[2])
	}
}

func openCommandBlacklistServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "command-blacklist-service.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createCommandBlacklistServiceSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE command_blacklist (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			pattern TEXT NOT NULL,
			match_type TEXT NOT NULL,
			severity TEXT NOT NULL,
			category TEXT,
			description TEXT,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			is_system BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
}
