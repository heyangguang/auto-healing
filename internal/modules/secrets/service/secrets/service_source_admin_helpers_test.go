package secrets

import (
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
