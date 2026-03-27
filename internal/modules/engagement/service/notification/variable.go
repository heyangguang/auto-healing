package notification

import (
	"time"

	"github.com/company/auto-healing/internal/model"
)

const (
	defaultSystemName    = "Auto-Healing"
	defaultSystemVersion = "1.0.0"
	defaultSystemEnv     = "production"
)

// VariableBuilder 变量构建器
// 从执行记录、任务等数据构建模板变量
type VariableBuilder struct {
	systemName    string
	systemURL     string
	systemVersion string
}

// NewVariableBuilder 创建变量构建器
func NewVariableBuilder(systemName, systemURL, systemVersion string) *VariableBuilder {
	if systemName == "" {
		systemName = defaultSystemName
	}
	if systemVersion == "" {
		systemVersion = defaultSystemVersion
	}
	return &VariableBuilder{
		systemName:    systemName,
		systemURL:     systemURL,
		systemVersion: systemVersion,
	}
}

// BuildFromExecution 从执行记录构建变量
func (b *VariableBuilder) BuildFromExecution(run *model.ExecutionRun, task *model.ExecutionTask) map[string]interface{} {
	now := time.Now()
	playbookVars, repositoryVars := b.buildPlaybookVariables(task)

	return map[string]interface{}{
		"timestamp":  now.Format(dateTimeLayout),
		"date":       now.Format(dateLayout),
		"time":       now.Format(clockLayout),
		"execution":  b.buildExecutionVariables(run),
		"task":       b.buildTaskVariables(task),
		"playbook":   playbookVars,
		"repository": repositoryVars,
		"stats":      b.parseStats(run.Stats),
		"error":      b.parseError(run),
		"system":     b.buildSystemVariables(),
	}
}

func (b *VariableBuilder) buildSystemVariables() map[string]interface{} {
	return map[string]interface{}{
		"name":    b.systemName,
		"url":     b.systemURL,
		"version": b.systemVersion,
		"env":     defaultSystemEnv,
	}
}
