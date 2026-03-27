package execution

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWatchRunCancellationCancelsWhenRunStatusTurnsCancelled(t *testing.T) {
	t.Helper()

	statuses := []string{"running", "running", "cancelled"}
	var idx atomic.Int32
	cancelled := make(chan struct{}, 1)

	stop := watchRunCancellation(context.Background(), time.Millisecond, func(context.Context) (string, error) {
		i := int(idx.Add(1) - 1)
		if i >= len(statuses) {
			return statuses[len(statuses)-1], nil
		}
		return statuses[i], nil
	}, func() {
		select {
		case cancelled <- struct{}{}:
		default:
		}
	})
	defer stop()

	select {
	case <-cancelled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected cancellation watcher to invoke cancel()")
	}
}

func TestWatchRunCancellationStopsOnTerminalSuccess(t *testing.T) {
	t.Helper()

	var called atomic.Bool
	stop := watchRunCancellation(context.Background(), time.Millisecond, func(context.Context) (string, error) {
		return "success", nil
	}, func() {
		called.Store(true)
	})
	defer stop()

	time.Sleep(20 * time.Millisecond)
	if called.Load() {
		t.Fatal("cancel should not be invoked for successful runs")
	}
}

func TestAppendLogErrPersistsSequentialLogs(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "execution-logs.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE execution_logs (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			run_id TEXT NOT NULL,
			workflow_instance_id TEXT,
			node_execution_id TEXT,
			log_level TEXT NOT NULL,
			stage TEXT NOT NULL,
			message TEXT NOT NULL,
			host TEXT,
			task_name TEXT,
			play_name TEXT,
			details TEXT,
			sequence INTEGER NOT NULL,
			created_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create execution_logs table: %v", err)
	}

	origDB := database.DB
	database.DB = db
	defer func() { database.DB = origDB }()

	svc := &Service{repo: automationrepo.NewExecutionRepository()}
	runID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), uuid.New())

	if err := svc.appendLogErr(ctx, runID, "info", "prepare", "first", nil); err != nil {
		t.Fatalf("appendLogErr(first) error = %v", err)
	}
	if err := svc.appendLogErr(ctx, runID, "info", "execute", "second", nil); err != nil {
		t.Fatalf("appendLogErr(second) error = %v", err)
	}

	type executionLogRow struct {
		Sequence int
	}
	var logs []executionLogRow
	if err := db.Table("execution_logs").Order("sequence asc").Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("log count = %d, want 2", len(logs))
	}
	if logs[0].Sequence != 1 || logs[1].Sequence != 2 {
		t.Fatalf("log sequences = %d,%d, want 1,2", logs[0].Sequence, logs[1].Sequence)
	}
}

func TestResolveSecretsSourceIDsRejectsInvalidTaskTemplateID(t *testing.T) {
	task := &model.ExecutionTask{
		SecretsSourceIDs: model.StringArray{"not-a-uuid"},
	}

	ids, err := resolveSecretsSourceIDs(task, &ExecuteOptions{})
	if err == nil {
		t.Fatal("resolveSecretsSourceIDs() should reject invalid UUID")
	}
	if ids != nil {
		t.Fatalf("ids = %v, want nil", ids)
	}
}
