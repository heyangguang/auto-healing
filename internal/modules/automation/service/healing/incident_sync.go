package healing

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

func instanceIncidentSyncOptions(instance *model.FlowInstance, healingStatus string) *repository.IncidentSyncOptions {
	if instance == nil || instance.IncidentID == nil {
		return nil
	}
	return &repository.IncidentSyncOptions{
		IncidentID:    *instance.IncidentID,
		HealingStatus: healingStatus,
	}
}

func incidentFailureSyncOptions(ctx context.Context, repo *repository.FlowInstanceRepository, instanceID uuid.UUID) *repository.IncidentSyncOptions {
	instance, err := repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil
	}
	return instanceIncidentSyncOptions(instance, "failed")
}
