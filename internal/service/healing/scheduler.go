package healing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
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

	interval time.Duration
	running  bool
	stopChan chan struct{}
	mu       sync.Mutex
	sem      chan struct{} // 并发执行限制
}

// NewScheduler 创建调度器
func NewScheduler() *Scheduler {
	return &Scheduler{
		ruleRepo:     repository.NewHealingRuleRepository(),
		flowRepo:     repository.NewHealingFlowRepository(),
		instanceRepo: repository.NewFlowInstanceRepository(),
		incidentRepo: repository.NewIncidentRepository(),
		approvalRepo: repository.NewApprovalTaskRepository(),

		matcher:  NewRuleMatcher(),
		executor: NewFlowExecutor(),

		interval: 10 * time.Second,
		stopChan: make(chan struct{}),
		sem:      make(chan struct{}, 10), // 最多 10 个并发流程执行
	}
}

// SetInterval 设置调度间隔
func (s *Scheduler) SetInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = interval
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	logger.Sched("HEAL").Info("调度器启动，间隔: %v", s.interval)

	// 启动时恢复孤儿实例（服务重启后遗留的 running 实例）
	s.recoverOrphanedInstances()

	go s.run()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
	logger.Sched("HEAL").Info("调度器已停止")
}

// IsRunning 检查是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run 调度循环
func (s *Scheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	s.scan()

	for {
		select {
		case <-ticker.C:
			s.scan()
		case <-s.stopChan:
			return
		}
	}
}

// scan 扫描未处理的工单
func (s *Scheduler) scan() {
	ctx := context.Background()

	// 1. 处理超时的审批任务
	s.processExpiredApprovals(ctx)

	// 2. 获取未扫描的工单
	incidents, err := s.incidentRepo.ListUnscanned(ctx, 100)
	if err != nil {
		logger.Sched("HEAL").Error("获取未扫描工单失败: %v", err)
		return
	}

	if len(incidents) == 0 {
		return
	}

	logger.Sched("HEAL").Info("发现 %d 个未扫描工单", len(incidents))

	// 3. 获取所有启用的规则（按优先级排序）
	rules, err := s.ruleRepo.ListActiveByPriority(ctx)
	if err != nil {
		logger.Sched("HEAL").Error("获取规则失败: %v", err)
		return
	}

	if len(rules) == 0 {
		// 没有启用的规则，标记所有工单为已扫描
		for _, incident := range incidents {
			s.incidentRepo.MarkScanned(ctx, incident.ID, nil, nil)
		}
		return
	}

	// 4. 对每个工单尝试匹配规则（使用该工单所属租户的 context）
	for _, incident := range incidents {
		// 注入工单所属租户的 context，确保 MarkScanned/Update/UpdateLastRunAt 均在正确租户范围内操作
		incCtx := ctx
		if incident.TenantID != nil {
			incCtx = repository.WithTenantID(ctx, *incident.TenantID)
		}
		s.processIncident(incCtx, &incident, rules)
	}
}

// processIncident 处理单个工单
func (s *Scheduler) processIncident(ctx context.Context, incident *model.Incident, rules []model.HealingRule) {
	var matchedRule *model.HealingRule

	// 按优先级从高到低尝试匹配（仅匹配与该工单同一租户的规则）
	for i := range rules {
		rule := &rules[i]
		// 租户隔离：规则必须属于与工单相同的租户
		if incident.TenantID != nil && rule.TenantID != nil && *rule.TenantID != *incident.TenantID {
			continue
		}
		if s.matcher.Match(ctx, incident, rule) {
			matchedRule = rule
			break // 匹配成功，不再尝试后续规则
		}
	}

	if matchedRule == nil {
		// 没有匹配的规则，标记为已扫描并设置为 skipped
		// 注意：必须先更新内存对象再调用 Update，否则 Save 会把 scanned 覆盖回 false
		incident.Scanned = true
		incident.HealingStatus = "skipped"
		s.incidentRepo.Update(ctx, incident)
		logger.Sched("HEAL").Debug("工单 %s 无匹配规则，已跳过", incident.ID)
		return
	}

	logger.Sched("HEAL").Info("工单 %s 匹配规则 %s (%s)", incident.ID, matchedRule.ID, matchedRule.Name)

	// 根据触发模式处理
	switch matchedRule.TriggerMode {
	case model.TriggerModeAuto:
		// 自动触发：立即创建流程实例并执行
		instance, err := s.createFlowInstance(ctx, incident, matchedRule)
		if err != nil {
			logger.Sched("HEAL").Error("创建流程实例失败: %v", err)
			s.incidentRepo.MarkScanned(ctx, incident.ID, &matchedRule.ID, nil)
			return
		}

		// 标记工单为已扫描
		s.incidentRepo.MarkScanned(ctx, incident.ID, &matchedRule.ID, &instance.ID)

		// 更新规则最后运行时间
		s.ruleRepo.UpdateLastRunAt(ctx, matchedRule.ID)

		// 异步执行流程（并发限制）
		select {
		case s.sem <- struct{}{}: // 获取令牌
			go func(inst *model.FlowInstance) {
				defer func() { <-s.sem }() // 释放令牌
				// 注入该实例所属租户的 context，供流程节点（CMDB等）使用
				execCtx := context.Background()
				if inst.TenantID != nil {
					execCtx = repository.WithTenantID(execCtx, *inst.TenantID)
				}
				defer func() {
					if r := recover(); r != nil {
						logger.Sched("HEAL").Error("流程执行 panic [%s]: %v", inst.ID.String()[:8], fmt.Sprintf("%v", r))
						s.instanceRepo.UpdateStatus(execCtx, inst.ID,
							model.FlowInstanceStatusFailed,
							fmt.Sprintf("执行异常(panic): %v", r))
					}
				}()
				s.executor.Execute(execCtx, inst)
			}(instance)
		default:
			logger.Sched("HEAL").Warn("并发执行达到上限，工单 %s 延迟执行", incident.ID)
			go func(inst *model.FlowInstance) {
				s.sem <- struct{}{} // 等待令牌
				defer func() { <-s.sem }()
				// 注入该实例所属租户的 context
				execCtx := context.Background()
				if inst.TenantID != nil {
					execCtx = repository.WithTenantID(execCtx, *inst.TenantID)
				}
				defer func() {
					if r := recover(); r != nil {
						logger.Sched("HEAL").Error("流程执行 panic [%s]: %v", inst.ID.String()[:8], fmt.Sprintf("%v", r))
						s.instanceRepo.UpdateStatus(execCtx, inst.ID,
							model.FlowInstanceStatusFailed,
							fmt.Sprintf("执行异常(panic): %v", r))
					}
				}()
				s.executor.Execute(execCtx, inst)
			}(instance)
		}

	case model.TriggerModeManual:
		// 手动触发：只标记匹配，不创建流程实例
		s.incidentRepo.MarkScanned(ctx, incident.ID, &matchedRule.ID, nil)
		logger.Sched("HEAL").Info("工单 %s 等待手动触发", incident.ID)
	}
}

