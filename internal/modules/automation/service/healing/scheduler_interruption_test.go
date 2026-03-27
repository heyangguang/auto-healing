package healing

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestScheduleAutoFlowExecutionMarksQueuedInstanceFailedOnStop(t *testing.T) {
	db := newHealingTestDB(t)
	createSchedulerFlowSchema(t, db)
	createSchedulerIncidentSchema(t, db)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	scheduler := NewScheduler()
	scheduler.instanceRepo = automationrepo.NewFlowInstanceRepository()
	scheduler.incidentRepo = incidentrepo.NewIncidentRepository()
	tenantID := uuid.MustParse("40404040-4040-4040-4040-404040404040")
	instanceID := uuid.MustParse("41414141-4141-4141-4141-414141414141")
	incidentID := uuid.MustParse("42424242-4242-4242-4242-424242424242")
	insertSchedulerFlowInstance(t, db, instanceID, incidentID, tenantID, model.FlowInstanceStatusPending)
	insertSchedulerIncident(t, db, incidentID, tenantID, "processing")

	scheduler.sem <- struct{}{}
	scheduler.scheduleAutoFlowExecution(&model.FlowInstance{
		ID:         instanceID,
		TenantID:   &tenantID,
		IncidentID: &incidentID,
		Status:     model.FlowInstanceStatusPending,
	}, incidentID)

	scheduler.lifecycle.Stop()

	assertSchedulerFlowStatus(t, db, instanceID, model.FlowInstanceStatusFailed)
	assertSchedulerIncidentStatus(t, db, incidentID, "failed")
}

func TestExecuteTrackedFlowMarksRunningInstanceFailedOnStop(t *testing.T) {
	db := newHealingTestDB(t)
	createSchedulerFlowSchema(t, db)
	createSchedulerIncidentSchema(t, db)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	scheduler := NewScheduler()
	scheduler.instanceRepo = automationrepo.NewFlowInstanceRepository()
	scheduler.incidentRepo = incidentrepo.NewIncidentRepository()
	tenantID := uuid.MustParse("43434343-4343-4343-4343-434343434343")
	instanceID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	incidentID := uuid.MustParse("45454545-4545-4545-4545-454545454545")
	rootCtx, cancel := context.WithCancel(context.Background())
	execCtx := platformrepo.WithTenantID(rootCtx, tenantID)

	insertSchedulerFlowInstance(t, db, instanceID, incidentID, tenantID, model.FlowInstanceStatusRunning)
	insertSchedulerIncident(t, db, incidentID, tenantID, "processing")

	done := make(chan struct{})
	scheduler.runFlow = func(ctx context.Context, instance *model.FlowInstance) error {
		<-ctx.Done()
		close(done)
		return nil
	}
	scheduler.sem <- struct{}{}
	cancel()
	scheduler.executeTrackedFlow(execCtx, &model.FlowInstance{
		ID:         instanceID,
		TenantID:   &tenantID,
		IncidentID: &incidentID,
		Status:     model.FlowInstanceStatusRunning,
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runFlow did not observe cancellation")
	}

	assertSchedulerFlowStatus(t, db, instanceID, model.FlowInstanceStatusFailed)
	assertSchedulerIncidentStatus(t, db, incidentID, "failed")
}

func TestScheduleManualFlowExecutionDoesNotBlockWhenCapacityIsFull(t *testing.T) {
	scheduler := NewScheduler()
	scheduler.sem <- struct{}{}
	executed := make(chan struct{})
	workerDone := make(chan struct{})

	returned := make(chan struct{})
	scheduler.runFlow = func(ctx context.Context, instance *model.FlowInstance) error {
		defer close(workerDone)
		close(executed)
		return nil
	}

	go func() {
		scheduler.scheduleManualFlowExecution(&model.FlowInstance{ID: uuid.New()})
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("scheduleManualFlowExecution should not block when capacity is full")
	}

	<-scheduler.sem
	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("queued manual flow did not execute after capacity was released")
	}
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("queued manual flow did not exit before scheduler stop")
	}
	scheduler.lifecycle.Stop()
}

func createSchedulerFlowSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT,
			rule_id TEXT,
			incident_id TEXT,
			status TEXT NOT NULL,
			current_node_id TEXT,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			context TEXT,
			node_states TEXT,
			flow_name TEXT,
			flow_nodes TEXT,
			flow_edges TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
}

func createSchedulerIncidentSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			scanned BOOLEAN,
			matched_rule_id TEXT,
			healing_flow_instance_id TEXT,
			updated_at DATETIME
		);
	`)
}

func insertSchedulerFlowInstance(t *testing.T, db *gorm.DB, instanceID, incidentID, tenantID uuid.UUID, status string) {
	t.Helper()
	mustExecHealing(t, db, `INSERT INTO flow_instances (id, tenant_id, incident_id, status, context, node_states, flow_name, flow_nodes, flow_edges) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, instanceID.String(), tenantID.String(), incidentID.String(), status, "{}", "{}", "flow", "[]", "[]")
}

func insertSchedulerIncident(t *testing.T, db *gorm.DB, incidentID, tenantID uuid.UUID, healingStatus string) {
	t.Helper()
	mustExecHealing(t, db, `INSERT INTO incidents (id, tenant_id, healing_status, scanned) VALUES (?, ?, ?, ?)`, incidentID.String(), tenantID.String(), healingStatus, true)
}

func assertSchedulerFlowStatus(t *testing.T, db *gorm.DB, instanceID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := db.Table("flow_instances").Select("status").Where("id = ?", instanceID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read flow instance status: %v", err)
	}
	if status != want {
		t.Fatalf("flow status = %s, want %s", status, want)
	}
}

func assertSchedulerIncidentStatus(t *testing.T, db *gorm.DB, incidentID uuid.UUID, want string) {
	t.Helper()
	var status string
	if err := db.Table("incidents").Select("healing_status").Where("id = ?", incidentID.String()).Scan(&status).Error; err != nil {
		t.Fatalf("read incident healing_status: %v", err)
	}
	if status != want {
		t.Fatalf("incident healing_status = %s, want %s", status, want)
	}
}
