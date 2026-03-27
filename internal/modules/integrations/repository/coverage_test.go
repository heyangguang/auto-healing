package repository

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestGitRepositoryListWithOptionsScopesTenantAndPlaybookCounts(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	createIntegrationsRepositorySchema(t, db)

	tenantA := uuid.New()
	tenantB := uuid.New()
	repoA := uuid.New()
	repoB := uuid.New()
	playbookID := uuid.New()
	now := time.Now().UTC()

	mustExecRepositorySQL(t, db, `
		INSERT INTO git_repositories (id, tenant_id, name, url, status, auth_type, sync_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, repoA.String(), tenantA.String(), "repo-a", "https://example.com/a.git", "ready", "token", true, now, now,
		repoB.String(), tenantB.String(), "repo-b", "https://example.com/b.git", "ready", "token", true, now, now)
	mustExecRepositorySQL(t, db, `
		INSERT INTO playbooks (id, tenant_id, repository_id, name, file_path, status, config_mode, variables, scanned_variables, tags, default_extra_vars, default_timeout_minutes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, playbookID.String(), tenantA.String(), repoA.String(), "playbook-a", "site.yml", "ready", "manual", "[]", "[]", "[]", "{}", 60, now, now)

	repo := NewGitRepositoryRepositoryWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantA)

	items, total, err := repo.ListWithOptions(ctx, &GitRepoListOptions{Status: "ready"})
	if err != nil {
		t.Fatalf("ListWithOptions() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("ListWithOptions() = total %d len %d, want 1/1", total, len(items))
	}
	if items[0].ID != repoA || items[0].PlaybookCount != 1 {
		t.Fatalf("repo = %+v, want tenant-scoped repo-a with playbook_count=1", items[0])
	}
}

func TestGitRepositoryUpdateSyncStateAndSyncLogs(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	createIntegrationsRepositorySchema(t, db)

	tenantID := uuid.New()
	repoID := uuid.New()
	logID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	mustExecRepositorySQL(t, db, `
		INSERT INTO git_repositories (id, tenant_id, name, url, status, auth_type, sync_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, repoID.String(), tenantID.String(), "repo-a", "https://example.com/a.git", "pending", "token", true, now, now)

	repo := NewGitRepositoryRepositoryWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	if err := repo.UpdateSyncState(ctx, repoID, "ready", "", "commit-1", &now, nil); err != nil {
		t.Fatalf("UpdateSyncState() error = %v", err)
	}

	var stored model.GitRepository
	if err := db.First(&stored, "id = ?", repoID.String()).Error; err != nil {
		t.Fatalf("load repo after UpdateSyncState: %v", err)
	}
	if stored.Status != "ready" || stored.LastCommitID != "commit-1" || stored.LastSyncAt == nil {
		t.Fatalf("stored repo = %+v, want ready/commit-1/non-nil last_sync_at", stored)
	}

	if err := repo.CreateSyncLog(ctx, &model.GitSyncLog{
		ID:           logID,
		RepositoryID: repoID,
		TriggerType:  "manual",
		Action:       "pull",
		Status:       "success",
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("CreateSyncLog() error = %v", err)
	}

	logs, total, err := repo.ListSyncLogs(ctx, repoID, 1, 10)
	if err != nil {
		t.Fatalf("ListSyncLogs() error = %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].ID != logID {
		t.Fatalf("ListSyncLogs() = total %d logs %+v, want one sync log", total, logs)
	}
}

func TestPlaybookRepositoryListWithOptionsAndScanLogs(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	createIntegrationsRepositorySchema(t, db)

	tenantA := uuid.New()
	tenantB := uuid.New()
	repoA := uuid.New()
	repoB := uuid.New()
	playbookA := uuid.New()
	playbookB := uuid.New()
	scanLogID := uuid.New()
	now := time.Now().UTC()

	mustExecRepositorySQL(t, db, `
		INSERT INTO git_repositories (id, tenant_id, name, url, status, auth_type, sync_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, repoA.String(), tenantA.String(), "repo-a", "https://example.com/a.git", "ready", "token", true, now, now,
		repoB.String(), tenantB.String(), "repo-b", "https://example.com/b.git", "ready", "token", true, now, now)
	mustExecRepositorySQL(t, db, `
		INSERT INTO playbooks (id, tenant_id, repository_id, name, file_path, status, config_mode, variables, scanned_variables, tags, default_extra_vars, default_timeout_minutes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, playbookA.String(), tenantA.String(), repoA.String(), "playbook-a", "site.yml", "ready", "manual", "[]", "[]", "[]", "{}", 60, now, now,
		playbookB.String(), tenantB.String(), repoB.String(), "playbook-b", "other.yml", "pending", "manual", "[]", "[]", "[]", "{}", 60, now, now)

	repo := NewPlaybookRepositoryWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantA)
	items, total, err := repo.ListWithOptions(ctx, &PlaybookListOptions{
		RepositoryID: &repoA,
		Status:       "ready",
		Page:         1,
		PageSize:     10,
	})
	if err != nil {
		t.Fatalf("ListWithOptions() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != playbookA {
		t.Fatalf("ListWithOptions() = total %d items %+v, want tenant-scoped playbook-a", total, items)
	}

	updatedVars := model.JSONArray{"host", "region"}
	scannedVars := model.JSONArray{"host", "region", "env"}
	if err := repo.UpdateVariables(ctx, playbookA, updatedVars, scannedVars); err != nil {
		t.Fatalf("UpdateVariables() error = %v", err)
	}
	if err := repo.UpdateStatus(ctx, playbookA, "active"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if err := repo.CreateScanLog(ctx, &model.PlaybookScanLog{
		ID:          scanLogID,
		PlaybookID:  playbookA,
		TriggerType: "manual",
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("CreateScanLog() error = %v", err)
	}

	var stored model.Playbook
	if err := db.First(&stored, "id = ?", playbookA.String()).Error; err != nil {
		t.Fatalf("load playbook: %v", err)
	}
	if stored.Status != "active" || len(stored.Variables) != 2 || len(stored.ScannedVariables) != 3 {
		t.Fatalf("stored playbook = %+v, want updated status and variables", stored)
	}

	logs, logsTotal, err := repo.ListScanLogs(ctx, playbookA, 1, 10)
	if err != nil {
		t.Fatalf("ListScanLogs() error = %v", err)
	}
	if logsTotal != 1 || len(logs) != 1 || logs[0].ID != scanLogID {
		t.Fatalf("ListScanLogs() = total %d logs %+v, want one scan log", logsTotal, logs)
	}
}

func TestPluginRepositoryAggregateStatsAndSyncInfo(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	createIntegrationsRepositorySchema(t, db)

	tenantA := uuid.New()
	tenantB := uuid.New()
	pluginA := uuid.New()
	pluginB := uuid.New()
	logID := uuid.New()
	now := time.Now().UTC()

	ctx := platformrepo.WithTenantID(context.Background(), tenantA)
	pluginRepo := NewPluginRepositoryWithDB(db)
	mustExecRepositorySQL(t, db, `
		INSERT INTO plugins (id, tenant_id, name, type, version, config, field_mapping, sync_enabled, sync_interval_minutes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, pluginA.String(), tenantA.String(), "itsm-a", "itsm", "1.0.0", "{}", "{}", true, 5, "active", now, now,
		pluginB.String(), tenantA.String(), "cmdb-a", "cmdb", "1.0.0", "{}", "{}", false, 5, "inactive", now, now)
	mustExecRepositorySQL(t, db, `
		INSERT INTO plugins (id, tenant_id, name, type, version, config, field_mapping, sync_enabled, sync_interval_minutes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), tenantB.String(), "other-tenant", "itsm", "1.0.0", "{}", "{}", true, 5, "active", now, now)

	stats, err := pluginRepo.GetAggregateStats(ctx)
	if err != nil {
		t.Fatalf("GetAggregateStats() error = %v", err)
	}
	if stats.Total != 2 || stats.ByType["itsm"] != 1 || stats.ByType["cmdb"] != 1 || stats.SyncEnabled != 1 || stats.SyncDisabled != 1 {
		t.Fatalf("GetAggregateStats() = %+v, want tenant-scoped counts", stats)
	}

	nextSync := now.Add(time.Hour)
	if err := pluginRepo.UpdateSyncInfo(ctx, pluginA, &now, &nextSync); err != nil {
		t.Fatalf("UpdateSyncInfo() error = %v", err)
	}
	syncLogRepo := NewPluginSyncLogRepositoryWithDB(db)
	if err := syncLogRepo.Create(ctx, &model.PluginSyncLog{
		ID:        logID,
		PluginID:  pluginA,
		SyncType:  "manual",
		Status:    "success",
		StartedAt: now,
	}); err != nil {
		t.Fatalf("Create() sync log error = %v", err)
	}

	var stored model.Plugin
	if err := db.First(&stored, "id = ?", pluginA.String()).Error; err != nil {
		t.Fatalf("load plugin: %v", err)
	}
	if stored.LastSyncAt == nil || stored.NextSyncAt == nil {
		t.Fatalf("stored plugin = %+v, want sync timestamps", stored)
	}

	logs, total, err := syncLogRepo.ListByPluginID(ctx, pluginA, 1, 10)
	if err != nil {
		t.Fatalf("ListByPluginID() error = %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].ID != logID {
		t.Fatalf("ListByPluginID() = total %d logs %+v, want one plugin sync log", total, logs)
	}
}
