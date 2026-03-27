package healing

import (
	"context"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
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

type SchedulerDeps struct {
	RuleRepo     *automationrepo.HealingRuleRepository
	FlowRepo     *automationrepo.HealingFlowRepository
	InstanceRepo *automationrepo.FlowInstanceRepository
	IncidentRepo *incidentrepo.IncidentRepository
	ApprovalRepo *automationrepo.ApprovalTaskRepository
	Matcher      *RuleMatcher
	Executor     *FlowExecutor
	Interval     time.Duration
	Lifecycle    *asyncLifecycle
	Sem          chan struct{}
}

func DefaultSchedulerDeps(executor *FlowExecutor) SchedulerDeps {
	return SchedulerDeps{
		RuleRepo:     automationrepo.NewHealingRuleRepository(),
		FlowRepo:     automationrepo.NewHealingFlowRepository(),
		InstanceRepo: automationrepo.NewFlowInstanceRepository(),
		IncidentRepo: incidentrepo.NewIncidentRepository(),
		ApprovalRepo: automationrepo.NewApprovalTaskRepository(),
		Matcher:      NewRuleMatcher(),
		Executor:     executor,
		Interval:     10 * time.Second,
		Lifecycle:    newAsyncLifecycle(),
		Sem:          make(chan struct{}, 10),
	}
}

func DefaultSchedulerRuntimeDeps() SchedulerDeps {
	return DefaultSchedulerDeps(NewFlowExecutor())
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return NewSchedulerWithDeps(DefaultSchedulerRuntimeDeps())
}

func NewSchedulerWithDeps(deps SchedulerDeps) *Scheduler {
	s := &Scheduler{
		ruleRepo:     deps.RuleRepo,
		flowRepo:     deps.FlowRepo,
		instanceRepo: deps.InstanceRepo,
		incidentRepo: deps.IncidentRepo,
		approvalRepo: deps.ApprovalRepo,
		matcher:      deps.Matcher,
		executor:     deps.Executor,
		interval:     deps.Interval,
		lifecycle:    deps.Lifecycle,
		sem:          deps.Sem,
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
