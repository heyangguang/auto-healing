package audit

import (
	"context"
	"testing"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

func TestAuditLogRepositoryCreateGetListAndStats(t *testing.T) {
	db := newAuditTestDB(t)
	repo := NewAuditLogRepository(db)
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	userID := uuid.New()

	createAuditUser(t, db, userID, "alice")

	ctx := WithTenantID(context.Background(), tenantID)
	loginID := uuid.New()
	highRiskID := uuid.New()
	normalID := uuid.New()
	insertAuditLog(t, db, platformmodel.AuditLog{
		ID:           loginID,
		TenantID:     &tenantID,
		UserID:       &userID,
		Username:     "alice",
		Category:     "login",
		Action:       "login",
		ResourceType: "session",
		RequestPath:  "/api/v1/auth/login",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})
	insertAuditLog(t, db, platformmodel.AuditLog{
		ID:           highRiskID,
		TenantID:     &tenantID,
		UserID:       &userID,
		Username:     "alice",
		Category:     "operation",
		Action:       "assign_role",
		ResourceType: "tenant-users",
		RequestPath:  "/api/v1/tenant/users/roles",
		Status:       "failed",
		CreatedAt:    fixedAuditTime(0),
	})
	insertAuditLog(t, db, platformmodel.AuditLog{
		ID:           normalID,
		TenantID:     &tenantID,
		UserID:       &userID,
		Username:     "alice",
		Category:     "operation",
		Action:       "create",
		ResourceType: "plugins",
		RequestPath:  "/api/v1/tenant/plugins",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})
	insertAuditLog(t, db, platformmodel.AuditLog{
		ID:           uuid.New(),
		TenantID:     &otherTenantID,
		Username:     "bob",
		Category:     "operation",
		Action:       "delete",
		ResourceType: "users",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})

	created := &platformmodel.AuditLog{
		ID:           uuid.New(),
		UserID:       &userID,
		Username:     "alice",
		Category:     "operation",
		Action:       "update",
		ResourceType: "workspaces",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.TenantID == nil || *created.TenantID != tenantID {
		t.Fatalf("Create() tenant fill = %#v, want %s", created.TenantID, tenantID)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got == nil || got.ID != created.ID {
		t.Fatalf("GetByID() = %#v, want created log", got)
	}

	missing, err := repo.GetByID(WithTenantID(context.Background(), otherTenantID), created.ID)
	if err != nil {
		t.Fatalf("GetByID(other tenant) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("GetByID(other tenant) = %#v, want nil", missing)
	}

	logs, total, err := repo.List(ctx, &AuditLogListOptions{Page: 1, PageSize: 10, Category: "operation"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 3 || len(logs) != 3 {
		t.Fatalf("List() total=%d len=%d, want 3/3", total, len(logs))
	}

	stats, err := repo.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.TotalCount != 4 || stats.SuccessCount != 3 || stats.FailedCount != 1 || stats.HighRiskCount != 1 {
		t.Fatalf("GetStats() = %#v", stats)
	}

	rankings, err := repo.GetUserRanking(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetUserRanking() error = %v", err)
	}
	if len(rankings) != 1 || rankings[0].Username != "alice" || rankings[0].Count != 4 {
		t.Fatalf("GetUserRanking() = %#v", rankings)
	}

	grouping, err := repo.GetActionGrouping(ctx, "assign_role", 0)
	if err != nil {
		t.Fatalf("GetActionGrouping() error = %v", err)
	}
	if len(grouping) != 1 || grouping[0].Action != "assign_role" {
		t.Fatalf("GetActionGrouping() = %#v", grouping)
	}

	resourceStats, err := repo.GetResourceTypeStats(ctx, 0)
	if err != nil {
		t.Fatalf("GetResourceTypeStats() error = %v", err)
	}
	if len(resourceStats) == 0 {
		t.Fatal("GetResourceTypeStats() returned no rows")
	}

	highRiskLogs, highRiskTotal, err := repo.GetHighRiskLogs(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetHighRiskLogs() error = %v", err)
	}
	if highRiskTotal != 1 || len(highRiskLogs) != 1 || highRiskLogs[0].ID != highRiskID {
		t.Fatalf("GetHighRiskLogs() total=%d logs=%#v", highRiskTotal, highRiskLogs)
	}

	loginHistory, err := repo.GetUserLoginHistory(ctx, userID, tenantID, 0)
	if err != nil {
		t.Fatalf("GetUserLoginHistory() error = %v", err)
	}
	if len(loginHistory) != 1 || loginHistory[0].ID != loginID {
		t.Fatalf("GetUserLoginHistory() = %#v", loginHistory)
	}

	activities, err := repo.GetUserActivities(ctx, userID, tenantID, 0)
	if err != nil {
		t.Fatalf("GetUserActivities() error = %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("GetUserActivities() len = %d, want 3", len(activities))
	}
}

func TestAuditLogRepositoryRequiresTenantContext(t *testing.T) {
	db := newAuditTestDB(t)
	repo := NewAuditLogRepository(db)

	_, _, err := repo.List(context.Background(), &AuditLogListOptions{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("List() error = nil, want tenant context error")
	}
}

func TestAuditLogRepositoryListsTenantVisiblePlatformLoginLogs(t *testing.T) {
	db := newAuditTestDB(t)
	repo := NewAuditLogRepository(db)
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()

	createAuditUser(t, db, userID, "alice")
	createAuditUser(t, db, otherUserID, "bob")

	mustExecAuditRepoSQL(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userID.String(), tenantID.String(), uuid.NewString(), fixedAuditTime(0))
	mustExecAuditRepoSQL(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), otherUserID.String(), otherTenantID.String(), uuid.NewString(), fixedAuditTime(0))

	loginID := uuid.New()
	insertPlatformAuditLog(t, db, platformmodel.PlatformAuditLog{
		ID:           loginID,
		UserID:       &userID,
		Username:     "alice",
		Category:     "login",
		Action:       "login",
		ResourceType: "auth",
		RequestPath:  "/api/v1/auth/login",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})
	insertPlatformAuditLog(t, db, platformmodel.PlatformAuditLog{
		ID:           uuid.New(),
		UserID:       &otherUserID,
		Username:     "bob",
		Category:     "login",
		Action:       "login",
		ResourceType: "auth",
		RequestPath:  "/api/v1/auth/login",
		Status:       "success",
		CreatedAt:    fixedAuditTime(0),
	})

	ctx := WithTenantID(context.Background(), tenantID)
	logs, total, err := repo.List(ctx, &AuditLogListOptions{Page: 1, PageSize: 10, Category: "login"})
	if err != nil {
		t.Fatalf("List(login) error = %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].ID != loginID {
		t.Fatalf("List(login) total=%d logs=%#v, want one tenant-visible login log", total, logs)
	}
	if logs[0].TenantID != nil {
		t.Fatalf("List(login) tenant_id = %#v, want nil projection", logs[0].TenantID)
	}

	got, err := repo.GetByID(ctx, loginID)
	if err != nil {
		t.Fatalf("GetByID(login projection) error = %v", err)
	}
	if got == nil || got.ID != loginID {
		t.Fatalf("GetByID(login projection) = %#v, want %s", got, loginID)
	}

	missing, err := repo.GetByID(WithTenantID(context.Background(), otherTenantID), loginID)
	if err != nil {
		t.Fatalf("GetByID(other tenant login projection) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("GetByID(other tenant login projection) = %#v, want nil", missing)
	}
}

func TestAuditLogGetTrendSurfacesDialectError(t *testing.T) {
	db := newAuditTestDB(t)
	repo := NewAuditLogRepository(db)
	_, err := repo.GetTrend(WithTenantID(context.Background(), uuid.New()), 7)
	if err == nil {
		t.Fatal("GetTrend() error = nil, want SQL dialect error")
	}
}
