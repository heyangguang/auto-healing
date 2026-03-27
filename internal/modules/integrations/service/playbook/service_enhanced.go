package playbook

import (
	"context"
	"os"
	"path/filepath"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// EnhancedVariable 增强模式变量定义
type EnhancedVariable struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Default     any      `yaml:"default"`
	Description string   `yaml:"description"`
	Enum        []string `yaml:"enum"`
	Min         *float64 `yaml:"min"`
	Max         *float64 `yaml:"max"`
	Pattern     string   `yaml:"pattern"`
}

// EnhancedConfig .auto-healing.yml 文件结构
type EnhancedConfig struct {
	Variables []EnhancedVariable `yaml:"variables"`
}

func (s *Service) parseEnhancedConfig(repoPath string) map[string]*EnhancedVariable {
	configPath := findEnhancedConfigPath(repoPath)
	if configPath == "" {
		return map[string]*EnhancedVariable{}
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		logger.Sync_("PLAYBOOK").Warn("读取增强配置文件失败: %v", err)
		return map[string]*EnhancedVariable{}
	}

	var config EnhancedConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		logger.Sync_("PLAYBOOK").Warn("解析增强配置文件失败: %v", err)
		return map[string]*EnhancedVariable{}
	}

	result := normalizeEnhancedVariables(config.Variables)
	logger.Sync_("PLAYBOOK").Info("从 %s 加载了 %d 个增强模式变量定义", filepath.Base(configPath), len(result))
	return result
}

func findEnhancedConfigPath(repoPath string) string {
	for _, path := range []string{
		filepath.Join(repoPath, ".auto-healing.yml"),
		filepath.Join(repoPath, ".auto-healing.yaml"),
		filepath.Join(repoPath, "playbook.meta.yml"),
		filepath.Join(repoPath, "playbook.meta.yaml"),
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func normalizeEnhancedVariables(variables []EnhancedVariable) map[string]*EnhancedVariable {
	result := make(map[string]*EnhancedVariable, len(variables))
	for i := range variables {
		variable := &variables[i]
		if variable.Name == "" {
			continue
		}
		if variable.Type == "" {
			variable.Type = "string"
		}
		result[variable.Name] = variable
	}
	return result
}

func (s *Service) notifyRelatedTasks(ctx context.Context, playbookID uuid.UUID, newVariables model.JSONArray) {
	tasks, err := s.executionRepo.ListTasksByPlaybookID(ctx, playbookID)
	if err != nil {
		logger.Sync_("PLAYBOOK").Warn("查询关联任务失败: %v", err)
		return
	}
	logger.Sync_("PLAYBOOK").Info("变量同步检查: Playbook %s 有 %d 个关联任务, %d 个变量", playbookID, len(tasks), len(newVariables))
	if len(tasks) == 0 {
		return
	}

	newVarMap := buildVariableTypeMap(newVariables)
	for _, task := range tasks {
		s.syncTaskReviewStatus(ctx, task, newVarMap)
	}
}

func buildVariableTypeMap(variables model.JSONArray) map[string]string {
	result := make(map[string]string, len(variables))
	for _, variable := range variables {
		vm, ok := variable.(map[string]any)
		if !ok {
			continue
		}
		name, ok := vm["name"].(string)
		if ok {
			varType, _ := vm["type"].(string)
			result[name] = varType
		}
	}
	return result
}

func (s *Service) syncTaskReviewStatus(ctx context.Context, task model.ExecutionTask, newVarMap map[string]string) {
	changedVars := s.detectChangedVariables(task.PlaybookVariablesSnapshot, newVarMap)
	logger.Sync_("PLAYBOOK").Info("任务 %s: 快照变量数=%d, Playbook变量数=%d, 检测到变更=%d",
		task.Name, len(task.PlaybookVariablesSnapshot), len(newVarMap), len(changedVars))

	switch {
	case len(changedVars) > 0:
		changedList := make(model.JSONArray, len(changedVars))
		for i, value := range changedVars {
			changedList[i] = value
		}
		if err := s.executionRepo.UpdateTaskReviewStatus(ctx, task.ID, true, changedList); err != nil {
			logger.Sync_("PLAYBOOK").Warn("更新任务 %s review 状态失败: %v", task.ID, err)
		} else {
			logger.Sync_("PLAYBOOK").Info("任务 %s 需要审核，变更字段: %v", task.Name, changedVars)
		}
	case !task.NeedsReview:
		logger.Sync_("PLAYBOOK").Info("任务 %s 无变更，跳过", task.Name)
	default:
		if err := s.executionRepo.UpdateTaskReviewStatus(ctx, task.ID, false, model.JSONArray{}); err != nil {
			logger.Sync_("PLAYBOOK").Warn("清除任务 %s review 状态失败: %v", task.ID, err)
		} else {
			logger.Sync_("PLAYBOOK").Info("任务 %s 变量已同步，清除审核状态", task.Name)
		}
	}
}

func (s *Service) detectChangedVariables(snapshot model.JSONArray, newVarMap map[string]string) []string {
	changed := make([]string, 0)
	snapshotVarMap := buildVariableTypeMap(snapshot)

	for name, oldType := range snapshotVarMap {
		if newType, exists := newVarMap[name]; !exists || oldType != newType {
			changed = append(changed, name)
		}
	}
	for name := range newVarMap {
		if _, exists := snapshotVarMap[name]; !exists {
			changed = append(changed, name)
		}
	}
	return changed
}
