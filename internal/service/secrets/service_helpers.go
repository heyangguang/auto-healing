package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

func (s *Service) countSourceReferences(ctx context.Context, id uuid.UUID) (int64, error) {
	taskCount, err := s.repo.CountTasksUsingSource(ctx, id.String())
	if err != nil {
		return 0, fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	scheduleCount, err := s.repo.CountSchedulesUsingSource(ctx, id.String())
	if err != nil {
		return 0, fmt.Errorf("检查关联调度任务失败: %w", err)
	}
	return taskCount + scheduleCount, nil
}

func jsonEqual(a, b model.JSON) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}
