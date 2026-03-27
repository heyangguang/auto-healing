package audit

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPlatformAuditGetStatsReturnsDatabaseError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	repo := NewPlatformAuditLogRepositoryWithDB(db)
	_, err = repo.GetStats(context.Background())
	if err == nil {
		t.Fatal("GetStats error = nil, want database error")
	}
}
