package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/company/auto-healing/internal/database"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestScheduleGetByIDReturnsNotFoundSentinel(t *testing.T) {
	db := newStateTestDB(t)
	mustExec(t, db, `
		CREATE TABLE execution_schedules (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			task_id TEXT
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())
	_, err := NewScheduleRepository().GetByID(ctx, uuid.New())
	if !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrScheduleNotFound)
	}
}
