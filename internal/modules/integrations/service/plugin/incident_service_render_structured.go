package plugin

import (
	"fmt"
	"strings"

	integrationmodel "github.com/company/auto-healing/internal/modules/integrations/model"
)

type solutionRenderSection struct {
	Title   string
	Content string
}

func renderStructuredSolutionTemplate(template *integrationmodel.IncidentSolutionTemplate, context map[string]any) (*renderedSolution, error) {
	problem, err := renderOptionalTemplate(template.ProblemTemplate, context)
	if err != nil {
		return nil, fmt.Errorf("渲染 problem_template 失败: %w", err)
	}
	solution, err := renderTemplate(template.SolutionTemplate, context)
	if err != nil {
		return nil, fmt.Errorf("渲染 solution_template 失败: %w", err)
	}
	verification, err := renderOptionalTemplate(template.VerificationTemplate, context)
	if err != nil {
		return nil, fmt.Errorf("渲染 verification_template 失败: %w", err)
	}
	conclusion, err := renderTemplate(template.ConclusionTemplate, context)
	if err != nil {
		return nil, fmt.Errorf("渲染 conclusion_template 失败: %w", err)
	}

	sections := make([]solutionRenderSection, 0, 4)
	appendSolutionSection(&sections, "问题说明", problem)
	appendSolutionSection(&sections, "解决方案", solution)
	appendSolutionSection(&sections, "执行步骤", renderStepsText(context, template))
	appendSolutionSection(&sections, "验证结果", verification)

	return &renderedSolution{
		Resolution:  conclusion,
		WorkNotes:   composeSolutionSections(sections),
		CloseCode:   strings.TrimSpace(template.DefaultCloseCode),
		CloseStatus: defaultCloseStatus(template.DefaultCloseStatus),
	}, nil
}

func renderOptionalTemplate(template string, context map[string]any) (string, error) {
	template = strings.TrimSpace(template)
	if template == "" {
		return "", nil
	}
	return renderTemplate(template, context)
}

func appendSolutionSection(sections *[]solutionRenderSection, title, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	*sections = append(*sections, solutionRenderSection{Title: title, Content: content})
}

func composeSolutionSections(sections []solutionRenderSection) string {
	if len(sections) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, fmt.Sprintf("%s：\n%s", section.Title, section.Content))
	}
	return strings.Join(parts, "\n\n")
}

func renderStepsText(context map[string]any, template *integrationmodel.IncidentSolutionTemplate) string {
	steps := templateContextSteps(context)
	if len(steps) == 0 {
		return ""
	}
	maxCount := template.StepsMaxCount
	if maxCount <= 0 {
		maxCount = 6
	}
	if len(steps) > maxCount {
		steps = steps[:maxCount]
	}

	mode := strings.TrimSpace(template.StepsRenderMode)
	if mode == "" {
		mode = "summary"
	}
	lines := make([]string, 0, len(steps))
	for index, step := range steps {
		line := fmt.Sprintf("%d. %s", index+1, stepSummary(step))
		if mode == "detailed" {
			if detail := stepDetail(step, template.StepOutputMaxLength); detail != "" {
				line += "\n" + indentLines(detail, "   ")
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func templateContextSteps(context map[string]any) []map[string]any {
	rawSteps, exists := context["steps"]
	if !exists || rawSteps == nil {
		return nil
	}
	switch typed := rawSteps.(type) {
	case []map[string]any:
		return typed
	case []any:
		steps := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			step, ok := item.(map[string]any)
			if ok {
				steps = append(steps, step)
			}
		}
		return steps
	default:
		return nil
	}
}

func stepSummary(step map[string]any) string {
	title := stringifyTemplateValue(step["title"])
	summary := stringifyTemplateValue(step["summary"])
	status := stringifyTemplateValue(step["status"])
	switch {
	case title != "" && summary != "" && title != summary:
		return fmt.Sprintf("%s：%s（%s）", title, summary, status)
	case summary != "":
		return fmt.Sprintf("%s（%s）", summary, status)
	case title != "":
		return fmt.Sprintf("%s（%s）", title, status)
	default:
		return fmt.Sprintf("步骤执行（%s）", status)
	}
}

func stepDetail(step map[string]any, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 240
	}
	for _, key := range []string{"detail", "output_excerpt"} {
		if step[key] == nil {
			continue
		}
		text := strings.TrimSpace(stringifyTemplateValue(step[key]))
		if text == "" || text == "<nil>" {
			continue
		}
		return normalizeRenderedDetail(text, maxLen)
	}
	return ""
}

func normalizeRenderedDetail(text string, maxLen int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "<nil>" {
			continue
		}
		if len(line) > maxLen {
			line = line[:maxLen] + "..."
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

func indentLines(text, prefix string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	indented := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		indented = append(indented, prefix+line)
	}
	return strings.Join(indented, "\n")
}
