package audit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

func TestTenantScopeHelpersRoundTrip(t *testing.T) {
	tenantID := uuid.New()
	ctx := WithTenantID(context.Background(), tenantID)

	got, ok := TenantIDFromContextOK(ctx)
	if !ok {
		t.Fatal("TenantIDFromContextOK() ok = false, want true")
	}
	if got != tenantID {
		t.Fatalf("TenantIDFromContextOK() = %s, want %s", got, tenantID)
	}
}

func TestOrderClauseAndApplyDaysFilter(t *testing.T) {
	if got := orderClause("username", "asc", map[string]bool{"username": true}); got != "username ASC" {
		t.Fatalf("orderClause() = %q, want %q", got, "username ASC")
	}
	if got := orderClause("forbidden", "DESC", map[string]bool{"username": true}); got != "created_at DESC" {
		t.Fatalf("orderClause() default = %q, want %q", got, "created_at DESC")
	}

	db := newAuditDryRunDB(t)
	stmt := applyDaysFilter(db.Model(&platformmodel.PlatformAuditLog{}), 7).
		Find(&[]platformmodel.PlatformAuditLog{}).Statement
	if !strings.Contains(stmt.SQL.String(), "created_at >= ?") {
		t.Fatalf("applyDaysFilter SQL = %q, want created_at filter", stmt.SQL.String())
	}

	stmt = applyDaysFilter(db.Model(&platformmodel.PlatformAuditLog{}), 0).
		Find(&[]platformmodel.PlatformAuditLog{}).Statement
	if strings.Contains(stmt.SQL.String(), "created_at >= ?") {
		t.Fatalf("applyDaysFilter SQL = %q, want no created_at filter", stmt.SQL.String())
	}
}

func TestApplyAuditFiltersBuildsExpectedSQL(t *testing.T) {
	db := newAuditDryRunDB(t)
	userID := uuid.New()
	createdAfter := fixedAuditTime(-2 * time.Hour)
	createdBefore := fixedAuditTime(2 * time.Hour)
	opts := &AuditLogListOptions{
		Category:             "operation",
		Action:               "assign_role",
		ResourceType:         "tenant-users",
		ExcludeActions:       []string{"login"},
		ExcludeResourceTypes: []string{"tokens"},
		Username:             query.StringFilter{Value: "alice", Exact: true},
		UserID:               &userID,
		Status:               "success",
		RiskLevel:            "high",
		RequestPath:          query.StringFilter{Value: "/api/v1/tenant/users", Exact: true},
		Search:               query.StringFilter{Value: "alice"},
		CreatedAfter:         &createdAfter,
		CreatedBefore:        &createdBefore,
	}

	stmt := applyAuditLogFilters(db.Model(&platformmodel.AuditLog{}), opts).
		Order(auditLogOrderClause(opts)).
		Find(&[]platformmodel.AuditLog{}).Statement
	sql := stmt.SQL.String()

	for _, fragment := range []string{
		"category = ?",
		"action = ?",
		"resource_type = ?",
		"action NOT IN",
		"resource_type NOT IN",
		"status = ?",
		"username = ?",
		"user_id = ?",
		"request_path = ?",
		"created_at >= ?",
		"created_at <= ?",
		"assign_role",
		"ORDER BY created_at DESC",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("applyAuditLogFilters SQL missing %q in %q", fragment, sql)
		}
	}
}

func TestApplyAuditRiskFilterNormalBuildsNegatedCondition(t *testing.T) {
	db := newAuditDryRunDB(t)
	stmt := applyAuditRiskFilter(db.Model(&platformmodel.AuditLog{}), "normal").
		Find(&[]platformmodel.AuditLog{}).Statement
	if !strings.Contains(stmt.SQL.String(), "NOT (") {
		t.Fatalf("applyAuditRiskFilter(normal) SQL = %q, want NOT (...)", stmt.SQL.String())
	}
}

func TestApplyPlatformAuditFiltersBuildsExpectedSQL(t *testing.T) {
	db := newAuditDryRunDB(t)
	userID := uuid.New()
	createdAfter := fixedAuditTime(-1 * time.Hour)
	opts := &PlatformAuditListOptions{
		Category:     "login",
		Action:       "delete",
		ResourceType: "users",
		Username:     query.StringFilter{Value: "root", Exact: true},
		UserID:       &userID,
		Status:       "failed",
		RequestPath:  query.StringFilter{Value: "/api/v1/platform/users", Exact: true},
		Search:       query.StringFilter{Value: "root"},
		CreatedAfter: &createdAfter,
		SortBy:       "category",
		SortOrder:    "asc",
	}

	stmt := applyPlatformAuditFilters(db.Model(&platformmodel.PlatformAuditLog{}), opts).
		Order(platformAuditOrderClause(opts)).
		Find(&[]platformmodel.PlatformAuditLog{}).Statement
	sql := stmt.SQL.String()

	for _, fragment := range []string{
		"category = ?",
		"action = ?",
		"resource_type = ?",
		"username = ?",
		"user_id = ?",
		"status = ?",
		"request_path = ?",
		"created_at >= ?",
		"ORDER BY category ASC",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("applyPlatformAuditFilters SQL missing %q in %q", fragment, sql)
		}
	}
}
