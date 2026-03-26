package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestCountTasksUsingNotificationResourcesChecksNestedTriggers(t *testing.T) {
	db := newSQLiteTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			notification_config TEXT NOT NULL DEFAULT '{}'
		);
	`)

	repo := &NotificationRepository{db: db}
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	templateID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	channelID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	ctx := WithTenantID(context.Background(), tenantA)

	mustExec(t, db, `INSERT INTO execution_tasks (id, tenant_id, notification_config) VALUES (?, ?, ?)`,
		uuid.NewString(),
		tenantA.String(),
		`{"enabled":true,"on_success":{"enabled":true,"template_id":"`+templateID.String()+`","channel_ids":["`+channelID.String()+`"]}}`,
	)
	mustExec(t, db, `INSERT INTO execution_tasks (id, tenant_id, notification_config) VALUES (?, ?, ?)`,
		uuid.NewString(),
		tenantB.String(),
		`{"enabled":true,"on_failure":{"enabled":true,"template_id":"`+templateID.String()+`","channel_ids":["`+channelID.String()+`"]}}`,
	)

	templateCount, err := repo.CountTasksUsingTemplate(ctx, templateID)
	if err != nil {
		t.Fatalf("CountTasksUsingTemplate() error = %v", err)
	}
	if templateCount != 1 {
		t.Fatalf("CountTasksUsingTemplate() = %d, want 1", templateCount)
	}

	channelCount, err := repo.CountTasksUsingChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("CountTasksUsingChannel() error = %v", err)
	}
	if channelCount != 1 {
		t.Fatalf("CountTasksUsingChannel() = %d, want 1", channelCount)
	}
}
