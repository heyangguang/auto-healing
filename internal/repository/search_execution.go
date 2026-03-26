package repository

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *SearchRepository) searchPlaybooks(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.Playbook{}).Where("name ILIKE ? OR description ILIKE ?", like, like).Count(&total)
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.Playbook
	err := db.Model(&model.Playbook{}).
		Select("id, name, description, status").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/execution/playbooks",
			Extra:       map[string]any{"status": item.Status},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchTemplates(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.ExecutionTask{}).Where("name ILIKE ? OR description ILIKE ?", like, like).Count(&total)
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.ExecutionTask
	err := db.Model(&model.ExecutionTask{}).
		Select("id, name, description").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/execution/templates",
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchSchedules(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.ExecutionSchedule{}).Where("name ILIKE ? OR description ILIKE ?", like, like).Count(&total)
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.ExecutionSchedule
	err := db.Model(&model.ExecutionSchedule{}).
		Select("id, name, enabled, schedule_expr, description").
		Where("name ILIKE ? OR description ILIKE ?", like, like).
		Order("name").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		extra := map[string]any{"is_enabled": item.Enabled}
		if item.ScheduleExpr != nil {
			extra["cron_expression"] = *item.ScheduleExpr
		}
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       item.Name,
			Description: item.Description,
			Path:        "/execution/schedules",
			Extra:       extra,
		})
	}
	return results, total, nil
}

func (r *SearchRepository) searchExecutionRuns(ctx context.Context, db *gorm.DB, like string, limit int) ([]SearchResultItem, int64, error) {
	var total int64
	db.Model(&model.ExecutionRun{}).
		Where("triggered_by ILIKE ? OR status ILIKE ? OR id::text ILIKE ?", like, like, like).
		Count(&total)
	if total == 0 {
		return nil, 0, nil
	}

	var items []model.ExecutionRun
	err := db.Model(&model.ExecutionRun{}).
		Select("id, task_id, status, triggered_by, created_at").
		Where("triggered_by ILIKE ? OR status ILIKE ? OR id::text ILIKE ?", like, like, like).
		Order("created_at DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	taskNameMap, err := r.loadExecutionTaskNames(items)
	if err != nil {
		return nil, 0, err
	}

	results := make([]SearchResultItem, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResultItem{
			ID:          item.ID,
			Title:       executionRunTitle(item, taskNameMap[item.TaskID]),
			Description: item.TriggeredBy,
			Path:        fmt.Sprintf("/execution/runs/%s", item.ID.String()),
			Extra: map[string]any{
				"status":     item.Status,
				"created_at": item.CreatedAt,
			},
		})
	}
	return results, total, nil
}

func (r *SearchRepository) loadExecutionTaskNames(items []model.ExecutionRun) (map[uuid.UUID]string, error) {
	taskIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item.TaskID != uuid.Nil {
			taskIDs = append(taskIDs, item.TaskID)
		}
	}
	if len(taskIDs) == 0 {
		return map[uuid.UUID]string{}, nil
	}

	type taskInfo struct {
		ID   uuid.UUID `gorm:"column:id"`
		Name string    `gorm:"column:name"`
	}
	var tasks []taskInfo
	if err := r.db.Model(&model.ExecutionTask{}).
		Select("id, name").
		Where("id IN ?", taskIDs).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	taskNameMap := make(map[uuid.UUID]string, len(tasks))
	for _, task := range tasks {
		taskNameMap[task.ID] = task.Name
	}
	return taskNameMap, nil
}

func executionRunTitle(item model.ExecutionRun, taskName string) string {
	switch {
	case taskName != "":
		return taskName
	case item.TriggeredBy != "":
		return item.TriggeredBy
	default:
		return item.ID.String()[:8]
	}
}
