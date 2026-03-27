package healing

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

func TestSchedulerStopWaitsForTrackedFlowWorker(t *testing.T) {
	db := newHealingTestDB(t)
	createSchedulerFlowSchema(t, db)
	createSchedulerIncidentSchema(t, db)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	scheduler := NewScheduler()
	scheduler.instanceRepo = automationrepo.NewFlowInstanceRepository()
	scheduler.incidentRepo = incidentrepo.NewIncidentRepository()
	scheduler.interval = time.Hour
	tenantID := uuid.MustParse("46464646-4646-4646-4646-464646464646")
	instanceID := uuid.MustParse("47474747-4747-4747-4747-474747474747")

	started := make(chan struct{})
	stopped := make(chan struct{})

	scheduler.recoverOrphans = func(context.Context) {}
	scheduler.scanNow = func(ctx context.Context) {
		insertSchedulerFlowInstance(t, db, instanceID, uuid.Nil, tenantID, model.FlowInstanceStatusRunning)
		scheduler.scheduleAutoFlowExecution(&model.FlowInstance{
			ID:       instanceID,
			TenantID: &tenantID,
			Status:   model.FlowInstanceStatusRunning,
		}, uuid.New())
	}
	scheduler.runFlow = func(ctx context.Context, instance *model.FlowInstance) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil
	}

	scheduler.Start()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("flow worker did not start")
	}

	scheduler.Stop()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("flow worker did not stop before Stop returned")
	}

	flowCtx := platformrepo.WithTenantID(context.Background(), tenantID)
	instance, err := scheduler.instanceRepo.GetByID(flowCtx, instanceID)
	if err != nil {
		t.Fatalf("GetByID(): %v", err)
	}
	if instance.Status != model.FlowInstanceStatusFailed {
		t.Fatalf("status = %s, want %s", instance.Status, model.FlowInstanceStatusFailed)
	}
}
