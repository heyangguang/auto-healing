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
	flow, shouldClose, err := e.resolveAutoCloseFlow(ctx, instance)
	if err != nil {
		e.logNode(ctx, instance.ID, "", "system", model.LogLevelWarn, "自动关单配置读取失败", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	if !shouldClose || instance.IncidentID == nil {
		return
	}

	result, closeErr := e.incidentCloser.CloseIncident(ctx, IncidentCloseParams{
		IncidentID:     *instance.IncidentID,
		CloseStatus:    autoCloseSuccessStatus,
		Resolution:     fmt.Sprintf("自愈流程「%s」执行成功", flow.Name),
		WorkNotes:      fmt.Sprintf("自愈流程实例 %s 执行完成，系统自动关闭源工单。", instance.ID),
		CloseCode:      "auto_healed",
		TriggerSource:  platformmodel.IncidentWritebackTriggerFlowAutoClose,
		OperatorName:   autoCloseOperatorName,
		FlowInstanceID: &instance.ID,
		ExecutionRunID: autoCloseExecutionRunID(instance),
	})
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

func (e *FlowExecutor) resolveAutoCloseFlow(ctx context.Context, instance *model.FlowInstance) (*model.HealingFlow, bool, error) {
	flow, err := e.flowRepo.GetByID(ctx, instance.FlowID)
	if err != nil {
		return nil, false, err
	}
	if flow == nil || !flow.AutoCloseSourceIncident {
		return flow, false, nil
	}
	return flow, true, nil
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
