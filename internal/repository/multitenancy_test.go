package repository

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func TestDashboardConfigTenantIsolation(t *testing.T) {
	db := newSQLiteTestDB(t)
	createDashboardSchema(t, db)

	repo := NewDashboardRepositoryWithDB(db)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	if err := repo.UpsertConfig(WithTenantID(context.Background(), tenantA), userID, model.JSON{"layout": "A"}); err != nil {
		t.Fatalf("upsert config tenant A: %v", err)
	}
	if err := repo.UpsertConfig(WithTenantID(context.Background(), tenantB), userID, model.JSON{"layout": "B"}); err != nil {
		t.Fatalf("upsert config tenant B: %v", err)
	}

	var count int64
	if err := db.Table("dashboard_configs").Count(&count).Error; err != nil {
		t.Fatalf("count dashboard configs: %v", err)
	}
	if count != 2 {
		t.Fatalf("dashboard config row count = %d, want 2", count)
	}

	cfgA, err := repo.GetConfigByUserID(WithTenantID(context.Background(), tenantA), userID)
	if err != nil || cfgA == nil || cfgA.Config["layout"] != "A" {
		t.Fatalf("tenant A config = %#v, err = %v", cfgA, err)
	}
	cfgB, err := repo.GetConfigByUserID(WithTenantID(context.Background(), tenantB), userID)
	if err != nil || cfgB == nil || cfgB.Config["layout"] != "B" {
		t.Fatalf("tenant B config = %#v, err = %v", cfgB, err)
	}
}

func TestWorkspaceRepositoryUsesCurrentTenantRoles(t *testing.T) {
	db := newSQLiteTestDB(t)
	createWorkspaceSchema(t, db)

	repo := NewWorkspaceRepositoryWithDB(db)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rolePlatform := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	roleA := uuid.MustParse("00000000-0000-0000-0000-0000000000ab")
	roleB := uuid.MustParse("00000000-0000-0000-0000-0000000000ac")
	now := time.Now().UTC().Format(time.RFC3339)

	wsDefault := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	wsAllowed := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	wsDenied := uuid.MustParse("10000000-0000-0000-0000-000000000003")
	wsPlatform := uuid.MustParse("10000000-0000-0000-0000-000000000004")

	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`, []any{wsDefault.String(), tenantA.String(), "default", true, now, now}},
		{`INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`, []any{wsAllowed.String(), tenantA.String(), "allowed", false, now, now}},
		{`INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`, []any{wsDenied.String(), tenantA.String(), "denied", false, now, now}},
		{`INSERT INTO system_workspaces (id, tenant_id, name, description, config, is_default, created_at, updated_at) VALUES (?, ?, ?, '', '{}', ?, ?, ?)`, []any{wsPlatform.String(), tenantA.String(), "platform", false, now, now}},
		{`INSERT INTO role_workspaces (id, tenant_id, role_id, workspace_id, created_at) VALUES (?, ?, ?, ?, ?)`, []any{uuid.NewString(), tenantA.String(), roleA.String(), wsAllowed.String(), now}},
		{`INSERT INTO role_workspaces (id, tenant_id, role_id, workspace_id, created_at) VALUES (?, ?, ?, ?, ?)`, []any{uuid.NewString(), tenantA.String(), roleB.String(), wsDenied.String(), now}},
		{`INSERT INTO role_workspaces (id, tenant_id, role_id, workspace_id, created_at) VALUES (?, ?, ?, ?, ?)`, []any{uuid.NewString(), tenantA.String(), rolePlatform.String(), wsPlatform.String(), now}},
		{`INSERT INTO user_platform_roles (id, user_id, role_id, created_at) VALUES (?, ?, ?, ?)`, []any{uuid.NewString(), userID.String(), rolePlatform.String(), now}},
		{`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`, []any{uuid.NewString(), userID.String(), tenantA.String(), roleA.String(), now}},
		{`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`, []any{uuid.NewString(), userID.String(), tenantB.String(), roleB.String(), now}},
	} {
		mustExec(t, db, stmt.sql, stmt.args...)
	}

	ctxA := WithTenantID(context.Background(), tenantA)
	roleIDs, err := repo.GetUserRoleIDs(ctxA, userID)
	if err != nil {
		t.Fatalf("GetUserRoleIDs: %v", err)
	}
	gotRoles := map[uuid.UUID]bool{}
	for _, id := range roleIDs {
		gotRoles[id] = true
	}
	if !gotRoles[rolePlatform] || !gotRoles[roleA] || gotRoles[roleB] {
		t.Fatalf("role IDs = %#v, expected platform+tenantA only", roleIDs)
	}

	workspaces, err := repo.GetWorkspacesByUserRoles(ctxA, userID)
	if err != nil {
		t.Fatalf("GetWorkspacesByUserRoles: %v", err)
	}
	gotWS := map[uuid.UUID]bool{}
	for _, ws := range workspaces {
		gotWS[ws.ID] = true
	}
	if !gotWS[wsDefault] || !gotWS[wsAllowed] || !gotWS[wsPlatform] || gotWS[wsDenied] {
		t.Fatalf("workspace visibility = %#v", gotWS)
	}
}

func TestGetUserTenantsReturnsStableOrder(t *testing.T) {
	db := newSQLiteTestDB(t)
	createTenantSchema(t, db)

	repo := &TenantRepository{db: db}
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mustExec(t, db, `INSERT INTO tenants (id, name, code, status) VALUES (?, ?, ?, ?)`, tenantB.String(), "Tenant B", "b", model.TenantStatusActive)
	mustExec(t, db, `INSERT INTO tenants (id, name, code, status) VALUES (?, ?, ?, ?)`, tenantA.String(), "Tenant A", "a", model.TenantStatusActive)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), userID.String(), tenantB.String(), uuid.NewString(), time.Now().UTC().Format(time.RFC3339))
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`, uuid.NewString(), userID.String(), tenantA.String(), uuid.NewString(), time.Now().UTC().Format(time.RFC3339))

	tenants, err := repo.GetUserTenants(context.Background(), userID, "")
	if err != nil {
		t.Fatalf("GetUserTenants: %v", err)
	}
	if len(tenants) != 2 {
		t.Fatalf("tenant count = %d, want 2", len(tenants))
	}
	if tenants[0].ID != tenantA || tenants[1].ID != tenantB {
		t.Fatalf("tenant order = [%s, %s], want [%s, %s]", tenants[0].ID, tenants[1].ID, tenantA, tenantB)
	}
}

