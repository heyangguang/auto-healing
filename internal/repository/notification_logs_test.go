package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestGetLogByIDPreloadRespectsTenantScope(t *testing.T) {
	db := newSQLiteTestDB(t)
	createNotificationLogRepositorySchema(t, db)

	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	logID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	templateID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	channelID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	runID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	taskID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	mustExec(t, db, `INSERT INTO notification_templates (id, tenant_id, name, body_template) VALUES (?, ?, ?, ?)`, templateID.String(), tenantB.String(), "tpl-b", "body")
	mustExec(t, db, `INSERT INTO notification_channels (id, tenant_id, name, type, config, recipients, is_active, is_default) VALUES (?, ?, ?, ?, '{}', '[]', 1, 0)`, channelID.String(), tenantB.String(), "channel-b", "email")
	mustExec(t, db, `INSERT INTO execution_tasks (id, tenant_id, name) VALUES (?, ?, ?)`, taskID.String(), tenantB.String(), "task-b")
	mustExec(t, db, `INSERT INTO execution_runs (id, tenant_id, task_id, status, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, runID.String(), tenantB.String(), taskID.String(), "failed")
	mustExec(t, db, `INSERT INTO notification_logs (id, tenant_id, template_id, channel_id, execution_run_id, recipients, body, status, created_at) VALUES (?, ?, ?, ?, ?, '[]', ?, ?, CURRENT_TIMESTAMP)`,
		logID.String(), tenantA.String(), templateID.String(), channelID.String(), runID.String(), "body", "failed")

	repo := &NotificationRepository{db: db}
	log, err := repo.GetLogByID(WithTenantID(context.Background(), tenantA), logID)
	if err != nil {
		t.Fatalf("GetLogByID() error = %v", err)
	}
	if log.Template != nil {
		t.Fatalf("GetLogByID() leaked cross-tenant template: %#v", log.Template)
	}
	if log.Channel != nil {
		t.Fatalf("GetLogByID() leaked cross-tenant channel: %#v", log.Channel)
	}
	if log.ExecutionRun != nil {
		t.Fatalf("GetLogByID() leaked cross-tenant execution run: %#v", log.ExecutionRun)
	}
}

func createNotificationLogRepositorySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE notification_templates (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			body_template TEXT NOT NULL
		);
	`)
	mustExec(t, db, `
		CREATE TABLE notification_channels (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			config TEXT NOT NULL DEFAULT '{}',
			recipients TEXT NOT NULL DEFAULT '[]',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			is_default BOOLEAN NOT NULL DEFAULT FALSE
		);
	`)
	mustExec(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL
		);
	`)
	mustExec(t, db, `
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			task_id TEXT,
			status TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE notification_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			template_id TEXT,
			channel_id TEXT NOT NULL,
			execution_run_id TEXT,
			recipients TEXT NOT NULL DEFAULT '[]',
			body TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME
		);
	`)
}
