package notification

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/modules/engagement/service/notification/provider"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var testNotificationTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

type fakeNotificationProvider struct {
	sendCalls int
	resp      *provider.SendResponse
	err       error
}

func (p *fakeNotificationProvider) Type() string { return "fake" }

func (p *fakeNotificationProvider) Send(ctx context.Context, req *provider.SendRequest) (*provider.SendResponse, error) {
	p.sendCalls++
	if p.resp != nil {
		return p.resp, p.err
	}
	return &provider.SendResponse{Success: true}, p.err
}

func (p *fakeNotificationProvider) Test(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func TestResolveChannelsRejectsMissingOrInactiveChannels(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationChannelSchema(t, db)
	activeID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	inactiveID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	tenantID := testNotificationTenantID

	insertNotificationChannel(t, db, activeID, tenantID, true)
	insertNotificationChannel(t, db, inactiveID, tenantID, false)

	svc := newNotificationTestService(db, &fakeNotificationProvider{})
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	if _, err := svc.resolveChannels(ctx, []uuid.UUID{activeID, uuid.New()}); err == nil {
		t.Fatal("resolveChannels() error = nil, want missing channel error")
	}
	if _, err := svc.resolveChannels(ctx, []uuid.UUID{inactiveID}); err == nil {
		t.Fatal("resolveChannels() error = nil, want inactive channel error")
	}
}

func TestSendFailsBeforeProviderWhenLogPersistenceFails(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationChannelSchema(t, db)
	channelID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantID := testNotificationTenantID
	insertNotificationChannel(t, db, channelID, tenantID, true)

	fakeProvider := &fakeNotificationProvider{}
	svc := newNotificationTestService(db, fakeProvider)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	logs, err := svc.Send(ctx, SendNotificationRequest{
		ChannelIDs: []uuid.UUID{channelID},
		Body:       "hello",
	})
	if err == nil {
		t.Fatal("Send() error = nil, want persistence error")
	}
	if fakeProvider.sendCalls != 0 {
		t.Fatalf("provider sendCalls = %d, want 0", fakeProvider.sendCalls)
	}
	if len(logs) != 0 {
		t.Fatalf("Send() logs = %#v, want no logs when persistence fails before send", logs)
	}
}

func TestSendReturnsErrorWhenAllChannelsFail(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationChannelSchema(t, db)
	createNotificationLogSchema(t, db)
	channelID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantID := testNotificationTenantID
	insertNotificationChannel(t, db, channelID, tenantID, true)

	fakeProvider := &fakeNotificationProvider{
		resp: &provider.SendResponse{Success: false, ErrorMessage: "downstream failed"},
	}
	svc := newNotificationTestService(db, fakeProvider)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	logs, err := svc.Send(ctx, SendNotificationRequest{
		ChannelIDs: []uuid.UUID{channelID},
		Body:       "hello",
		Format:     "markdown",
	})
	if err == nil {
		t.Fatal("Send() error = nil, want all-failed error")
	}
	if len(logs) != 1 || logs[0].Status != "failed" {
		t.Fatalf("Send() logs = %#v, want one failed log", logs)
	}
	if notificationRequestFormat(logs[0].ResponseData) != "markdown" {
		t.Fatalf("stored request format = %q, want markdown", notificationRequestFormat(logs[0].ResponseData))
	}
}

func TestRetryNotificationFormatUsesStoredRequestFormat(t *testing.T) {
	svc := &Service{}
	format, err := svc.retryNotificationFormat(context.Background(), &model.NotificationLog{
		ResponseData: model.JSON{"request_format": "markdown"},
	})
	if err != nil {
		t.Fatalf("retryNotificationFormat() error = %v", err)
	}
	if format != "markdown" {
		t.Fatalf("retryNotificationFormat() = %q, want markdown", format)
	}
}

func TestRetryFailedReturnsJoinedErrors(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationLogSchema(t, db)
	tenantID := testNotificationTenantID
	mustExecNotification(
		t,
		db,
		`INSERT INTO notification_logs (id, tenant_id, channel_id, recipients, body, status, next_retry_at, created_at) VALUES (?, ?, ?, '[]', ?, 'failed', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		uuid.NewString(),
		tenantID.String(),
		uuid.NewString(),
		"hello",
	)

	svc := newNotificationTestService(db, &fakeNotificationProvider{})
	err := svc.RetryFailed(context.Background())
	if err == nil {
		t.Fatal("RetryFailed() error = nil, want joined retry error")
	}
}

func newNotificationTestService(db *gorm.DB, fake provider.Provider) *Service {
	registry := provider.NewRegistry()
	registry.Register(fake)
	return &Service{
		repo:             engagementrepo.NewNotificationRepository(db),
		providerRegistry: registry,
		templateParser:   NewTemplateParser(),
		variableBuilder:  NewVariableBuilder("test", "http://localhost", "v1"),
		rateLimiter:      NewRateLimiter(),
	}
}

func newNotificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "notification.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createNotificationChannelSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecNotification(t, db, `
		CREATE TABLE notification_channels (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT,
			config TEXT NOT NULL DEFAULT '{}',
			retry_config TEXT,
			recipients TEXT NOT NULL DEFAULT '[]',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			rate_limit_per_minute INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExecNotification(t, db, `CREATE UNIQUE INDEX idx_channel_tenant_name ON notification_channels(tenant_id, name);`)
}

func createNotificationLogSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecNotification(t, db, `
		CREATE TABLE notification_logs (
			id TEXT PRIMARY KEY NOT NULL DEFAULT (
				lower(hex(randomblob(4))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(2))) || '-' ||
				lower(hex(randomblob(6)))
			),
			tenant_id TEXT,
			template_id TEXT,
			channel_id TEXT NOT NULL,
			execution_run_id TEXT,
			workflow_instance_id TEXT,
			incident_id TEXT,
			recipients TEXT NOT NULL DEFAULT '[]',
			subject TEXT,
			body TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			external_message_id TEXT,
			response_data TEXT,
			error_message TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			next_retry_at DATETIME,
			sent_at DATETIME,
			created_at DATETIME
		);
	`)
}

func insertNotificationChannel(t *testing.T, db *gorm.DB, channelID, tenantID uuid.UUID, active bool) {
	t.Helper()
	mustExecNotification(
		t,
		db,
		`INSERT INTO notification_channels (id, tenant_id, name, type, config, recipients, is_active, is_default) VALUES (?, ?, ?, ?, '{}', '[]', ?, false)`,
		channelID.String(),
		tenantID.String(),
		fmt.Sprintf("channel-%s", channelID.String()[:8]),
		"fake",
		active,
	)
}

func mustExecNotification(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

var _ provider.Provider = (*fakeNotificationProvider)(nil)
var _ = model.NotificationLog{}
