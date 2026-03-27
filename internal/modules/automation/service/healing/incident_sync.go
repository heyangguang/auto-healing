package healing

import (
	"context"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/google/uuid"
)

func instanceIncidentSyncOptions(instance *model.FlowInstance, healingStatus string) *automationrepo.IncidentSyncOptions {
	if instance == nil || instance.IncidentID == nil {
		return nil
	}
	return &automationrepo.IncidentSyncOptions{
		IncidentID:    *instance.IncidentID,
		HealingStatus: healingStatus,
	}
}

func incidentFailureSyncOptions(ctx context.Context, repo *automationrepo.FlowInstanceRepository, instanceID uuid.UUID) *automationrepo.IncidentSyncOptions {
	instance, err := repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil
	}
	return instanceIncidentSyncOptions(instance, "failed")
}
