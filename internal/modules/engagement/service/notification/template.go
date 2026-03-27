package notification

import (
	"fmt"
	"regexp"
	"strings"
)

// TemplateParser 模板解析器
// 支持 {{variable}} 和 {{variable.field}} 格式的变量替换
type TemplateParser struct{}

// NewTemplateParser 创建模板解析器
func NewTemplateParser() *TemplateParser {
	return &TemplateParser{}
}

// Parse 解析模板，替换变量
func (p *TemplateParser) Parse(template string, variables map[string]interface{}) (string, error) {
	// 匹配 {{xxx}} 或 {{xxx.yyy.zzz}} 格式
	re := regexp.MustCompile(`\{\{(\w+(?:\.\w+)*)\}\}`)

	result := re.ReplaceAllStringFunc(template, func(match string) string {
		// 提取变量路径，去掉 {{ 和 }}
		path := match[2 : len(match)-2]
		value := p.getValue(variables, path)
		if value != nil {
			return fmt.Sprintf("%v", value)
		}
		// 找不到变量保持原样
		return match
	})

	return result, nil
}

// ExtractVariables 提取模板中的变量名
func (p *TemplateParser) ExtractVariables(template string) []string {
	re := regexp.MustCompile(`\{\{(\w+(?:\.\w+)*)\}\}`)
	matches := re.FindAllStringSubmatch(template, -1)

	// 去重
	seen := make(map[string]bool)
	var variables []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			variables = append(variables, match[1])
		}
	}
	return variables
}

// ValidateVariables 验证所需变量是否都已提供
func (p *TemplateParser) ValidateVariables(template string, variables map[string]interface{}) ([]string, error) {
	required := p.ExtractVariables(template)
	var missing []string

	for _, varName := range required {
		if p.getValue(variables, varName) == nil {
			missing = append(missing, varName)
		}
	}

	if len(missing) > 0 {
		return missing, fmt.Errorf("缺少变量: %s", strings.Join(missing, ", "))
	}
	return nil, nil
}

// getValue 根据路径获取嵌套值
// 支持 "execution.status" 这样的嵌套路径
func (p *TemplateParser) getValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil
		}
	}
	return current
}

// GetAvailableVariables 返回所有可用变量的描述
func (p *TemplateParser) GetAvailableVariables() []VariableInfo {
	return []VariableInfo{
		// execution.*
		{Name: "execution.run_id", Description: "执行记录 ID", Category: "execution"},
		{Name: "execution.status", Description: "执行状态 (success/failed/timeout/cancelled)", Category: "execution"},
		{Name: "execution.status_emoji", Description: "状态表情 (✅/❌/⏱️/🚫)", Category: "execution"},
		{Name: "execution.exit_code", Description: "退出码", Category: "execution"},
		{Name: "execution.triggered_by", Description: "触发者", Category: "execution"},
		{Name: "execution.trigger_type", Description: "触发类型 (manual/scheduled/workflow)", Category: "execution"},
		{Name: "execution.started_at", Description: "开始时间", Category: "execution"},
		{Name: "execution.completed_at", Description: "完成时间", Category: "execution"},
		{Name: "execution.duration", Description: "执行时长 (如 2m 35s)", Category: "execution"},
		{Name: "execution.duration_seconds", Description: "执行时长(秒)", Category: "execution"},

		// task.*
		{Name: "task.id", Description: "任务模板 ID", Category: "task"},
		{Name: "task.name", Description: "任务名称", Category: "task"},
		{Name: "task.target_hosts", Description: "目标主机列表", Category: "task"},
		{Name: "task.host_count", Description: "主机数量", Category: "task"},
		{Name: "task.executor_type", Description: "执行器类型 (local/docker)", Category: "task"},

		// repository.*
		{Name: "repository.id", Description: "仓库 ID", Category: "repository"},
		{Name: "repository.name", Description: "仓库名称", Category: "repository"},
		{Name: "repository.url", Description: "仓库 URL", Category: "repository"},
		{Name: "repository.branch", Description: "分支", Category: "repository"},
		{Name: "repository.playbook", Description: "Playbook 文件", Category: "repository"},

		// stats.*
		{Name: "stats.ok", Description: "成功任务数", Category: "stats"},
		{Name: "stats.changed", Description: "变更任务数", Category: "stats"},
		{Name: "stats.failed", Description: "失败任务数", Category: "stats"},
		{Name: "stats.unreachable", Description: "不可达主机数", Category: "stats"},
		{Name: "stats.skipped", Description: "跳过任务数", Category: "stats"},

		// error.*
		{Name: "error.message", Description: "错误信息", Category: "error"},
		{Name: "error.host", Description: "出错主机", Category: "error"},

		// system.*
		{Name: "system.name", Description: "系统名称", Category: "system"},
		{Name: "system.url", Description: "系统 URL", Category: "system"},
		{Name: "system.version", Description: "系统版本", Category: "system"},
		{Name: "timestamp", Description: "当前时间 (YYYY-MM-DD HH:MM:SS)", Category: "system"},
		{Name: "date", Description: "当前日期 (YYYY-MM-DD)", Category: "system"},
		{Name: "time", Description: "当前时间 (HH:MM:SS)", Category: "system"},
	}
}

// VariableInfo 变量信息
type VariableInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
