package repository

import (
	"context"
	"testing"
)

func TestPlatformAuditGetStatsReturnsDatabaseError(t *testing.T) {
	db := newSQLiteTestDB(t)
	repo := NewPlatformAuditLogRepositoryWithDB(db)

	_, err := repo.GetStats(context.Background())
	if err == nil {
		t.Fatal("GetStats error = nil, want database error")
	}
}
