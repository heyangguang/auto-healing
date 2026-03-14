package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	executionService "github.com/company/auto-healing/internal/service/execution"
	scheduleService "github.com/company/auto-healing/internal/service/schedule"
	"github.com/google/uuid"
)

// ExecutionScheduler 执行任务调度器
// 使用独立的 ExecutionSchedule 表管理定时任务
type ExecutionScheduler struct {
	execSvc      *executionService.Service
	scheduleSvc  *scheduleService.Service
	scheduleRepo *repository.ScheduleRepository
	interval     time.Duration // 检查间隔
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
}

// NewExecutionScheduler 创建执行任务调度器
func NewExecutionScheduler() *ExecutionScheduler {
	return &ExecutionScheduler{
		execSvc:      executionService.NewService(),
		scheduleSvc:  scheduleService.NewService(),
		scheduleRepo: repository.NewScheduleRepository(),
		interval:     30 * time.Second, // 每30秒检查一次
		stopCh:       make(chan struct{}),
	}
}

// Start 启动调度器
func (s *ExecutionScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
	logger.Sched("TASK").Info("执行任务调度器已启动 (检查间隔: %v)", s.interval)
}

// Stop 停止调度器
func (s *ExecutionScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	logger.Sched("TASK").Info("执行任务调度器已停止")
}

// run 调度器主循环
func (s *ExecutionScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 启动时立即检查一次
	s.checkAndExecute()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndExecute()
		}
	}
}

// checkAndExecute 检查并执行定时任务
func (s *ExecutionScheduler) checkAndExecute() {
	ctx := context.Background()

	// 从 execution_schedules 表查询到期的调度
	schedules, err := s.scheduleRepo.GetDueSchedules(ctx)
	if err != nil {
		logger.Sched("TASK").Error("查询待执行调度失败: %v", err)
		return
	}

	if len(schedules) == 0 {
		return
	}

	logger.Sched("TASK").Info("发现 %d 个定时任务需要执行", len(schedules))

	// 并发执行任务
	for _, schedule := range schedules {
		go func(sched model.ExecutionSchedule) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Sched("TASK").Error("[%s] 定时任务 panic: %v", sched.ID.String()[:8], rec)
				}
			}()
			logger.Sched("TASK").Info("[%s] 开始执行定时任务: %s", sched.ID.String()[:8], sched.Name)

			// 注入该调度所属租户的上下文，确保 ExecuteTask 及后续操作在正确租户范围内运行
			tenantCtx := ctx
			if sched.TenantID != nil {
				tenantCtx = repository.WithTenantID(ctx, *sched.TenantID)
			}

			// 构建执行选项，传递调度中的覆盖参数
			opts := &executionService.ExecuteOptions{
				TriggeredBy: func() string {
					if sched.IsCron() {
						return "scheduler:cron"
					}
					return "scheduler:once"
				}(),
				TargetHosts:      sched.TargetHostsOverride,
				ExtraVars:        sched.ExtraVarsOverride,
				SkipNotification: sched.SkipNotification,
			}

			// 转换 SecretsSourceIDs
			if len(sched.SecretsSourceIDs) > 0 {
				for _, idStr := range sched.SecretsSourceIDs {
					if id, err := uuid.Parse(idStr); err == nil {
						opts.SecretsSourceIDs = append(opts.SecretsSourceIDs, id)
					}
				}
			}

			shortID := sched.ID.String()[:8]
			_, err := s.execSvc.ExecuteTask(tenantCtx, sched.TaskID, opts)

			if err != nil {
				// 连续失败计数 +1
				newCount := sched.ConsecutiveFailures + 1
				updates := map[string]interface{}{
					"consecutive_failures": newCount,
				}

				// 检查是否需要自动暂停（max_failures > 0 才启用，仅 Cron 任务）
				if sched.MaxFailures > 0 && newCount >= sched.MaxFailures && sched.IsCron() {
					updates["enabled"] = false
					updates["status"] = model.ScheduleStatusAutoPaused
					updates["next_run_at"] = nil
					updates["pause_reason"] = fmt.Sprintf("连续失败 %d 次后自动暂停 (最后错误: %s)", newCount, truncateStr(err.Error(), 200))
					logger.Sched("TASK").Warn("[%s] ⚠ 连续失败 %d/%d 次，已自动暂停: %s",
						shortID, newCount, sched.MaxFailures, sched.Name)
				} else {
					if sched.MaxFailures > 0 {
						logger.Sched("TASK").Error("[%s] 执行失败 (%d/%d): %s - %v",
							shortID, newCount, sched.MaxFailures, sched.Name, err)
					} else {
						logger.Sched("TASK").Error("[%s] 执行失败 (第%d次): %s - %v",
							shortID, newCount, sched.Name, err)
					}
				}

				database.DB.WithContext(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", sched.ID).Updates(updates)
			} else {
				// 成功 → 重置失败计数
				updates := map[string]interface{}{
					"consecutive_failures": 0,
					"pause_reason":         "",
				}
				database.DB.WithContext(ctx).Model(&model.ExecutionSchedule{}).Where("id = ?", sched.ID).Updates(updates)

				if sched.ConsecutiveFailures > 0 {
					logger.Sched("TASK").Info("[%s] 执行成功: %s | 失败计数已重置 (之前: %d)",
						shortID, sched.Name, sched.ConsecutiveFailures)
				} else {
					logger.Sched("TASK").Info("[%s] 执行完成: %s", shortID, sched.Name)
				}
			}

			// 更新上次执行时间
			s.scheduleRepo.UpdateLastRunAt(tenantCtx, sched.ID)

			// 更新下次执行时间（仅 Cron 模式且未被自动暂停）或标记完成（Once 模式）
			if sched.IsCron() {
				// 如果已经被自动暂停，不再计算下次时间
				if !(sched.MaxFailures > 0 && err != nil && sched.ConsecutiveFailures+1 >= sched.MaxFailures) {
					s.scheduleSvc.UpdateNextRunAt(tenantCtx, sched.ID, *sched.ScheduleExpr)
				}
			} else {
				// Once 模式执行后标记为已完成
				s.scheduleSvc.MarkCompleted(tenantCtx, sched.ID)
			}
		}(schedule)
	}
}
