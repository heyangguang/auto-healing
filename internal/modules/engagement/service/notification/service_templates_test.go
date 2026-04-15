package notification

import (
	"context"
	"errors"
	"testing"

	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"gorm.io/gorm"
)

func TestCreateTemplateNormalizesEmptyEventTypeToManualNotification(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationTemplateSchema(t, db)
	svc := newNotificationTemplateTestService(db)
	ctx := platformrepo.WithTenantID(context.Background(), testNotificationTenantID)

	template, err := svc.CreateTemplate(ctx, CreateTemplateRequest{
		Name:         "tpl-manual-notification",
		BodyTemplate: "hello",
	})
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	if template.EventType != NotificationEventTypeManualNotification {
		t.Fatalf("EventType = %q, want %q", template.EventType, NotificationEventTypeManualNotification)
	}
}

func TestCreateTemplateRejectsUnsupportedEventType(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationTemplateSchema(t, db)
	svc := newNotificationTemplateTestService(db)
	ctx := platformrepo.WithTenantID(context.Background(), testNotificationTenantID)

	_, err := svc.CreateTemplate(ctx, CreateTemplateRequest{
		Name:         "tpl-invalid",
		EventType:    "acceptance.event",
		BodyTemplate: "hello",
	})
	if !errors.Is(err, ErrNotificationUnsupportedEventType) {
		t.Fatalf("CreateTemplate() error = %v, want %v", err, ErrNotificationUnsupportedEventType)
	}
}

func createNotificationTemplateSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecNotification(t, db, `
		CREATE TABLE notification_templates (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			description TEXT,
			event_type TEXT,
			supported_channels TEXT NOT NULL DEFAULT '[]',
			subject_template TEXT,
			body_template TEXT NOT NULL,
			format TEXT NOT NULL DEFAULT 'text',
			available_variables TEXT NOT NULL DEFAULT '[]',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func newNotificationTemplateTestService(db *gorm.DB) *Service {
	return &Service{
		repo:             engagementrepo.NewNotificationRepository(db),
		healingFlowRepo:  nil,
		providerRegistry: nil,
		templateParser:   NewTemplateParser(),
		variableBuilder:  NewVariableBuilder("test", "http://localhost", "v1"),
		rateLimiter:      NewRateLimiter(),
	}
}
