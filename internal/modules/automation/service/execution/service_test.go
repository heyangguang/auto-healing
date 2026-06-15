package execution

import (
	"context"
	"path/filepath"
	"strings"
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

	svc := &Service{repo: automationrepo.NewExecutionRepositoryWithDB(db)}
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

func TestCreateTaskRejectsTargetHostsMissingFromCMDB(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	_, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "bad target",
		PlaybookID:   playbookID,
		TargetHosts:  "127.0.0.1",
		ExecutorType: "docker",
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want missing CMDB host rejection")
	}
}

func TestCreateTaskAcceptsActiveCMDBTargetHost(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	task, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "good target",
		PlaybookID:   playbookID,
		TargetHosts:  "118.196.22.79",
		ExecutorType: "docker",
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.TargetHosts != "118.196.22.79" {
		t.Fatalf("TargetHosts = %q, want 118.196.22.79", task.TargetHosts)
	}
}

func TestCreateTaskDeduplicatesCMDBHostAliases(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	task, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "alias target",
		PlaybookID:   playbookID,
		TargetHosts:  "e2e-target-01,118.196.22.79,e2e-target-01",
		ExecutorType: "docker",
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.TargetHosts != "e2e-target-01" {
		t.Fatalf("TargetHosts = %q, want e2e-target-01", task.TargetHosts)
	}
}

func TestBuildSecretQueryResolvesCMDBHostIdentity(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	byHostname := svc.buildSecretQuery(ctx, "e2e-target-01", "password")
	if byHostname.Hostname != "e2e-target-01" || byHostname.IPAddress != "118.196.22.79" {
		t.Fatalf("hostname query = %#v, want hostname e2e-target-01 and ip 118.196.22.79", byHostname)
	}
	byIP := svc.buildSecretQuery(ctx, "118.196.22.79", "password")
	if byIP.Hostname != "e2e-target-01" || byIP.IPAddress != "118.196.22.79" {
		t.Fatalf("ip query = %#v, want hostname e2e-target-01 and ip 118.196.22.79", byIP)
	}
}

func TestCreateTaskRejectsUnsupportedExecutorType(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	_, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "bad executor",
		PlaybookID:   playbookID,
		TargetHosts:  "118.196.22.79",
		ExecutorType: "shell",
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want unsupported executor rejection")
	}
}

func TestCreateTaskRejectsInactiveSecretsSource(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	secretID := insertExecutionServiceSecretsSource(t, db, tenantID, "demo-secret", "inactive")
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	_, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:               uuid.New(),
		Name:             "bad secret",
		PlaybookID:       playbookID,
		TargetHosts:      "118.196.22.79",
		ExecutorType:     "docker",
		SecretsSourceIDs: model.StringArray{secretID.String()},
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want inactive secrets source rejection")
	}
}

func TestCreateTaskRejectsCredentialExtraVarsWithSecretsSource(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	secretID := insertExecutionServiceSecretsSource(t, db, tenantID, "demo-secret", "active")
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)

	_, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:               uuid.New(),
		Name:             "credential conflict",
		PlaybookID:       playbookID,
		TargetHosts:      "e2e-target-01",
		ExecutorType:     "docker",
		SecretsSourceIDs: model.StringArray{secretID.String()},
		ExtraVars: model.JSON{
			"ansible_password": "should-not-be-here",
		},
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want credential extra var rejection")
	}
	if !strings.Contains(err.Error(), "认证变量") {
		t.Fatalf("CreateTask() error = %v, want credential extra var rejection", err)
	}
}

func TestValidateRuntimeCredentialExtraVarsRejectsOverridesWithSecretsSource(t *testing.T) {
	err := validateRuntimeCredentialExtraVars(
		[]uuid.UUID{uuid.New()},
		model.JSON{"safe_var": "ok"},
		map[string]any{"ansible_ssh_pass": "should-not-be-here"},
	)
	if err == nil {
		t.Fatal("validateRuntimeCredentialExtraVars() error = nil, want runtime credential override rejection")
	}
	if !strings.Contains(err.Error(), "运行时 extra_vars") {
		t.Fatalf("validateRuntimeCredentialExtraVars() error = %v, want runtime context", err)
	}
}

