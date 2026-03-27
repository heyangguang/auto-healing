package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestDashboardSectionsPreloadTenantScopedRelations(t *testing.T) {
	db := newSQLiteTestDB(t)
	createDashboardPreloadScopeSchema(t, db)

	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Now().UTC().Format(time.RFC3339)

	repoID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	taskID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	runID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	syncID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	mustExec(t, db, `INSERT INTO git_repositories (id, tenant_id, name, url, status, default_branch, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, repoID.String(), tenantB.String(), "repo-b", "https://b", "active", "main", now)
	mustExec(t, db, `INSERT INTO git_sync_logs (id, tenant_id, repository_id, status, created_at) VALUES (?, ?, ?, ?, ?)`, syncID.String(), tenantA.String(), repoID.String(), "success", now)
	mustExec(t, db, `INSERT INTO execution_tasks (id, tenant_id, name, created_at) VALUES (?, ?, ?, ?)`, taskID.String(), tenantB.String(), "task-b", now)
	mustExec(t, db, `INSERT INTO execution_runs (id, tenant_id, task_id, status, created_at) VALUES (?, ?, ?, ?, ?)`, runID.String(), tenantA.String(), taskID.String(), "failed", now)

	repo := &DashboardRepository{db: db}
	ctx := WithTenantID(context.Background(), tenantA)

	gitSection, err := repo.GetGitSection(ctx)
	if err != nil {
		t.Fatalf("GetGitSection() error = %v", err)
	}
	if len(gitSection.RecentSyncs) != 1 {
		t.Fatalf("RecentSyncs len = %d, want 1", len(gitSection.RecentSyncs))
	}
	if gitSection.RecentSyncs[0].RepoName != "" {
		t.Fatalf("GetGitSection() leaked cross-tenant repo name: %#v", gitSection.RecentSyncs[0])
	}

	execSection, err := repo.GetExecutionSection(ctx)
	if err != nil {
		t.Fatalf("GetExecutionSection() error = %v", err)
	}
	if len(execSection.RecentRuns) != 1 {
		t.Fatalf("RecentRuns len = %d, want 1", len(execSection.RecentRuns))
	}
	if execSection.RecentRuns[0].TaskName != "" {
		t.Fatalf("GetExecutionSection() leaked cross-tenant task name: %#v", execSection.RecentRuns[0])
	}
	if len(execSection.TaskTop10) != 0 {
		t.Fatalf("GetExecutionSection() leaked cross-tenant top task: %#v", execSection.TaskTop10)
	}
}

func createDashboardPreloadScopeSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			url TEXT,
			auth_type TEXT,
			auth_config TEXT,
			local_path TEXT,
			branches TEXT,
			status TEXT,
			default_branch TEXT,
			last_sync_at DATETIME,
			last_commit_id TEXT,
			error_message TEXT,
			sync_enabled BOOLEAN,
			sync_interval TEXT,
			next_sync_at DATETIME,
			max_failures INTEGER,
			consecutive_failures INTEGER,
			pause_reason TEXT,
			updated_at DATETIME,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE git_sync_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			repository_id TEXT,
			status TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			playbook_id TEXT,
			target_hosts TEXT,
			extra_vars TEXT,
			executor_type TEXT,
			description TEXT,
			secrets_source_ids TEXT,
			notification_config TEXT,
			playbook_variables_snapshot TEXT,
			needs_review BOOLEAN,
			changed_variables TEXT,
			updated_at DATETIME,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			task_id TEXT,
			status TEXT,
			exit_code INTEGER,
			stats TEXT,
			stdout TEXT,
			stderr TEXT,
			triggered_by TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			runtime_target_hosts TEXT,
			runtime_secrets_source_ids TEXT,
			runtime_extra_vars TEXT,
			runtime_skip_notification BOOLEAN,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE execution_schedules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			schedule_type TEXT,
			enabled BOOLEAN
		);
	`)
}
