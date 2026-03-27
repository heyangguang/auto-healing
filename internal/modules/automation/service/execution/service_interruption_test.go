package execution

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/secrets"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMarkPendingRunInterruptedFinalizesPendingRun(t *testing.T) {
	db := newExecutionRunTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{repo: automationrepo.NewExecutionRepository()}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "pending")

	svc.markPendingRunInterrupted(runID, ctx)

	assertExecutionRunStatus(t, db, runID, "failed")
	assertExecutionRunLogCount(t, db, runID, 1)
}

func TestMarkPendingRunInterruptedPreservesTenantAfterCancellation(t *testing.T) {
	db := newExecutionRunTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{repo: automationrepo.NewExecutionRepository()}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "pending")

	svc.markPendingRunInterrupted(runID, ctx)

	assertExecutionRunStatus(t, db, runID, "failed")
}

func TestInterruptRunningRunFinalizesRunningRun(t *testing.T) {
	db := newExecutionRunTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{repo: automationrepo.NewExecutionRepository()}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "running")

	svc.interruptRunningRun(runID, ctx)

	assertExecutionRunStatus(t, db, runID, "failed")
	assertExecutionRunLogCount(t, db, runID, 1)
}

func TestPrepareBasicInventoryPersistsErrorLogAfterContextCancellation(t *testing.T) {
	db := newExecutionRunTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{repo: automationrepo.NewExecutionRepository()}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "running")

	if _, err := svc.prepareBasicInventory(ctx, runID, filepath.Join(t.TempDir(), "missing"), "127.0.0.1"); err == nil {
		t.Fatal("prepareBasicInventory() should fail for missing workdir")
	}

	assertExecutionRunStatus(t, db, runID, "failed")
	assertExecutionRunLogCount(t, db, runID, 1)
}

func TestPrepareAuthenticatedInventoryFinalizesRunWhenCredentialBuildFails(t *testing.T) {
	db := newExecutionRunTestDB(t)
	createExecutionCMDBSchema(t, db)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{
		repo:     automationrepo.NewExecutionRepository(),
		cmdbRepo: repository.NewCMDBItemRepository(),
	}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "running")

	task := &model.ExecutionTask{ExecutorType: "local"}
	providers := []sourceProvider{{
		source:   &model.SecretsSource{Name: "provider-a", AuthType: "ssh_key"},
		provider: fakeSecretsProvider{secret: &model.Secret{AuthType: "ssh_key", Username: "root", PrivateKey: "bad-key"}},
	}}

	if _, err := svc.prepareAuthenticatedInventory(ctx, runID, task, filepath.Join(t.TempDir(), "missing"), "10.0.0.1", providers); err == nil {
		t.Fatal("prepareAuthenticatedInventory() should fail when key file write fails")
	}

	assertExecutionRunStatus(t, db, runID, "failed")
	assertExecutionRunLogCount(t, db, runID, 1)
}

func TestResolveHostCredentialReturnsBackendErrors(t *testing.T) {
	db := newExecutionRunTestDB(t)
	createExecutionCMDBSchema(t, db)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{
		repo:     automationrepo.NewExecutionRepository(),
		cmdbRepo: repository.NewCMDBItemRepository(),
	}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "running")

	_, err := svc.resolveHostCredential(ctx, runID, &model.ExecutionTask{}, t.TempDir(), "10.0.0.2", []sourceProvider{{
		source:   &model.SecretsSource{Name: "provider-a", AuthType: "ssh_key"},
		provider: fakeSecretsProvider{err: errors.New("backend down")},
	}})
	if err == nil {
		t.Fatal("resolveHostCredential() should return non-not-found provider errors")
	}
}

func TestResolveHostCredentialContinuesOnSecretNotFound(t *testing.T) {
	db := newExecutionRunTestDB(t)
	createExecutionCMDBSchema(t, db)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	svc := &Service{
		repo:     automationrepo.NewExecutionRepository(),
		cmdbRepo: repository.NewCMDBItemRepository(),
	}
	ctx := repository.WithTenantID(context.Background(), uuid.New())
	runID := uuid.New()
	insertExecutionRun(t, db, runID, repository.TenantIDFromContext(ctx), "running")

	credential, err := svc.resolveHostCredential(ctx, runID, &model.ExecutionTask{}, t.TempDir(), "10.0.0.3", []sourceProvider{{
		source:   &model.SecretsSource{Name: "provider-a", AuthType: "ssh_key"},
		provider: fakeSecretsProvider{err: secrets.ErrSecretNotFound},
	}})
	if err != nil {
		t.Fatalf("resolveHostCredential() error = %v", err)
	}
	if credential.Host != "10.0.0.3" {
		t.Fatalf("credential.Host = %s, want 10.0.0.3", credential.Host)
	}
}

type fakeSecretsProvider struct {
	secret *model.Secret
	err    error
}

func (f fakeSecretsProvider) GetSecret(context.Context, model.SecretQuery) (*model.Secret, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.secret, nil
}

func (f fakeSecretsProvider) TestConnection(context.Context) error {
	return nil
}

func (f fakeSecretsProvider) Name() string {
	return "fake"
}

func createExecutionCMDBSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			ip_address TEXT,
			hostname TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create cmdb_items table: %v", err)
	}
}

func newExecutionRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "execution-runs.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE execution_runs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			status TEXT NOT NULL,
			task_id TEXT,
			exit_code INTEGER,
			stdout TEXT,
			stderr TEXT,
			stats TEXT,
			started_at DATETIME,
			completed_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create execution_runs table: %v", err)
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
	return db
}

func insertExecutionRun(t *testing.T, db *gorm.DB, runID, tenantID uuid.UUID, status string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO execution_runs (id, tenant_id, status) VALUES (?, ?, ?)`, runID.String(), tenantID.String(), status).Error; err != nil {
		t.Fatalf("insert execution run: %v", err)
	}
}

func assertExecutionRunStatus(t *testing.T, db *gorm.DB, runID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := db.Table("execution_runs").Select("status").Where("id = ?", runID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read execution run status: %v", err)
	}
	if status != want {
		t.Fatalf("status = %s, want %s", status, want)
	}
}

func assertExecutionRunLogCount(t *testing.T, db *gorm.DB, runID uuid.UUID, want int64) {
	t.Helper()
	var count int64
	if err := db.Table("execution_logs").Where("run_id = ?", runID.String()).Count(&count).Error; err != nil {
		t.Fatalf("count execution logs: %v", err)
	}
	if count != want {
		t.Fatalf("execution log count = %d, want %d", count, want)
	}
}