func TestCMDBUpsertPreservesTenantScope(t *testing.T) {
	db := newSQLiteTestDB(t)
	createCMDBSchema(t, db)

	repo := &CMDBItemRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ctx := WithTenantID(context.Background(), tenantID)

	item := &model.CMDBItem{
		ID:               uuid.New(),
		PluginID:         &pluginID,
		SourcePluginName: "cmdb-sync",
		ExternalID:       "host-001",
		Name:             "host-one",
		Status:           "active",
		RawData:          model.JSON{"host": "one"},
	}

	isNew, err := repo.UpsertByExternalID(ctx, item)
	if err != nil {
		t.Fatalf("UpsertByExternalID create: %v", err)
	}
	if !isNew {
		t.Fatalf("expected first upsert to create a row")
	}
	if item.TenantID == nil || *item.TenantID != tenantID {
		t.Fatalf("tenant id after create = %v, want %s", item.TenantID, tenantID)
	}

	var row model.CMDBItem
	if err := db.Where("external_id = ?", "host-001").First(&row).Error; err != nil {
		t.Fatalf("query created row: %v", err)
	}
	if row.TenantID == nil || *row.TenantID != tenantID {
		t.Fatalf("stored tenant id = %v, want %s", row.TenantID, tenantID)
	}

	updated := &model.CMDBItem{
		PluginID:         &pluginID,
		SourcePluginName: "cmdb-sync",
		ExternalID:       "host-001",
		Name:             "host-one-updated",
		Status:           "maintenance",
		Dependencies:     model.JSONArray{},
		Tags:             model.JSON{},
		RawData:          model.JSON{"host": "one-updated"},
	}
	isNew, err = repo.UpsertByExternalID(ctx, updated)
	if err != nil {
		t.Fatalf("UpsertByExternalID update: %v", err)
	}
	if isNew {
		t.Fatalf("expected second upsert to update existing row")
	}

	scoped, err := repo.GetByID(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetByID with tenant scope: %v", err)
	}
	if scoped.Name != "host-one-updated" {
		t.Fatalf("updated name = %s, want host-one-updated", scoped.Name)
	}
	if scoped.TenantID == nil || *scoped.TenantID != tenantID {
		t.Fatalf("tenant id after update = %v, want %s", scoped.TenantID, tenantID)
	}
}

func TestDashboardUsersSectionUsesCurrentTenantMembership(t *testing.T) {
	db := newSQLiteTestDB(t)
	createDashboardUsersSchema(t, db)

	repo := NewDashboardRepositoryWithDB(db)
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	roleSystem := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	roleTenantA := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	roleTenantB := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	now := time.Now().UTC().Format(time.RFC3339)

	mustExec(t, db, `INSERT INTO users (id, username, email, display_name, status, last_login_at, last_login_ip, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userA.String(), "tenant-a-user", "a@example.com", "Tenant A User", "active", now, "10.0.0.1", now, now)
	mustExec(t, db, `INSERT INTO users (id, username, email, display_name, status, last_login_at, last_login_ip, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userB.String(), "tenant-b-user", "b@example.com", "Tenant B User", "disabled", now, "10.0.0.2", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, NULL, ?, ?)`,
		roleSystem.String(), "viewer", "Viewer", "tenant", now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		roleTenantA.String(), "tenant-a-custom", "Tenant A Custom", "tenant", tenantA.String(), now, now)
	mustExec(t, db, `INSERT INTO roles (id, name, display_name, scope, tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		roleTenantB.String(), "tenant-b-custom", "Tenant B Custom", "tenant", tenantB.String(), now, now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userA.String(), tenantA.String(), roleSystem.String(), now)
	mustExec(t, db, `INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), userB.String(), tenantB.String(), roleTenantB.String(), now)

	section, err := repo.GetUsersSection(WithTenantID(context.Background(), tenantA))
	if err != nil {
		t.Fatalf("GetUsersSection: %v", err)
	}
	if section.Total != 1 {
		t.Fatalf("section.Total = %d, want 1", section.Total)
	}
	if section.Active != 1 {
		t.Fatalf("section.Active = %d, want 1", section.Active)
	}
	if section.RolesTotal != 2 {
		t.Fatalf("section.RolesTotal = %d, want 2 (system tenant role + tenant A custom role)", section.RolesTotal)
	}
	if len(section.RecentLogins) != 1 || section.RecentLogins[0].ID != userA {
		t.Fatalf("recent logins = %#v, want only tenant A user", section.RecentLogins)
	}
}
