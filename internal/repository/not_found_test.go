package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/database"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGitRepositoryGetByIDReturnsNotFoundSentinel(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	mustExecRepositorySQL(t, db, `
		CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			url TEXT
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())
	_, err := integrationrepo.NewGitRepositoryRepository().GetByID(ctx, uuid.New())
	if !errors.Is(err, integrationrepo.ErrGitRepositoryNotFound) {
		t.Fatalf("GetByID() error = %v, want %v", err, integrationrepo.ErrGitRepositoryNotFound)
	}
}

func TestPlaybookGetByIDReturnsNotFoundSentinel(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	mustExecRepositorySQL(t, db, `
		CREATE TABLE playbooks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			repository_id TEXT
		);
	`)

	originalDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = originalDB })

	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())
	_, err := integrationrepo.NewPlaybookRepository().GetByID(ctx, uuid.New())
	if !errors.Is(err, integrationrepo.ErrPlaybookNotFound) {
		t.Fatalf("GetByID() error = %v, want %v", err, integrationrepo.ErrPlaybookNotFound)
	}
}

func TestScheduleGetByIDReturnsNotFoundSentinel(t *testing.T) {
	db := openRepositoryLookupTestDB(t)
	mustExecRepositorySQL(t, db, `
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
	_, err := automationrepo.NewScheduleRepository().GetByID(ctx, uuid.New())
	if !errors.Is(err, automationrepo.ErrScheduleNotFound) {
		t.Fatalf("GetByID() error = %v, want %v", err, automationrepo.ErrScheduleNotFound)
	}
}

func openRepositoryLookupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExecRepositorySQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()

	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
