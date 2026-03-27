package repository

import (
	"context"
	"path/filepath"
	"testing"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "engagement-repository.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return platformrepo.WithTenantID(ctx, tenantID)
}