// createFlowInstance 创建流程实例（快照流程定义）
func (s *Scheduler) createFlowInstance(ctx context.Context, incident *model.Incident, rule *model.HealingRule) (*model.FlowInstance, error) {
	if rule.FlowID == nil {
		return nil, fmt.Errorf("规则 %s 未关联流程", rule.ID)
	}

	// 获取当前流程定义，用于快照
	flow, err := s.flowRepo.GetByID(ctx, *rule.FlowID)
	if err != nil {
		return nil, err
	}

	// 将 incident 结构体转换为 map，确保 JSON 序列化正确
	incidentMap := incidentToMap(incident)

	instance := &model.FlowInstance{
		FlowID:     *rule.FlowID,
		FlowName:   flow.Name,
		FlowNodes:  flow.Nodes,
		FlowEdges:  flow.Edges,
		RuleID:     &rule.ID,
		IncidentID: &incident.ID,
		Status:     model.FlowInstanceStatusPending,
		Context:    model.JSON{"incident": incidentMap},
	}

	if err := s.instanceRepo.Create(ctx, instance); err != nil {
		return nil, err
	}

	// 更新工单自愈状态为 processing
	incident.HealingStatus = "processing"
	if err := s.incidentRepo.Update(ctx, incident); err != nil {
		logger.Sched("HEAL").Error("更新工单自愈状态失败: %v", err)
	}

	logger.Sched("HEAL").Info("创建流程实例 %s（快照流程 %s）", instance.ID, flow.Name)
	return instance, nil
}

// incidentToMap 将 Incident 结构体转换为 map，确保 JSON 序列化正确
func incidentToMap(incident *model.Incident) map[string]interface{} {
	if incident == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":                   incident.ID.String(),
		"plugin_id":            nil,
		"source_plugin_name":   incident.SourcePluginName,
		"external_id":          incident.ExternalID,
		"title":                incident.Title,
		"description":          incident.Description,
		"severity":             incident.Severity,
		"priority":             incident.Priority,
		"status":               incident.Status,
		"category":             incident.Category,
		"affected_ci":          incident.AffectedCI,
		"affected_service":     incident.AffectedService,
		"assignee":             incident.Assignee,
		"reporter":             incident.Reporter,
		"raw_data":             incident.RawData,
		"healing_status":       incident.HealingStatus,
		"workflow_instance_id": nil,
		"scanned":              incident.Scanned,
	}

	if incident.PluginID != nil {
		result["plugin_id"] = incident.PluginID.String()
	}
	if incident.WorkflowInstanceID != nil {
		result["workflow_instance_id"] = incident.WorkflowInstanceID.String()
	}
	if incident.SourceCreatedAt != nil {
		result["source_created_at"] = incident.SourceCreatedAt.Format("2006-01-02 15:04:05")
	}
	if incident.SourceUpdatedAt != nil {
		result["source_updated_at"] = incident.SourceUpdatedAt.Format("2006-01-02 15:04:05")
	}

	return result
}

