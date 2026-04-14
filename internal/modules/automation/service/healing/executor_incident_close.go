package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

const autoCloseSuccessStatus = "resolved"
const autoCloseOperatorName = "system:auto-close"

func (e *FlowExecutor) tryAutoCloseSourceIncident(ctx context.Context, instance *model.FlowInstance) {
	flow, policy, useLegacy, err := e.resolveAutoCloseFlow(ctx, instance)
	if err != nil {
		e.logNode(ctx, instance.ID, "", "system", model.LogLevelWarn, "自动关单配置读取失败", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	if instance.IncidentID == nil || (!useLegacy && !policy.Enabled) {
		return
	}
	params, closeErr := e.buildAutoCloseIncidentParams(instance, flow, policy, useLegacy)
	if closeErr != nil {
		e.logNode(ctx, instance.ID, "", "system", model.LogLevelWarn, "自动关单配置无效", map[string]interface{}{
			"incident_id": instance.IncidentID.String(),
			"error":       closeErr.Error(),
		})
		return
	}
	result, closeErr := e.incidentCloser.CloseIncident(ctx, params)
	if closeErr != nil {
		e.logNode(ctx, instance.ID, "", "system", model.LogLevelWarn, "自动回写关闭源工单失败", map[string]interface{}{
			"incident_id": instance.IncidentID.String(),
			"error":       closeErr.Error(),
		})
		return
	}

	e.logNode(ctx, instance.ID, "", "system", model.LogLevelInfo, "自动回写关闭源工单完成", map[string]interface{}{
		"incident_id":      instance.IncidentID.String(),
		"local_status":     result.LocalStatus,
		"source_updated":   result.SourceUpdated,
		"writeback_log_id": uuidString(result.WritebackLogID),
	})
}

func (e *FlowExecutor) buildAutoCloseIncidentParams(instance *model.FlowInstance, flow *model.HealingFlow, policy flowClosePolicy, useLegacy bool) (IncidentCloseParams, error) {
	params := IncidentCloseParams{
		IncidentID:     *instance.IncidentID,
		TriggerSource:  platformmodel.IncidentWritebackTriggerFlowAutoClose,
		OperatorName:   autoCloseOperatorName,
		FlowInstanceID: &instance.ID,
		ExecutionRunID: autoCloseExecutionRunID(instance),
	}
	if useLegacy {
		params.CloseStatus = autoCloseSuccessStatus
		params.Resolution = fmt.Sprintf("自愈流程「%s」执行成功", flow.Name)
		params.WorkNotes = fmt.Sprintf("自愈流程实例 %s 执行完成，系统自动关闭源工单。", instance.ID)
		params.CloseCode = "auto_healed"
		return params, nil
	}
	if !policy.isFlowSuccessTrigger() {
		return params, fmt.Errorf("close_policy.trigger_on 仅支持 %s", flowClosePolicyTriggerOnSuccess)
	}
	if policy.SolutionTemplateID == nil {
		return params, fmt.Errorf("close_policy 已启用但未配置 solution_template_id")
	}
	params.SolutionTemplateID = policy.SolutionTemplateID
	params.CloseStatus = policy.DefaultCloseStatus
	params.CloseCode = policy.DefaultCloseCode
	params.TemplateVars = buildAutoCloseTemplateVars(instance, flow)
	return params, nil
}

func (e *FlowExecutor) resolveAutoCloseFlow(ctx context.Context, instance *model.FlowInstance) (*model.HealingFlow, flowClosePolicy, bool, error) {
	flow, err := e.flowRepo.GetByID(ctx, instance.FlowID)
	if err != nil {
		return nil, flowClosePolicy{}, false, err
	}
	if flow == nil {
		return nil, flowClosePolicy{}, false, nil
	}
	policy, err := resolveFlowClosePolicy(flow)
	if err != nil {
		return nil, flowClosePolicy{}, false, err
	}
	if policy.Enabled {
		return flow, policy, false, nil
	}
	return flow, flowClosePolicy{}, flow.AutoCloseSourceIncident, nil
}

func buildAutoCloseTemplateVars(instance *model.FlowInstance, flow *model.HealingFlow) map[string]any {
	return map[string]any{
		"flow": map[string]any{
			"id":          flow.ID.String(),
			"name":        flow.Name,
			"instance_id": instance.ID.String(),
		},
		"execution": buildAutoCloseExecutionContext(instance),
	}
}

func buildAutoCloseExecutionContext(instance *model.FlowInstance) map[string]any {
	execution := map[string]any{
		"status":       "",
		"message":      "",
		"run_id":       "",
		"target_hosts": "",
		"task_id":      "",
		"stats":        map[string]any{},
	}
	if instance == nil || instance.Context == nil {
		return execution
	}
	result, ok := instance.Context["execution_result"].(map[string]interface{})
	if !ok {
		return execution
	}
	if status, ok := result["status"]; ok {
		execution["status"] = status
	}
	if message, ok := result["message"]; ok {
		execution["message"] = message
	}
	if taskID, ok := result["task_id"]; ok {
		execution["task_id"] = taskID
	}
	if targetHosts, ok := result["target_hosts"]; ok {
		execution["target_hosts"] = targetHosts
	}
	if run, ok := result["run"].(map[string]interface{}); ok {
		if runID, ok := run["run_id"]; ok {
			execution["run_id"] = runID
		}
		if stats, ok := run["stats"]; ok {
			execution["stats"] = stats
		}
	}
	return execution
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func autoCloseExecutionRunID(instance *model.FlowInstance) *uuid.UUID {
	if instance == nil || instance.Context == nil {
		return nil
	}
	result, ok := instance.Context["execution_result"].(map[string]interface{})
	if !ok {
		return nil
	}
	run, ok := result["run"].(map[string]interface{})
	if !ok {
		return nil
	}
	runID, ok := run["run_id"].(string)
	if !ok || runID == "" {
		return nil
	}
	id, err := uuid.Parse(runID)
	if err != nil {
		return nil
	}
	return &id
}
