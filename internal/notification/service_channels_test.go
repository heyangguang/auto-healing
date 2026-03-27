package notification

import (
	"context"
	"errors"
	"testing"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
)

func TestCreateChannelReturnsConflictForDuplicateName(t *testing.T) {
	db := newNotificationTestDB(t)
	createNotificationChannelSchema(t, db)
	svc := newNotificationTestService(db, &fakeNotificationProvider{})
	ctx := platformrepo.WithTenantID(context.Background(), testNotificationTenantID)

	_, err := svc.CreateChannel(ctx, CreateChannelRequest{
		Name: "dup-name",
		Type: "fake",
		Config: map[string]interface{}{
			"url": "http://example.com",
		},
	})
	if err != nil {
		t.Fatalf("CreateChannel() first error = %v", err)
	}

	_, err = svc.CreateChannel(ctx, CreateChannelRequest{
		Name: "dup-name",
		Type: "fake",
		Config: map[string]interface{}{
			"url": "http://example.com",
		},
	})
	if !errors.Is(err, ErrNotificationChannelExists) {
		t.Fatalf("CreateChannel() duplicate error = %v, want %v", err, ErrNotificationChannelExists)
	}
}
