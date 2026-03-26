package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func (e *FlowExecutor) persistNodeStates(ctx context.Context, instance *model.FlowInstance, action string) error {
	return e.persistNodeStatesByID(ctx, instance.ID, instance.NodeStates, action)
}

func (e *FlowExecutor) persistNodeStatesByID(ctx context.Context, instanceID uuid.UUID, nodeStates model.JSON, action string) error {
	if err := e.instanceRepo.UpdateNodeStates(ctx, instanceID, nodeStates); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func (e *FlowExecutor) persistInstance(ctx context.Context, instance *model.FlowInstance, action string) error {
	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func (e *FlowExecutor) persistIncident(ctx context.Context, incident *model.Incident, action string) error {
	if err := e.incidentRepo.UpdateHealingStatus(ctx, incident.ID, incident.HealingStatus); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