func TestCreateTaskRejectsMissingNotificationTemplate(t *testing.T) {
	db := openExecutionServiceValidationDB(t)
	tenantID := uuid.New()
	playbookID := insertExecutionServicePlaybook(t, db, tenantID)
	channelID := insertExecutionServiceNotificationChannel(t, db, tenantID, "webhook", true)
	insertExecutionServiceCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")
	svc := NewServiceWithDB(db)
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	missingTemplateID := uuid.New()

	_, err := svc.CreateTask(ctx, &model.ExecutionTask{
		ID:           uuid.New(),
		Name:         "bad notification",
		PlaybookID:   playbookID,
		TargetHosts:  "118.196.22.79",
		ExecutorType: "docker",
		NotificationConfig: &model.TaskNotificationConfig{
			Enabled: true,
			OnSuccess: &model.NotificationTriggerConfig{
				Enabled:    true,
				TemplateID: &missingTemplateID,
				ChannelIDs: []uuid.UUID{channelID},
			},
		},
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want missing notification template rejection")
	}
}

func openExecutionServiceValidationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "execution-service-validation.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			url TEXT,
			default_branch TEXT,
			status TEXT
		)
	`)
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE playbooks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			repository_id TEXT,
			name TEXT,
			file_path TEXT,
			variables TEXT,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE execution_tasks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			playbook_id TEXT,
			target_hosts TEXT,
			extra_vars TEXT,
			executor_type TEXT,
			description TEXT,
			secrets_source_ids TEXT,
			notification_config TEXT,
			playbook_variables_snapshot TEXT,
			needs_review BOOLEAN,
			changed_variables TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			external_id TEXT,
			name TEXT,
			type TEXT,
			status TEXT,
			ip_address TEXT,
			hostname TEXT,
			raw_data TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE secrets_sources (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			type TEXT,
			auth_type TEXT,
			config TEXT,
			is_default BOOLEAN,
			priority INTEGER,
			status TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE notification_templates (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			description TEXT,
			event_type TEXT,
			supported_channels TEXT,
			subject_template TEXT,
			body_template TEXT,
			format TEXT,
			available_variables TEXT,
			is_active BOOLEAN,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	mustExecExecutionServiceValidationSQL(t, db, `
		CREATE TABLE notification_channels (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT,
			type TEXT,
			description TEXT,
			config TEXT,
			retry_config TEXT,
			recipients TEXT,
			is_active BOOLEAN,
			is_default BOOLEAN,
			rate_limit_per_minute INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	return db
}

func insertExecutionServicePlaybook(t *testing.T, db *gorm.DB, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	repoID := uuid.New()
	playbookID := uuid.New()
	mustExecExecutionServiceValidationSQL(t, db, `
		INSERT INTO git_repositories (id, tenant_id, name, url, default_branch, status)
		VALUES (?, ?, 'repo', 'http://git/repo.git', 'main', 'ready')
	`, repoID.String(), tenantID.String())
	mustExecExecutionServiceValidationSQL(t, db, `
		INSERT INTO playbooks (id, tenant_id, repository_id, name, file_path, variables, status)
		VALUES (?, ?, ?, 'playbook', 'site.yml', '[]', 'ready')
	`, playbookID.String(), tenantID.String(), repoID.String())
	return playbookID
}

func insertExecutionServiceCMDBItem(t *testing.T, db *gorm.DB, tenantID uuid.UUID, name, ipAddress, status string) {
	t.Helper()
	mustExecExecutionServiceValidationSQL(t, db, `
		INSERT INTO cmdb_items (id, tenant_id, external_id, name, type, status, ip_address, hostname, raw_data)
		VALUES (?, ?, ?, ?, 'server', ?, ?, ?, '{}')
	`, uuid.NewString(), tenantID.String(), name, name, status, ipAddress, name)
}

func insertExecutionServiceSecretsSource(t *testing.T, db *gorm.DB, tenantID uuid.UUID, name, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExecExecutionServiceValidationSQL(t, db, `
		INSERT INTO secrets_sources (id, tenant_id, name, type, auth_type, config, is_default, priority, status)
		VALUES (?, ?, ?, 'file', 'ssh_key', '{}', false, 0, ?)
	`, id.String(), tenantID.String(), name, status)
	return id
}

func insertExecutionServiceNotificationChannel(t *testing.T, db *gorm.DB, tenantID uuid.UUID, name string, active bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	mustExecExecutionServiceValidationSQL(t, db, `
		INSERT INTO notification_channels (id, tenant_id, name, type, config, recipients, is_active, is_default)
		VALUES (?, ?, ?, 'webhook', '{}', '[]', ?, false)
	`, id.String(), tenantID.String(), name, active)
	return id
}

func mustExecExecutionServiceValidationSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}
