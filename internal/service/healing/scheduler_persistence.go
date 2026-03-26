package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func (s *Scheduler) markIncidentScanned(ctx context.Context, incidentID uuid.UUID, matchedRuleID, flowInstanceID *uuid.UUID) error {
	if err := s.incidentRepo.MarkScanned(ctx, incidentID, matchedRuleID, flowInstanceID); err != nil {
		return fmt.Errorf("标记工单已扫描失败: %w", err)
	}
	return nil
}

func (s *Scheduler) persistIncident(ctx context.Context, incident *model.Incident, action string) error {
	if err := s.incidentRepo.UpdateHealingStatus(ctx, incident.ID, incident.HealingStatus); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
