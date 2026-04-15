package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestGetNotificationSectionUsesFreshQueryState(t *testing.T) {
	db := newSQLiteTestDB(t)
	createNotificationDashboardSchema(t, db)

	repo := NewDashboardRepositoryWithDB(db)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	section, err := repo.GetNotificationSection(WithTenantID(context.Background(), tenantID))
	if err != nil {
		t.Fatalf("GetNotificationSection() error = %v", err)
	}
	if section.ChannelsTotal != 0 || section.TemplatesTotal != 0 || section.LogsTotal != 0 {
		t.Fatalf("unexpected non-zero notification counts: %#v", section)
	}
	if len(section.ByChannelType) != 0 || len(section.ByLogStatus) != 0 || len(section.RecentLogs) != 0 || len(section.FailedLogs) != 0 {
		t.Fatalf("unexpected notification detail rows: %#v", section)
	}
}

func createNotificationDashboardSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE notification_channels (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			type TEXT,
			description TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE notification_templates (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			created_at DATETIME
		);
	`)
	mustExec(t, db, `
		CREATE TABLE notification_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			subject TEXT,
			status TEXT,
			created_at DATETIME
		);
	`)
}
