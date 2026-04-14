package plugin

import (
	"context"
	"fmt"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

func (s *IncidentService) ListWritebackLogs(ctx context.Context, incidentID uuid.UUID) ([]platformmodel.IncidentWritebackLog, error) {
	if _, err := s.incidentRepo.GetByID(ctx, incidentID); err != nil {
		return nil, fmt.Errorf("获取工单失败: %w", err)
	}
	return s.writebackLogRepo.ListByIncidentID(ctx, incidentID)
}
