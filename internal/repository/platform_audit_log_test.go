package repository

import (
	"context"
	"testing"

	auditrepo "github.com/company/auto-healing/internal/platform/repository/audit"
)

func TestPlatformAuditGetStatsReturnsDatabaseError(t *testing.T) {
	db := newSQLiteTestDB(t)
	repo := auditrepo.NewPlatformAuditLogRepositoryWithDB(db)

	_, err := repo.GetStats(context.Background())
	if err == nil {
		t.Fatal("GetStats error = nil, want database error")
	}
}
