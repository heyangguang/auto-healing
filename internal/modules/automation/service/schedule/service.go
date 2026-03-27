package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Service 定时任务调度服务
type Service struct {
	repo     *automationrepo.ScheduleRepository
	execRepo *automationrepo.ExecutionRepository
}

// NewService 创建定时任务调度服务
func NewService() *Service {
	return &Service{
		repo:     automationrepo.NewScheduleRepository(),
		execRepo: automationrepo.NewExecutionRepository(),
	}
}

// Create 创建定时任务调度
func (s *Service) Create(ctx context.Context, schedule *model.ExecutionSchedule) (*model.ExecutionSchedule, error) {
	// 验证任务模板存在
	task, err := s.execRepo.GetTaskByID(ctx, schedule.TaskID)
	if err != nil {
		return nil, fmt.Errorf("任务模板不存在: %w", err)
	}
	if schedule.MaxFailures < 0 {
		return nil, fmt.Errorf("最大连续失败次数不能为负数")
	}

	// 根据调度类型验证和准备 next_run_at
	if err := s.prepareScheduleForSave(schedule); err != nil {
		return nil, err
	}

	// 设置初始状态
	schedule.Status = schedule.CalculateStatus()

	if err := s.repo.Create(ctx, schedule); err != nil {
		return nil, err
	}

	logger.Sched("SCHEDULE").Info("已创建: %s | 任务: %s | 类型: %s | 状态: %s", schedule.ID, task.Name, schedule.ScheduleType, schedule.Status)
	return schedule, nil
}

// validateAndSetNextRun 根据调度类型验证并设置 next_run_at
func (s *Service) validateAndSetNextRun(schedule *model.ExecutionSchedule) error {
	if err := validateScheduleDefinition(schedule); err != nil {
		return err
	}
	switch schedule.ScheduleType {
	case model.ScheduleTypeCron:
		nextRun, err := s.nextRunFromExpr(*schedule.ScheduleExpr)
		if err != nil {
			return err
		}
		schedule.NextRunAt = nextRun
	case model.ScheduleTypeOnce:
		if schedule.ScheduledAt.Before(time.Now()) {
			return fmt.Errorf("执行时间不能是过去时间")
		}
		schedule.NextRunAt = schedule.ScheduledAt
	}
	return nil
}

// Get 获取定时任务调度详情
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.ExecutionSchedule, error) {
	return s.repo.GetByID(ctx, id)
}

// List 列出定时任务调度（支持多条件筛选）
func (s *Service) List(ctx context.Context, opts *automationrepo.ScheduleListOptions) ([]model.ExecutionSchedule, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return s.repo.List(ctx, opts)
}

// Update 更新定时任务调度
func (s *Service) Update(ctx context.Context, id uuid.UUID, input *UpdateInput) (*model.ExecutionSchedule, error) {
	schedule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.applyUpdate(schedule, input); err != nil {
		return nil, err
	}

	if err := s.repo.UpdateFields(ctx, schedule.ID, buildScheduleDefinitionUpdates(schedule)); err != nil {
		return nil, err
	}

	logger.Sched("SCHEDULE").Info("已更新: %s | 名称: %s", schedule.ID, schedule.Name)
	return schedule, nil
}

// Delete 删除定时任务调度（保护性删除：必须先禁用）
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	schedule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否已禁用
	if schedule.Enabled {
		return fmt.Errorf("无法删除：调度任务正在启用中，请先禁用再删除")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	logger.Sched("SCHEDULE").Info("已删除: %s | 名称: %s", id, schedule.Name)
	return nil
}

// Enable 启用定时任务调度
func (s *Service) Enable(ctx context.Context, id uuid.UUID) error {
	schedule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	schedule.Enabled = true
	if err := s.prepareScheduleForSave(schedule); err != nil {
		schedule.Enabled = false
		return err
	}
	if schedule.IsOnce() && schedule.LastRunAt != nil {
		schedule.LastRunAt = nil
	}
	schedule.ConsecutiveFailures = 0
	schedule.PauseReason = ""
	schedule.Status = schedule.CalculateStatus()

	if err := s.repo.UpdateFields(ctx, schedule.ID, buildScheduleEnableUpdates(schedule)); err != nil {
		return err
	}

	logger.Sched("SCHEDULE").Info("已启用: %s | 名称: %s | 状态: %s", id, schedule.Name, schedule.Status)
	return nil
}

// Disable 禁用定时任务调度
func (s *Service) Disable(ctx context.Context, id uuid.UUID) error {
	schedule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	schedule.Enabled = false
	schedule.Status = model.ScheduleStatusDisabled
	schedule.NextRunAt = nil // 禁用后清除下次执行时间

	if err := s.repo.UpdateFields(ctx, schedule.ID, buildScheduleDisableUpdates(schedule)); err != nil {
		return err
	}

	logger.Sched("SCHEDULE").Info("已禁用: %s | 名称: %s", id, schedule.Name)
	return nil
}

// GetDueSchedules 获取到期需要执行的调度
func (s *Service) GetDueSchedules(ctx context.Context) ([]model.ExecutionSchedule, error) {
	return s.repo.GetDueSchedules(ctx)
}

// UpdateNextRunAt 更新下次执行时间（仅用于 Cron 模式）
func (s *Service) UpdateNextRunAt(ctx context.Context, id uuid.UUID, scheduleExpr string) error {
	nextRun, err := s.nextRunFromExpr(scheduleExpr)
	if err != nil {
		return err
	}
	return s.repo.UpdateNextRunAt(ctx, id, *nextRun)
}

// MarkCompleted 标记单次调度为已完成
func (s *Service) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	schedule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	schedule.Enabled = false
	schedule.Status = model.ScheduleStatusCompleted

	if err := s.repo.UpdateFields(ctx, schedule.ID, buildScheduleCompletionUpdates(schedule)); err != nil {
		return err
	}

	logger.Sched("SCHEDULE").Info("已完成: %s | 名称: %s", id, schedule.Name)
	return nil
}

// calculateNextRun 计算下次执行时间（使用本地时区 Asia/Shanghai）
func (s *Service) calculateNextRun(scheduleExpr string) *time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(scheduleExpr)
	if err != nil {
		return nil
	}

	now := time.Now().In(loc)
	next := schedule.Next(now)
	return &next
}

func (s *Service) nextRunFromExpr(scheduleExpr string) (*time.Time, error) {
	nextRun := s.calculateNextRun(scheduleExpr)
	if nextRun == nil {
		return nil, fmt.Errorf("无效的 Cron 表达式")
	}
	return nextRun, nil
}

// ==================== 统计 ====================

// GetStats 获取定时任务调度统计信息
func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}

// ListTimeline 获取调度时间线（轻量接口，用于可视化）
func (s *Service) ListTimeline(ctx context.Context, date time.Time, enabled *bool, scheduleType string) ([]automationrepo.ScheduleTimelineItem, error) {
	return s.repo.ListTimeline(ctx, date, enabled, scheduleType)
}
