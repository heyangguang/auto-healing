package incident

import (
	"path/filepath"
	"testing"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newStateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "state.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
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

var WithTenantID = platformrepo.WithTenantID
