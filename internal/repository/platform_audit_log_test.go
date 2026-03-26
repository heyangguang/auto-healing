package repository

import (
	"context"
	"testing"
)

func TestPlatformAuditGetStatsReturnsDatabaseError(t *testing.T) {
	db := newSQLiteTestDB(t)
	repo := &PlatformAuditLogRepository{db: db}

	_, err := repo.GetStats(context.Background())
	if err == nil {
		t.Fatal("GetStats error = nil, want database error")
	}
}
