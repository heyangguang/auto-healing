package audit

import (
	"path/filepath"
	"testing"
	"time"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "audit.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		username TEXT,
		email TEXT,
		password_hash TEXT
	)`,
		`CREATE TABLE audit_logs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT,
		user_id TEXT,
		username TEXT,
		ip_address TEXT,
		user_agent TEXT,
		category TEXT,
		action TEXT,
		resource_type TEXT,
		resource_id TEXT,
		resource_name TEXT,
		request_method TEXT,
		request_path TEXT,
		request_body TEXT,
		response_status INTEGER,
		changes TEXT,
		status TEXT,
		error_message TEXT,
		created_at DATETIME
	)`,
		`CREATE TABLE platform_audit_logs (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		username TEXT,
		ip_address TEXT,
		user_agent TEXT,
		category TEXT,
		action TEXT,
		resource_type TEXT,
		resource_id TEXT,
		resource_name TEXT,
		request_method TEXT,
		request_path TEXT,
		request_body TEXT,
		response_status INTEGER,
		changes TEXT,
		status TEXT,
		error_message TEXT,
		created_at DATETIME
	)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create audit test table: %v", err)
		}
	}
	return db
}

func newAuditDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dryrun sqlite db: %v", err)
	}
	return db
}

func createAuditUser(t *testing.T, db *gorm.DB, id uuid.UUID, username string) {
	t.Helper()

	if err := db.Exec(
		"INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)",
		id.String(),
		username,
		username+"@example.com",
		"hash",
	).Error; err != nil {
		t.Fatalf("create audit user: %v", err)
	}
}

func insertAuditLog(t *testing.T, db *gorm.DB, log platformmodel.AuditLog) {
	t.Helper()
	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("insert audit log: %v", err)
	}
}

func insertPlatformAuditLog(t *testing.T, db *gorm.DB, log platformmodel.PlatformAuditLog) {
	t.Helper()
	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("insert platform audit log: %v", err)
	}
}

func fixedAuditTime(offset time.Duration) time.Time {
	return time.Now().Add(offset).UTC().Truncate(time.Second)
}
