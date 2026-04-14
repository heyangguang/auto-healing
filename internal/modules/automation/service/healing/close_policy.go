package healing

import (
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

const flowClosePolicyTriggerOnSuccess = "flow_success"

type flowClosePolicy struct {
	Enabled            bool
	TriggerOn          string
	SolutionTemplateID *uuid.UUID
	DefaultCloseCode   string
	DefaultCloseStatus string
}

func resolveFlowClosePolicy(flow *model.HealingFlow) (flowClosePolicy, error) {
	policy := flowClosePolicy{}
	if flow == nil || len(flow.ClosePolicy) == 0 {
		return policy, nil
	}
	rawEnabled, _ := flow.ClosePolicy["enabled"].(bool)
	policy.Enabled = rawEnabled
	if rawTrigger, ok := flow.ClosePolicy["trigger_on"].(string); ok {
		policy.TriggerOn = rawTrigger
	}
	if rawCode, ok := flow.ClosePolicy["default_close_code"].(string); ok {
		policy.DefaultCloseCode = rawCode
	}
	if rawStatus, ok := flow.ClosePolicy["default_close_status"].(string); ok {
		policy.DefaultCloseStatus = rawStatus
	}
	rawTemplateID, _ := flow.ClosePolicy["solution_template_id"].(string)
	if rawTemplateID == "" {
		return policy, nil
	}
	id, err := uuid.Parse(rawTemplateID)
	if err != nil {
		return policy, fmt.Errorf("close_policy.solution_template_id 无效: %w", err)
	}
	policy.SolutionTemplateID = &id
	return policy, nil
}

func (p flowClosePolicy) isFlowSuccessTrigger() bool {
	return p.TriggerOn == "" || p.TriggerOn == flowClosePolicyTriggerOnSuccess
}
