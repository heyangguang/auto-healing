package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func createCommandBlacklistSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE command_blacklist (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			pattern TEXT NOT NULL,
			match_type TEXT NOT NULL,
			severity TEXT NOT NULL,
			category TEXT,
			description TEXT,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			is_system BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE tenant_blacklist_overrides (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func TestCommandBlacklistGetByIDAppliesTenantOverride(t *testing.T) {
	db := newSQLiteTestDB(t)
	createCommandBlacklistSchema(t, db)

	repo := &CommandBlacklistRepository{db: db, cacheTTL: time.Minute, cache: map[string][]model.CommandBlacklist{}, cacheTime: map[string]time.Time{}}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ruleID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	overrideID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ruleID.String(), "删除数据库", "DROP DATABASE", "contains", "critical", "database", "system rule", false, true, time.Now(), time.Now())
	mustExec(t, db, `
		INSERT INTO tenant_blacklist_overrides (id, tenant_id, rule_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, overrideID.String(), tenantID.String(), ruleID.String(), true, time.Now(), time.Now())

	ctx := WithTenantID(context.Background(), tenantID)
	rule, err := repo.GetByID(ctx, ruleID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if rule == nil {
		t.Fatalf("expected rule")
	}
	if !rule.IsActive {
		t.Fatalf("expected tenant override to mark system rule active")
	}
}

func TestCommandBlacklistListFiltersAndPaginatesAfterOverrides(t *testing.T) {
	db := newSQLiteTestDB(t)
	createCommandBlacklistSchema(t, db)

	repo := &CommandBlacklistRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	systemRuleA := uuid.MustParse("22222222-2222-2222-2222-222222222221")
	systemRuleB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	now := time.Now()
	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, systemRuleA.String(), "系统规则 A", "rm -rf /", "contains", "critical", "system", "a", false, true, now, now)
	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, systemRuleB.String(), "系统规则 B", "shutdown", "contains", "critical", "system", "b", false, true, now.Add(time.Second), now.Add(time.Second))
	mustExec(t, db, `
		INSERT INTO tenant_blacklist_overrides (id, tenant_id, rule_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), systemRuleA.String(), true, now, now)

	ctx := WithTenantID(context.Background(), tenantID)
	active := true
	items, total, err := repo.List(ctx, &CommandBlacklistListOptions{
		Page:     1,
		PageSize: 10,
		IsActive: &active,
	})
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if total != 1 {
		t.Fatalf("active total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != systemRuleA {
		t.Fatalf("active items = %#v, want only overridden active rule", items)
	}

	inactive := false
	items, total, err = repo.List(ctx, &CommandBlacklistListOptions{
		Page:     1,
		PageSize: 1,
		IsActive: &inactive,
	})
	if err != nil {
		t.Fatalf("List inactive: %v", err)
	}
	if total != 1 {
		t.Fatalf("inactive total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].ID != systemRuleB {
		t.Fatalf("inactive items = %#v, want only non-overridden inactive rule", items)
	}
}

func TestCommandBlacklistGetActiveRulesHonorsDefaultStateAndOverrides(t *testing.T) {
	db := newSQLiteTestDB(t)
	createCommandBlacklistSchema(t, db)

	repo := &CommandBlacklistRepository{db: db}
	tenantID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	now := time.Now()

	tenantRuleID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1")
	systemDefaultID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2")
	systemEnabledID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb3")
	systemDisabledID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb4")

	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tenantRuleID.String(), tenantID.String(), "租户规则", "curl", "contains", "high", "network", "tenant", true, false, now, now)
	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, systemDefaultID.String(), "默认启用系统规则", "rm -rf /", "contains", "critical", "system", "default active", true, true, now, now)
	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, systemEnabledID.String(), "被 override 启用", "shutdown", "contains", "critical", "system", "override active", false, true, now, now)
	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, systemDisabledID.String(), "被 override 禁用", "reboot", "contains", "critical", "system", "override disabled", true, true, now, now)
	mustExec(t, db, `
		INSERT INTO tenant_blacklist_overrides (id, tenant_id, rule_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), systemEnabledID.String(), true, now, now)
	mustExec(t, db, `
		INSERT INTO tenant_blacklist_overrides (id, tenant_id, rule_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), systemDisabledID.String(), false, now, now)

	rules, err := repo.GetActiveRules(WithTenantID(context.Background(), tenantID))
	if err != nil {
		t.Fatalf("GetActiveRules: %v", err)
	}

	got := make(map[uuid.UUID]bool, len(rules))
	for _, rule := range rules {
		got[rule.ID] = true
	}
	for _, wantID := range []uuid.UUID{tenantRuleID, systemDefaultID, systemEnabledID} {
		if !got[wantID] {
			t.Fatalf("GetActiveRules missing %s: %#v", wantID, rules)
		}
	}
	if got[systemDisabledID] {
		t.Fatalf("GetActiveRules should exclude disabled override rule: %#v", rules)
	}
}

func TestCommandBlacklistGetByIDPropagatesOverrideLookupError(t *testing.T) {
	db := newSQLiteTestDB(t)
	mustExec(t, db, `
		CREATE TABLE command_blacklist (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			pattern TEXT NOT NULL,
			match_type TEXT NOT NULL,
			severity TEXT NOT NULL,
			category TEXT,
			description TEXT,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			is_system BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)

	repo := &CommandBlacklistRepository{db: db}
	tenantID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	ruleID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	now := time.Now()
	mustExec(t, db, `
		INSERT INTO command_blacklist (id, tenant_id, name, pattern, match_type, severity, category, description, is_active, is_system, created_at, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ruleID.String(), "系统规则", "DROP DATABASE", "contains", "critical", "database", "system", false, true, now, now)

	_, err := repo.GetByID(WithTenantID(context.Background(), tenantID), ruleID)
	if err == nil {
		t.Fatal("GetByID error = nil, want override lookup error")
	}
	if !strings.Contains(err.Error(), "tenant_blacklist_overrides") {
		t.Fatalf("GetByID error = %v, want missing override table detail", err)
	}
}
