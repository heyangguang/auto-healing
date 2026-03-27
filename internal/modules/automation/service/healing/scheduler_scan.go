package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

// scan 扫描未处理的工单
func (s *Scheduler) scan(ctx context.Context) {
	s.processExpiredApprovals(ctx)

	incidents, err := s.incidentRepo.ListUnscanned(ctx, 100)
	if err != nil {
		logger.Sched("HEAL").Error("获取未扫描工单失败: %v", err)
		return
	}
	if len(incidents) == 0 {
		return
	}
	logger.Sched("HEAL").Info("发现 %d 个未扫描工单", len(incidents))

	rules, err := s.ruleRepo.ListActiveByPriority(ctx)
	if err != nil {
		logger.Sched("HEAL").Error("获取规则失败: %v", err)
		return
	}
	if len(rules) == 0 {
		s.markIncidentsScannedWithoutRules(ctx, incidents)
		return
	}

	for _, incident := range incidents {
		s.processIncident(schedulerTenantContext(ctx, &incident), &incident, rules)
	}
}

func (s *Scheduler) markIncidentsScannedWithoutRules(ctx context.Context, incidents []platformmodel.Incident) {
	for _, incident := range incidents {
		incidentCtx := schedulerTenantContext(ctx, &incident)
		if err := s.syncIncidentSkipped(incidentCtx, incident.ID); err != nil {
			logger.Sched("HEAL").Error("标记工单 %s 已跳过失败: %v", incident.ID, err)
		}
	}
}

func schedulerTenantContext(ctx context.Context, incident *platformmodel.Incident) context.Context {
	if incident.TenantID != nil {
		return platformrepo.WithTenantID(ctx, *incident.TenantID)
	}
	return ctx
}

// processIncident 处理单个工单
func (s *Scheduler) processIncident(ctx context.Context, incident *platformmodel.Incident, rules []model.HealingRule) {
	matchedRule := s.matchIncidentRule(ctx, incident, rules)
	if matchedRule == nil {
		if err := s.syncIncidentSkipped(ctx, incident.ID); err != nil {
			logger.Sched("HEAL").Error("更新工单 %s 跳过状态失败: %v", incident.ID, err)
			return
		}
		incident.HealingStatus = "skipped"
		logger.Sched("HEAL").Debug("工单 %s 无匹配规则，已跳过", incident.ID)
		return
	}

	logger.Sched("HEAL").Info("工单 %s 匹配规则 %s (%s)", incident.ID, matchedRule.ID, matchedRule.Name)
	switch matchedRule.TriggerMode {
	case model.TriggerModeAuto:
		s.processAutoTriggeredIncident(ctx, incident, matchedRule)
	case model.TriggerModeManual:
		if err := s.markIncidentScanned(ctx, incident.ID, &matchedRule.ID, nil); err != nil {
			logger.Sched("HEAL").Error("标记工单 %s 手动触发待处理失败: %v", incident.ID, err)
			return
		}
		logger.Sched("HEAL").Info("工单 %s 等待手动触发", incident.ID)
	}
}

func (s *Scheduler) syncIncidentSkipped(ctx context.Context, incidentID uuid.UUID) error {
	scanned := true
	return s.incidentRepo.SyncState(ctx, incidentrepo.IncidentSyncOptions{
		IncidentID:    incidentID,
		HealingStatus: "skipped",
		Scanned:       &scanned,
	})
}

func (s *Scheduler) matchIncidentRule(ctx context.Context, incident *platformmodel.Incident, rules []model.HealingRule) *model.HealingRule {
	for i := range rules {
		rule := &rules[i]
		if incident.TenantID != nil && rule.TenantID != nil && *rule.TenantID != *incident.TenantID {
			continue
		}
		if s.matcher.Match(ctx, incident, rule) {
			return rule
		}
	}
	return nil
}

func (s *Scheduler) processAutoTriggeredIncident(ctx context.Context, incident *platformmodel.Incident, rule *model.HealingRule) {
	instance, err := s.createFlowInstance(ctx, incident, rule)
	if err != nil {
		logger.Sched("HEAL").Error("创建流程实例失败: %v", err)
		if markErr := s.markIncidentScanned(ctx, incident.ID, &rule.ID, nil); markErr != nil {
			logger.Sched("HEAL").Error("创建流程实例失败后标记工单 %s 已扫描失败: %v", incident.ID, markErr)
		}
		return
	}
	s.ruleRepo.UpdateLastRunAt(ctx, rule.ID)
	s.scheduleAutoFlowExecution(instance, incident.ID)
}

// createFlowInstance 创建流程实例（快照流程定义）
func (s *Scheduler) createFlowInstance(ctx context.Context, incident *platformmodel.Incident, rule *model.HealingRule) (*model.FlowInstance, error) {
	if rule.FlowID == nil {
		return nil, fmt.Errorf("规则 %s 未关联流程", rule.ID)
	}
	flow, err := s.flowRepo.GetByID(ctx, *rule.FlowID)
	if err != nil {
		return nil, err
	}

	instance := &model.FlowInstance{
		FlowID:     *rule.FlowID,
		FlowName:   flow.Name,
		FlowNodes:  flow.Nodes,
		FlowEdges:  flow.Edges,
		RuleID:     &rule.ID,
		IncidentID: &incident.ID,
		Status:     model.FlowInstanceStatusPending,
		Context:    model.JSON{"incident": incidentToMap(incident)},
	}
	scanned := true
	if err := s.instanceRepo.CreateWithIncidentSync(ctx, instance, automationrepo.IncidentSyncOptions{
		IncidentID:     incident.ID,
		HealingStatus:  "processing",
		MatchedRuleID:  &rule.ID,
		FlowInstanceID: &instance.ID,
		Scanned:        &scanned,
	}); err != nil {
		return nil, err
	}
	incident.HealingStatus = "processing"

	logger.Sched("HEAL").Info("创建流程实例 %s（快照流程 %s）", instance.ID, flow.Name)
	return instance, nil
}

// incidentToMap 将 Incident 结构体转换为 map，确保 JSON 序列化正确
func incidentToMap(incident *platformmodel.Incident) map[string]interface{} {
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
