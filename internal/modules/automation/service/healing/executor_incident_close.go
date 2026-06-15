package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

const autoCloseOperatorName = "system:auto-close"

func (e *FlowExecutor) tryCloseSourceIncident(ctx context.Context, instance *model.FlowInstance) {
	flow, policy, err := e.resolveAutoCloseFlow(ctx, instance)
	if err != nil {
		e.logNode(ctx, instance.ID, "", "system", model.LogLevelWarn, "自动关单配置读取失败", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	if instance.IncidentID == nil || !policy.Enabled {
		return
	}
	if !autoCloseAllowedByExecutionResult(instance) {
		e.logNode(ctx, instance.ID, "", "system", model.LogLevelWarn, "自动关单已跳过", map[string]interface{}{
			"incident_id": instance.IncidentID.String(),
			"reason":      "execution_result_not_success",
		})
		return
	}
	params, closeErr := e.buildAutoCloseIncidentParams(ctx, instance, flow, policy)
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

func autoCloseAllowedByExecutionResult(instance *model.FlowInstance) bool {
	if instance == nil || instance.Context == nil {
		return true
	}
	result, ok := instance.Context["execution_result"].(map[string]interface{})
	if !ok {
		return true
	}
	status, _ := result["status"].(string)
	switch status {
	case "", "completed", "success":
		return true
	default:
		return false
	}
}

func (e *FlowExecutor) buildAutoCloseIncidentParams(ctx context.Context, instance *model.FlowInstance, flow *model.HealingFlow, policy flowClosePolicy) (IncidentCloseParams, error) {
	params := IncidentCloseParams{
		IncidentID:     *instance.IncidentID,
		TriggerSource:  platformmodel.IncidentWritebackTriggerFlowAutoClose,
		OperatorName:   autoCloseOperatorName,
		FlowInstanceID: &instance.ID,
		ExecutionRunID: autoCloseExecutionRunID(instance),
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
	params.TemplateVars = e.buildAutoCloseTemplateVars(ctx, instance, flow)
	return params, nil
}

func (e *FlowExecutor) resolveAutoCloseFlow(ctx context.Context, instance *model.FlowInstance) (*model.HealingFlow, flowClosePolicy, error) {
	flow, err := e.flowRepo.GetByID(ctx, instance.FlowID)
	if err != nil {
		return nil, flowClosePolicy{}, err
	}
	if flow == nil {
		return nil, flowClosePolicy{}, nil
	}
	policy, err := resolveFlowClosePolicy(flow)
	if err != nil {
		return nil, flowClosePolicy{}, err
	}
	return flow, policy, nil
}

func buildAutoCloseExecutionContext(instance *model.FlowInstance) map[string]any {
	execution := map[string]any{
		"status":       "",
		"message":      "",
		"run_id":       "",
		"target_hosts": "",
		"task_id":      "",
		"stats":        map[string]any{},
		"stdout":       "",
		"stderr":       "",
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
	if stdout, ok := result["stdout"]; ok {
		execution["stdout"] = stdout
	}
	if stderr, ok := result["stderr"]; ok {
		execution["stderr"] = stderr
	}
	if run, ok := result["run"].(map[string]interface{}); ok {
		if runID, ok := run["run_id"]; ok {
			execution["run_id"] = runID
		}
		if stats, ok := run["stats"]; ok {
			execution["stats"] = stats
		}
		if stdout, ok := run["stdout"]; ok && execution["stdout"] == "" {
			execution["stdout"] = stdout
		}
		if stderr, ok := run["stderr"]; ok && execution["stderr"] == "" {
			execution["stderr"] = stderr
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
