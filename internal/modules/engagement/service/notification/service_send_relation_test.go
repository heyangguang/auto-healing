package notification

import (
	"context"
	"testing"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestSendPersistsWorkflowAndIncidentRelations(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationChannelSchema(t, db)
	createNotificationLogSchema(t, db)
	channelID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantID := testNotificationTenantID
	workflowInstanceID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	incidentID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	insertNotificationChannel(t, db, channelID, tenantID, true)

	svc := newNotificationTestService(db, &fakeNotificationProvider{})
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	logs, err := svc.Send(ctx, SendNotificationRequest{
		ChannelIDs:         []uuid.UUID{channelID},
		Body:               "hello",
		WorkflowInstanceID: &workflowInstanceID,
		IncidentID:         &incidentID,
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("Send() logs len = %d, want 1", len(logs))
	}
	stored := loadNotificationLog(t, db, logs[0].ID)
	if stored.WorkflowInstanceID == nil || *stored.WorkflowInstanceID != workflowInstanceID {
		t.Fatalf("WorkflowInstanceID = %v, want %s", stored.WorkflowInstanceID, workflowInstanceID)
	}
	if stored.IncidentID == nil || *stored.IncidentID != incidentID {
		t.Fatalf("IncidentID = %v, want %s", stored.IncidentID, incidentID)
	}
}

func loadNotificationLog(t *testing.T, db *gorm.DB, id uuid.UUID) notificationLogRow {
	t.Helper()
	var row notificationLogRow
	if err := db.Raw(`
		SELECT workflow_instance_id, incident_id
		FROM notification_logs
		WHERE id = ?
	`, id.String()).Scan(&row).Error; err != nil {
		t.Fatalf("load notification log: %v", err)
	}
	return row
}

type notificationLogRow struct {
	WorkflowInstanceID *uuid.UUID
	IncidentID         *uuid.UUID
}
