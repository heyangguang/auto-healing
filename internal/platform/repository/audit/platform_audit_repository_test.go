package audit

import (
	"context"
	"testing"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

func TestPlatformAuditRepositoryListStatsAndUserQueries(t *testing.T) {
	db := newAuditTestDB(t)
	repo := NewPlatformAuditLogRepositoryWithDB(db)
	userID := uuid.New()

	createAuditUser(t, db, userID, "root")

	loginID := uuid.New()
	highRiskID := uuid.New()
	insertPlatformAuditLog(t, db, platformmodel.PlatformAuditLog{
		ID:           loginID,
		UserID:       &userID,
		Username:     "root",
		Category:     "login",
		Action:       "login",
		ResourceType: "session",
		RequestPath:  "/api/v1/auth/login",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})
	insertPlatformAuditLog(t, db, platformmodel.PlatformAuditLog{
		ID:           highRiskID,
		UserID:       &userID,
		Username:     "root",
		Category:     "operation",
		Action:       "delete",
		ResourceType: "users",
		RequestPath:  "/api/v1/platform/users",
		Status:       "failed",
		CreatedAt:    fixedAuditTime(0),
	})
	insertPlatformAuditLog(t, db, platformmodel.PlatformAuditLog{
		ID:           uuid.New(),
		UserID:       &userID,
		Username:     "root",
		Category:     "operation",
		Action:       "update",
		ResourceType: "settings",
		RequestPath:  "/api/v1/platform/settings",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})

	created := &platformmodel.PlatformAuditLog{
		ID:           uuid.New(),
		UserID:       &userID,
		Username:     "root",
		Category:     "operation",
		Action:       "create",
		ResourceType: "roles",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	}
	if err := repo.Create(context.Background(), created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got == nil || got.ID != created.ID {
		t.Fatalf("GetByID() = %#v, want created log", got)
	}

	logs, total, err := repo.List(context.Background(), &PlatformAuditListOptions{
		Page:     1,
		PageSize: 10,
		Category: "operation",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 3 || len(logs) != 3 {
		t.Fatalf("List() total=%d len=%d, want 3/3", total, len(logs))
	}

	stats, err := repo.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.TotalCount != 4 || stats.LoginCount != 1 || stats.OperCount != 3 || stats.SuccessCount != 3 || stats.FailedCount != 1 {
		t.Fatalf("GetStats() = %#v", stats)
	}

	rankings, err := repo.GetUserRanking(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("GetUserRanking() error = %v", err)
	}
	if len(rankings) != 1 || rankings[0].Username != "root" || rankings[0].Count != 4 {
		t.Fatalf("GetUserRanking() = %#v", rankings)
	}

	loginHistory, err := repo.GetUserLoginHistory(context.Background(), userID, 0)
	if err != nil {
		t.Fatalf("GetUserLoginHistory() error = %v", err)
	}
	if len(loginHistory) != 1 || loginHistory[0].ID != loginID {
		t.Fatalf("GetUserLoginHistory() = %#v", loginHistory)
	}

	activities, err := repo.GetUserActivities(context.Background(), userID, 0)
	if err != nil {
		t.Fatalf("GetUserActivities() error = %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("GetUserActivities() len = %d, want 3", len(activities))
	}

	resourceStats, err := repo.GetResourceTypeStats(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetResourceTypeStats() error = %v", err)
	}
	if len(resourceStats) == 0 {
		t.Fatal("GetResourceTypeStats() returned no rows")
	}

	grouping, err := repo.GetActionGrouping(context.Background(), "delete", 0)
	if err != nil {
		t.Fatalf("GetActionGrouping() error = %v", err)
	}
	if len(grouping) != 1 || grouping[0].Action != "delete" {
		t.Fatalf("GetActionGrouping() = %#v", grouping)
	}

	highRiskLogs, highRiskTotal, err := repo.GetHighRiskLogs(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("GetHighRiskLogs() error = %v", err)
	}
	if highRiskTotal != 1 || len(highRiskLogs) != 1 || highRiskLogs[0].ID != highRiskID {
		t.Fatalf("GetHighRiskLogs() total=%d logs=%#v", highRiskTotal, highRiskLogs)
	}
}

func TestPlatformAuditGetTrendSurfacesDialectError(t *testing.T) {
	db := newAuditTestDB(t)
	repo := NewPlatformAuditLogRepositoryWithDB(db)
	_, err := repo.GetTrend(context.Background(), 7)
	if err == nil {
		t.Fatal("GetTrend() error = nil, want SQL dialect error")
	}
}