// processExpiredApprovals 处理超时的审批任务
func (s *Scheduler) processExpiredApprovals(ctx context.Context) {
	// 直接执行超时更新，返回实际更新的数量
	count, err := s.approvalRepo.ExpireTimedOut(ctx)
	if err != nil {
		logger.Sched("HEAL").Error("处理超时审批失败: %v", err)
		return
	}
	if count == 0 {
		return
	}

	// 查询已被标记为超时的任务（而不是先查后改）
	var expiredTasks []model.ApprovalTask
	database.DB.WithContext(ctx).
		Where("status = ?", "expired").
		Where("updated_at >= NOW() - INTERVAL '1 minute'").
		Find(&expiredTasks)

	// 更新关联的 FlowInstance 和 Incident 状态
	for _, task := range expiredTasks {
		// 注入该审批任务所属租户的 context，确保后续 DB 操作在正确租户范围内
		taskCtx := ctx
		if task.TenantID != nil {
			taskCtx = repository.WithTenantID(ctx, *task.TenantID)
		}
		updated, err := s.instanceRepo.UpdateStatusIfCurrent(
			taskCtx,
			task.FlowInstanceID,
			[]string{model.FlowInstanceStatusWaitingApproval},
			model.FlowInstanceStatusFailed,
			"审批超时",
		)
		if err != nil || !updated {
			continue
		}

		if instance, err := s.instanceRepo.GetByID(taskCtx, task.FlowInstanceID); err == nil && instance.IncidentID != nil {
			if incident, err := s.incidentRepo.GetByID(taskCtx, *instance.IncidentID); err == nil {
				incident.HealingStatus = "failed"
				s.incidentRepo.Update(taskCtx, incident)
				logger.Sched("HEAL").Info("审批超时，工单 %s 状态已更新为 failed", incident.ID.String()[:8])
			}
		}
	}

	logger.Sched("HEAL").Info("已将 %d 个审批任务标记为超时", count)
}

// TriggerManual 手动触发流程
func (s *Scheduler) TriggerManual(ctx context.Context, incidentID string, ruleID uuid.UUID) (*model.FlowInstance, error) {
	// 获取工单
	incident, err := s.incidentRepo.GetByID(ctx, parseUUID(incidentID))
	if err != nil {
		return nil, err
	}

	// 获取规则
	rule, err := s.ruleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}

	// 创建流程实例
	instance, err := s.createFlowInstance(ctx, incident, rule)
	if err != nil {
		return nil, err
	}

	// 更新工单
	s.incidentRepo.MarkScanned(ctx, incident.ID, &rule.ID, &instance.ID)

	// 异步执行流程（使用独立 context，避免 HTTP 请求结束后 context 被取消，并发限制）
	execCtx := context.Background()
	if instance.TenantID != nil {
		execCtx = repository.WithTenantID(execCtx, *instance.TenantID)
	}
	s.sem <- struct{}{} // 获取令牌
	go func(inst *model.FlowInstance) {
		defer func() { <-s.sem }() // 释放令牌
		defer func() {
			if r := recover(); r != nil {
				logger.Sched("HEAL").Error("手动触发流程 panic [%s]: %v", inst.ID.String()[:8], fmt.Sprintf("%v", r))
				s.instanceRepo.UpdateStatus(execCtx, inst.ID,
					model.FlowInstanceStatusFailed,
					fmt.Sprintf("执行异常(panic): %v", r))
			}
		}()
		s.executor.Execute(execCtx, inst)
	}(instance)

	return instance, nil
}

// recoverOrphanedInstances 恢复服务重启前遗留的 running/pending 实例
// 查找 updated_at 超过 30 分钟的实例，标记为 failed
func (s *Scheduler) recoverOrphanedInstances() {
	ctx := context.Background()
	staleThreshold := 30 * time.Minute

	instances, err := s.instanceRepo.ListStaleRunning(ctx, staleThreshold)
	if err != nil {
		logger.Sched("HEAL").Error("查询停滞实例失败: %v", err)
		return
	}

	if len(instances) == 0 {
		return
	}

	logger.Sched("HEAL").Warn("发现 %d 个停滞的实例，开始恢复...", len(instances))

	for _, inst := range instances {
		errMsg := fmt.Sprintf("服务重启恢复: 实例已停滞超过 %v (上次更新: %s)",
			staleThreshold, inst.UpdatedAt.Format("2006-01-02 15:04:05"))
		// 注入该实例所属租户的 context
		instCtx := ctx
		if inst.TenantID != nil {
			instCtx = repository.WithTenantID(ctx, *inst.TenantID)
		}
		if err := s.instanceRepo.UpdateStatus(instCtx, inst.ID, model.FlowInstanceStatusFailed, errMsg); err != nil {
			logger.Sched("HEAL").Error("恢复实例 %s 失败: %v", inst.ID.String()[:8], err)
			continue
		}

		// 更新关联工单的自愈状态
		if inst.IncidentID != nil {
			if incident, err := s.incidentRepo.GetByID(instCtx, *inst.IncidentID); err == nil {
				incident.HealingStatus = "failed"
				s.incidentRepo.Update(instCtx, incident)
			}
		}

		logger.Sched("HEAL").Warn("已恢复孤儿实例 %s (%s) -> failed", inst.ID.String()[:8], inst.FlowName)
	}
}
