package repository

import "github.com/google/uuid"

type IncidentSyncOptions struct {
	IncidentID        uuid.UUID
	HealingStatus     string
	MatchedRuleID     *uuid.UUID
	FlowInstanceID    *uuid.UUID
	Scanned           *bool
	ResetFlowInstance bool
}

func incidentSyncUpdates(opts IncidentSyncOptions) map[string]interface{} {
	updates := map[string]interface{}{}
	if opts.HealingStatus != "" {
		updates["healing_status"] = opts.HealingStatus
	}
	if opts.MatchedRuleID != nil {
		updates["matched_rule_id"] = *opts.MatchedRuleID
	}
	if opts.FlowInstanceID != nil {
		updates["healing_flow_instance_id"] = *opts.FlowInstanceID
	}
	if opts.Scanned != nil {
		updates["scanned"] = *opts.Scanned
	}
	if opts.ResetFlowInstance {
		updates["healing_flow_instance_id"] = nil
	}
	return updates
}
