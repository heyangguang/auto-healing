package healing

import (
	"path/filepath"
	"regexp"
	"strings"
)

var ansibleTaskLinePattern = regexp.MustCompile(`^TASK \[(.+?)\] \*+$`)

type ansibleTaskResult struct {
	Name   string
	Status string
	Detail string
}

func autoCloseExecutionTaskDetail(stdout string) string {
	tasks := parseAnsibleTaskResults(stdout)
	if len(tasks) == 0 {
		return strings.TrimSpace(stdout)
	}

	lines := make([]string, 0, len(tasks))
	for _, task := range tasks {
		lines = append(lines, formatAnsibleTaskResult(task))
	}
	return strings.Join(lines, "\n")
}

func parseAnsibleTaskResults(stdout string) []ansibleTaskResult {
	lines := strings.Split(stdout, "\n")
	results := make([]ansibleTaskResult, 0, len(lines)/3)
	current := ansibleTaskResult{}
	extras := make([]string, 0, 2)

	flush := func() {
		task, ok := finalizeAnsibleTask(current, extras)
		if ok {
			results = append(results, task)
		}
		current = ansibleTaskResult{}
		extras = extras[:0]
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "PLAY RECAP") {
			flush()
			break
		}
		if name := parseAnsibleTaskName(line); name != "" {
			flush()
			current.Name = name
			continue
		}
		if current.Name == "" {
			continue
		}
		if status, extra, ok := parseAnsibleTaskStatus(line); ok {
			current.Status = status
			if extra != "" {
				extras = append(extras, extra)
			}
			continue
		}
		extras = append(extras, line)
	}
	flush()
	return results
}

func parseAnsibleTaskName(line string) string {
	matches := ansibleTaskLinePattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func parseAnsibleTaskStatus(line string) (string, string, bool) {
	for _, prefix := range []string{"ok:", "changed:", "skipping:", "fatal:", "failed:"} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSuffix(prefix, ":"), "", true
		}
	}
	if strings.HasPrefix(line, "included: ") {
		return "included", parseIncludedTask(line), true
	}
	return "", "", false
}

func finalizeAnsibleTask(task ansibleTaskResult, extras []string) (ansibleTaskResult, bool) {
	task.Name = strings.TrimSpace(task.Name)
	if task.Name == "" {
		return ansibleTaskResult{}, false
	}
	if task.Status == "" {
		return ansibleTaskResult{}, false
	}
	task.Detail = parseAnsibleTaskDetail(extras)
	return task, true
}

func parseIncludedTask(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "included: "))
	if before, _, found := strings.Cut(line, " for "); found {
		line = before
	}
	return filepath.Base(strings.TrimSpace(line))
}

func parseAnsibleTaskDetail(lines []string) string {
	candidates := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		if detail := parseAnsibleTaskDetailLine(rawLine); detail != "" {
			candidates = append(candidates, detail)
		}
	}
	return strings.Join(candidates, "；")
}

func parseAnsibleTaskDetailLine(line string) string {
	line = strings.TrimSpace(line)
	switch {
	case line == "", line == "{", line == "}":
		return ""
	case strings.HasPrefix(line, "\"msg\":"):
		return parseJSONFieldValue(line)
	case strings.HasPrefix(line, "\"stdout\":"):
		return parseJSONFieldValue(line)
	case strings.HasPrefix(line, "\"stderr\":"):
		return parseJSONFieldValue(line)
	default:
		if strings.Contains(line, ":") {
			return ""
		}
		return line
	}
}

func parseJSONFieldValue(line string) string {
	_, value, found := strings.Cut(line, ":")
	if !found {
		return ""
	}
	value = strings.TrimSpace(strings.TrimSuffix(value, ","))
	value = strings.Trim(value, `"`)
	return strings.TrimSpace(value)
}

func formatAnsibleTaskResult(task ansibleTaskResult) string {
	line := "- " + task.Name + "：" + ansibleStatusText(task.Status)
	if strings.TrimSpace(task.Detail) != "" {
		line += "（" + task.Detail + "）"
	}
	return line
}

func ansibleStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case "changed":
		return "已变更"
	case "skipping":
		return "已跳过"
	case "fatal", "failed":
		return "失败"
	case "included":
		return "已加载"
	default:
		return "成功"
	}
}
