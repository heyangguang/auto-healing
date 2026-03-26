package healing

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
)

// Scheduler 全局自愈调度器
type Scheduler struct {
	ruleRepo     *repository.HealingRuleRepository
	flowRepo     *repository.HealingFlowRepository
	instanceRepo *repository.FlowInstanceRepository
	incidentRepo *repository.IncidentRepository
	approvalRepo *repository.ApprovalTaskRepository

	matcher  *RuleMatcher
	executor *FlowExecutor

	interval       time.Duration
	running        bool
	lifecycle      *asyncLifecycle
	mu             sync.Mutex
	sem            chan struct{}
	scanNow        func(context.Context)
	recoverOrphans func(context.Context)
	runFlow        func(context.Context, *model.FlowInstance) error
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	s := &Scheduler{
		ruleRepo:     repository.NewHealingRuleRepository(),
		flowRepo:     repository.NewHealingFlowRepository(),
		instanceRepo: repository.NewFlowInstanceRepository(),
		incidentRepo: repository.NewIncidentRepository(),
		approvalRepo: repository.NewApprovalTaskRepository(),
		matcher:      NewRuleMatcher(),
		executor:     NewFlowExecutor(),
		interval:     10 * time.Second,
		lifecycle:    newAsyncLifecycle(),
		sem:          make(chan struct{}, 10),
	}
	s.scanNow = s.scan
	s.recoverOrphans = s.recoverOrphanedInstances
	s.runFlow = s.executor.Execute
	return s
}

// SetInterval 设置调度间隔
func (s *Scheduler) SetInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = interval
}
