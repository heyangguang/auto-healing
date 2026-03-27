package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	executionService "github.com/company/auto-healing/internal/modules/automation/service/execution"
	scheduleService "github.com/company/auto-healing/internal/modules/automation/service/schedule"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformsched "github.com/company/auto-healing/internal/platform/schedulerx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const executionClaimLease = 40 * time.Minute

// ExecutionScheduler 执行任务调度器
// 使用独立的 ExecutionSchedule 表管理定时任务
type ExecutionScheduler struct {
	execSvc               *executionService.Service
	scheduleSvc           *scheduleService.Service
	scheduleRepo          *automationrepo.ScheduleRepository
	db                    *gorm.DB
	interval              time.Duration
	lifecycle             *platformsched.Lifecycle
	inFlight              *platformsched.InFlightSet
	running               bool
	mu                    sync.Mutex
	sem                   chan struct{}
	loadDueSchedules      func(context.Context) ([]model.ExecutionSchedule, error)
	runScheduledExecution func(context.Context, model.ExecutionSchedule)
	executeTask           func(context.Context, uuid.UUID, *executionService.ExecuteOptions) (*model.ExecutionRun, error)
	getRun                func(context.Context, uuid.UUID) (*model.ExecutionRun, error)
	updateScheduleState   func(context.Context, uuid.UUID, map[string]interface{}) error
	updateScheduleLastRun func(context.Context, uuid.UUID) error
	updateScheduleNextRun func(context.Context, uuid.UUID, string) error
	markScheduleCompleted func(context.Context, uuid.UUID) error
	updateLastRunAt       func(context.Context, uuid.UUID) error
	updateNextRunAt       func(context.Context, uuid.UUID, string) error
	markCompleted         func(context.Context, uuid.UUID) error
	claimDueSchedule      func(context.Context, model.ExecutionSchedule) (bool, error)
	followRunAfterStop    func(context.Context, model.ExecutionSchedule, uuid.UUID)
}

var errExecutionSchedulerStopped = errors.New("execution scheduler stopped")

type ExecutionSchedulerDeps struct {
	ExecutionService *executionService.Service
	ScheduleService  *scheduleService.Service
	ScheduleRepo     *automationrepo.ScheduleRepository
	DB               *gorm.DB
	Interval         time.Duration
	Lifecycle        *platformsched.Lifecycle
	InFlight         *platformsched.InFlightSet
	Sem              chan struct{}
}

func DefaultExecutionSchedulerDepsWithDB(db *gorm.DB) ExecutionSchedulerDeps {
	return ExecutionSchedulerDeps{
		ExecutionService: executionService.NewServiceWithDB(db),
		ScheduleService:  scheduleService.NewServiceWithDB(db),
		ScheduleRepo:     automationrepo.NewScheduleRepositoryWithDB(db),
		DB:               db,
		Interval:         30 * time.Second,
		InFlight:         platformsched.NewInFlightSet(),
		Sem:              make(chan struct{}, 8),
	}
}

func NewExecutionSchedulerWithDeps(deps ExecutionSchedulerDeps) *ExecutionScheduler {
	switch {
	case deps.ExecutionService == nil:
		panic("automation execution scheduler requires execution service")
	case deps.ScheduleService == nil:
		panic("automation execution scheduler requires schedule service")
	case deps.ScheduleRepo == nil:
		panic("automation execution scheduler requires schedule repo")
	}
	if deps.Interval == 0 {
		deps.Interval = 30 * time.Second
	}
	if deps.InFlight == nil {
		deps.InFlight = platformsched.NewInFlightSet()
	}
	if deps.Sem == nil {
		deps.Sem = make(chan struct{}, 8)
	}
	s := &ExecutionScheduler{
		execSvc:      deps.ExecutionService,
		scheduleSvc:  deps.ScheduleService,
		scheduleRepo: deps.ScheduleRepo,
		db:           deps.DB,
		interval:     deps.Interval,
		lifecycle:    deps.Lifecycle,
		inFlight:     deps.InFlight,
		sem:          deps.Sem,
	}
	s.loadDueSchedules = s.scheduleRepo.GetDueSchedules
	s.runScheduledExecution = s.executeSchedule
	s.executeTask = s.execSvc.ExecuteTask
	s.getRun = s.execSvc.GetRun
	s.updateScheduleState = s.applyScheduleStateUpdate
	s.updateLastRunAt = s.scheduleRepo.UpdateLastRunAt
	s.updateNextRunAt = s.scheduleSvc.UpdateNextRunAt
	s.markCompleted = s.scheduleSvc.MarkCompleted
	s.claimDueSchedule = s.claimSchedule
	s.followRunAfterStop = s.detachScheduleCompletion
	return s
}

// Start 启动调度器
func (s *ExecutionScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	if s.lifecycle == nil || s.lifecycle.Context().Err() != nil {
		s.lifecycle = platformsched.NewLifecycle()
	}
	lifecycle := s.lifecycle
	s.running = true
	s.mu.Unlock()

	lifecycle.Go(s.run)
	logger.Sched("TASK").Info("执行任务调度器已启动 (检查间隔: %v, 最大并发: %d)", s.interval, cap(s.sem))
}

// Stop 停止调度器
func (s *ExecutionScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	lifecycle := s.lifecycle
	s.mu.Unlock()

	if lifecycle != nil {
		lifecycle.Stop()
	}
	logger.Sched("TASK").Info("执行任务调度器已停止")
}

// run 调度器主循环
func (s *ExecutionScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.checkAndExecute(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndExecute(ctx)
		}
	}
}

// checkAndExecute 检查并执行定时任务
func (s *ExecutionScheduler) checkAndExecute(ctx context.Context) {
	schedules, err := s.loadDueSchedules(ctx)
	if err != nil {
		logger.Sched("TASK").Error("查询待执行调度失败: %v", err)
		return
	}
	if len(schedules) == 0 {
		return
	}

	logger.Sched("TASK").Info("发现 %d 个定时任务需要执行", len(schedules))

	lifecycle := s.lifecycleSnapshot()
	for _, schedule := range schedules {
		claimed, err := s.claimDueSchedule(ctx, schedule)
		if err != nil {
			logger.Sched("TASK").Error("认领定时任务失败: %s (%s) - %v", schedule.Name, schedule.ID.String()[:8], err)
			continue
		}
		if !claimed {
			continue
		}

		select {
		case <-ctx.Done():
			s.rollbackExecutionClaim(ctx, schedule)
			return
		case s.sem <- struct{}{}:
			if !s.dispatchScheduledExecution(lifecycle, schedule) {
				s.rollbackExecutionClaim(ctx, schedule)
			}
		default:
			s.rollbackExecutionClaim(ctx, schedule)
			logger.Sched("TASK").Warn("执行调度器并发已满，延后调度: %s (%s)", schedule.Name, schedule.ID.String()[:8])
		}
	}
}

func (s *ExecutionScheduler) dispatchScheduledExecution(lifecycle *platformsched.Lifecycle, schedule model.ExecutionSchedule) bool {
	if lifecycle == nil {
		s.releaseSemaphoreToken()
		return false
	}
	if !s.inFlight.Start(schedule.ID) {
		s.releaseSemaphoreToken()
		return false
	}

	sched := schedule
	started := lifecycle.Go(func(rootCtx context.Context) {
		defer s.inFlight.Finish(sched.ID)
		defer s.releaseSemaphoreToken()
		s.runScheduledExecution(rootCtx, sched)
	})
	if !started {
		s.inFlight.Finish(sched.ID)
		s.releaseSemaphoreToken()
	}
	return started
}
