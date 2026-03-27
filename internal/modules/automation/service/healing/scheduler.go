package healing

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
)

// Scheduler 全局自愈调度器
type Scheduler struct {
	ruleRepo     *automationrepo.HealingRuleRepository
	flowRepo     *automationrepo.HealingFlowRepository
	instanceRepo *automationrepo.FlowInstanceRepository
	incidentRepo *incidentrepo.IncidentRepository
	approvalRepo *automationrepo.ApprovalTaskRepository

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
		ruleRepo:     automationrepo.NewHealingRuleRepository(),
		flowRepo:     automationrepo.NewHealingFlowRepository(),
		instanceRepo: automationrepo.NewFlowInstanceRepository(),
		incidentRepo: incidentrepo.NewIncidentRepository(),
		approvalRepo: automationrepo.NewApprovalTaskRepository(),
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
