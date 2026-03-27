package httpapi

import (
	"strconv"
	"time"

	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func buildFlowInstanceListOptions(c *gin.Context, page, pageSize int) automationrepo.FlowInstanceListOptions {
	opts := automationrepo.FlowInstanceListOptions{
		Page:           page,
		PageSize:       pageSize,
		Status:         c.Query("status"),
		FlowName:       GetStringFilter(c, "flow_name"),
		RuleName:       GetStringFilter(c, "rule_name"),
		IncidentTitle:  GetStringFilter(c, "incident_title"),
		CurrentNodeID:  c.Query("current_node_id"),
		ErrorMessage:   GetStringFilter(c, "error_message"),
		SortBy:         c.Query("sort_by"),
		SortOrder:      c.Query("sort_order"),
		ApprovalStatus: c.Query("approval_status"),
		CreatedFrom:    parseOptionalTime(c.Query("created_from")),
		CreatedTo:      parseOptionalTime(c.Query("created_to")),
		StartedFrom:    parseOptionalTime(c.Query("started_from")),
		StartedTo:      parseOptionalTime(c.Query("started_to")),
		CompletedFrom:  parseOptionalTime(c.Query("completed_from")),
		CompletedTo:    parseOptionalTime(c.Query("completed_to")),
		MinNodes:       parseOptionalInt(c.Query("min_nodes")),
		MaxNodes:       parseOptionalInt(c.Query("max_nodes")),
		MinFailedNodes: parseOptionalInt(c.Query("min_failed_nodes")),
		MaxFailedNodes: parseOptionalInt(c.Query("max_failed_nodes")),
	}
	applyFlowInstanceUUIDOptions(&opts, c)
	applyFlowInstanceBoolOptions(&opts, c)
	return opts
}

func applyFlowInstanceUUIDOptions(opts *automationrepo.FlowInstanceListOptions, c *gin.Context) {
	opts.FlowID = parseOptionalUUID(c.Query("flow_id"))
	opts.RuleID = parseOptionalUUID(c.Query("rule_id"))
	opts.IncidentID = parseOptionalUUID(c.Query("incident_id"))
}

func applyFlowInstanceBoolOptions(opts *automationrepo.FlowInstanceListOptions, c *gin.Context) {
	if str := c.Query("has_error"); str != "" {
		val := str == "true" || str == "1"
		opts.HasError = &val
	}
}

func parseOptionalUUID(value string) *uuid.UUID {
	if value == "" {
		return nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseOptionalTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return &t
	}
	return nil
}

func parseOptionalInt(value string) *int {
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}
