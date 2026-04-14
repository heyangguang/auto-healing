package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	integrationmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
)

var solutionPlaceholderPattern = regexp.MustCompile(`{{\s*([a-zA-Z0-9_.-]+)\s*}}`)

type renderedSolution struct {
	Resolution  string
	WorkNotes   string
	CloseCode   string
	CloseStatus string
}

func (s *IncidentService) resolveCloseIncidentParams(ctx context.Context, incident *platformmodel.Incident, params CloseIncidentParams) (CloseIncidentParams, error) {
	params = normalizeCloseIncidentParams(params)
	if params.SolutionTemplateID == nil {
		if params.Resolution == "" {
			return params, fmt.Errorf("必须提供 resolution 或 solution_template_id")
		}
		return params, nil
	}
	template, err := s.solutionRepo.GetByID(ctx, *params.SolutionTemplateID)
	if err != nil {
		return params, fmt.Errorf("获取解决方案模板失败: %w", err)
	}
	rendered, err := renderSolutionTemplate(template, buildSolutionTemplateContext(incident, params))
	if err != nil {
		return params, err
	}
	if params.Resolution == "" {
		params.Resolution = rendered.Resolution
	}
	if params.WorkNotes == "" {
		params.WorkNotes = rendered.WorkNotes
	}
	if params.CloseCode == "" {
		params.CloseCode = rendered.CloseCode
	}
	if params.CloseStatus == "" {
		params.CloseStatus = rendered.CloseStatus
	}
	if params.Resolution == "" {
		return params, fmt.Errorf("解决方案模板渲染后 resolution 为空")
	}
	return params, nil
}

func renderSolutionTemplate(template *integrationmodel.IncidentSolutionTemplate, context map[string]any) (*renderedSolution, error) {
	if template == nil {
		return nil, fmt.Errorf("解决方案模板不能为空")
	}
	if template.UsesStructuredSections() {
		return renderStructuredSolutionTemplate(template, context)
	}
	resolution, err := renderTemplate(template.ResolutionTemplate, context)
	if err != nil {
		return nil, fmt.Errorf("渲染 resolution_template 失败: %w", err)
	}
	workNotes, err := renderTemplate(template.WorkNotesTemplate, context)
	if err != nil {
		return nil, fmt.Errorf("渲染 work_notes_template 失败: %w", err)
	}
	return &renderedSolution{
		Resolution:  resolution,
		WorkNotes:   workNotes,
		CloseCode:   strings.TrimSpace(template.DefaultCloseCode),
		CloseStatus: defaultCloseStatus(template.DefaultCloseStatus),
	}, nil
}

func buildSolutionTemplateContext(incident *platformmodel.Incident, params CloseIncidentParams) map[string]any {
	context := map[string]any{
		"incident": buildIncidentTemplateContext(incident),
		"system": map[string]any{
			"trigger_source": params.TriggerSource,
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		},
		"operator": map[string]any{
			"name": params.OperatorName,
		},
		"input":        map[string]any{},
		"close_code":   params.CloseCode,
		"close_status": defaultCloseStatus(params.CloseStatus),
	}
	mergeTemplateVars(context, params.TemplateVars)
	if _, exists := context["steps_text"]; !exists {
		context["steps_text"] = ""
	}
	if len(templateContextSteps(context)) > 0 {
		context["steps_text"] = renderStepsText(context, &integrationmodel.IncidentSolutionTemplate{
			StepsRenderMode:     "summary",
			StepsMaxCount:       len(templateContextSteps(context)),
			StepOutputMaxLength: 240,
		})
	}
	return context
}

func buildIncidentTemplateContext(incident *platformmodel.Incident) map[string]any {
	if incident == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":               incident.ID.String(),
		"external_id":      incident.ExternalID,
		"title":            incident.Title,
		"description":      incident.Description,
		"severity":         incident.Severity,
		"priority":         incident.Priority,
		"status":           incident.Status,
		"category":         incident.Category,
		"affected_ci":      incident.AffectedCI,
		"affected_service": incident.AffectedService,
		"assignee":         incident.Assignee,
		"reporter":         incident.Reporter,
		"source_plugin":    incident.SourcePluginName,
	}
}

func mergeTemplateVars(context map[string]any, vars integrationmodel.JSON) {
	if len(vars) == 0 {
		return
	}
	input, _ := context["input"].(map[string]any)
	for key, value := range vars {
		input[key] = value
		if _, exists := context[key]; !exists {
			context[key] = value
		}
	}
}

func renderTemplate(template string, context map[string]any) (string, error) {
	matches := solutionPlaceholderPattern.FindAllStringSubmatch(template, -1)
	rendered := template
	for _, match := range matches {
		value, err := resolveTemplatePath(context, match[1])
		if err != nil {
			return "", err
		}
		rendered = strings.ReplaceAll(rendered, match[0], stringifyTemplateValue(value))
	}
	return strings.TrimSpace(rendered), nil
}

func resolveTemplatePath(context map[string]any, path string) (any, error) {
	current := any(context)
	for _, part := range strings.Split(path, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("模板变量 %s 不是有效对象路径", path)
		}
		value, exists := node[part]
		if !exists {
			return nil, fmt.Errorf("模板变量 %s 不存在", path)
		}
		current = value
	}
	if current == nil {
		return nil, fmt.Errorf("模板变量 %s 为空", path)
	}
	return current, nil
}

func stringifyTemplateValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case map[string]any, []any:
		return marshalTemplateValue(value)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func marshalTemplateValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func defaultCloseStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "resolved"
	}
	return status
}
