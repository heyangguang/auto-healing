package playbook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// ScanVariables 深度扫描变量（完全递归 + 增强模式）
func (s *Service) ScanVariables(ctx context.Context, playbookID uuid.UUID, triggerType string) (*model.PlaybookScanLog, error) {
	playbook, gitRepo, err := s.loadPlaybookWithRepository(ctx, playbookID)
	if err != nil {
		return nil, err
	}

	scanner, err := s.scanPlaybookFiles(gitRepo.LocalPath, playbook.FilePath)
	if err != nil {
		return nil, err
	}

	enhancedVars := s.parseEnhancedConfig(gitRepo.LocalPath)
	scannedVars := buildScannedVariables(scanner, enhancedVars)
	mergedVars := s.mergeVariables(playbook.Variables, scannedVars)
	newCount, removedCount := s.countChanges(playbook.Variables, scannedVars)

	logEntry, err := s.persistScanOutcome(ctx, playbook, scanner, triggerType, scannedVars, mergedVars, newCount, removedCount)
	if err != nil {
		return nil, err
	}

	s.notifyRelatedTasks(ctx, playbookID, mergedVars)
	return logEntry, nil
}

func (s *Service) loadPlaybookWithRepository(ctx context.Context, playbookID uuid.UUID) (*model.Playbook, *model.GitRepository, error) {
	playbook, err := s.repo.GetByID(ctx, playbookID)
	if err != nil {
		return nil, nil, err
	}
	gitRepo, err := s.gitRepo.GetByID(ctx, playbook.RepositoryID)
	if err != nil {
		return nil, nil, err
	}
	return playbook, gitRepo, nil
}

func (s *Service) scanPlaybookFiles(repoPath, playbookPath string) (*VariableScanner, error) {
	scanner := &VariableScanner{
		basePath:     repoPath,
		scannedFiles: make(map[string]bool),
		variables:    make(map[string]*ScannedVariable),
	}
	fullPath := filepath.Join(repoPath, playbookPath)
	if err := scanner.ScanFile(fullPath); err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	return scanner, nil
}

func buildScannedVariables(scanner *VariableScanner, enhancedVars map[string]*EnhancedVariable) model.JSONArray {
	scannedVars := make(model.JSONArray, 0, len(scanner.variables)+len(enhancedVars))
	for _, variable := range scanner.variables {
		scannedVars = append(scannedVars, buildVariableData(variable, enhancedVars[variable.Name]))
	}
	for name, enhanced := range enhancedVars {
		if _, exists := scanner.variables[name]; !exists {
			scannedVars = append(scannedVars, buildEnhancedOnlyVariable(name, enhanced))
		}
	}
	return scannedVars
}

func buildVariableData(variable *ScannedVariable, enhanced *EnhancedVariable) map[string]any {
	data := map[string]any{
		"name":           variable.Name,
		"type":           variable.Type,
		"required":       variable.Required,
		"default":        variable.Default,
		"description":    variable.Description,
		"sources":        variable.Sources,
		"primary_source": variable.PrimarySource,
		"in_code":        true,
		"type_source":    "inferred",
	}
	if enhanced == nil {
		return data
	}

	applyEnhancedOverrides(data, enhanced)
	return data
}

func applyEnhancedOverrides(data map[string]any, enhanced *EnhancedVariable) {
	if enhanced.Type != "" {
		data["type"] = enhanced.Type
		data["type_source"] = "enhanced"
	}
	if enhanced.Description != "" {
		data["description"] = enhanced.Description
	}
	if enhanced.Default != nil {
		data["default"] = enhanced.Default
	}
	data["required"] = enhanced.Required
	if len(enhanced.Enum) > 0 {
		data["enum"] = enhanced.Enum
	}
	if enhanced.Min != nil {
		data["min"] = enhanced.Min
	}
	if enhanced.Max != nil {
		data["max"] = enhanced.Max
	}
	if enhanced.Pattern != "" {
		data["pattern"] = enhanced.Pattern
	}
}

func buildEnhancedOnlyVariable(name string, enhanced *EnhancedVariable) map[string]any {
	return map[string]any{
		"name":        name,
		"type":        enhanced.Type,
		"required":    enhanced.Required,
		"default":     enhanced.Default,
		"description": enhanced.Description,
		"enum":        enhanced.Enum,
		"min":         enhanced.Min,
		"max":         enhanced.Max,
		"pattern":     enhanced.Pattern,
		"source_file": ".auto-healing.yml",
		"source_line": 0,
		"in_code":     false,
	}
}

func (s *Service) persistScanOutcome(ctx context.Context, playbook *model.Playbook, scanner *VariableScanner, triggerType string, scannedVars, mergedVars model.JSONArray, newCount, removedCount int) (*model.PlaybookScanLog, error) {
	if err := s.repo.UpdateVariables(ctx, playbook.ID, mergedVars, scannedVars); err != nil {
		return nil, err
	}
	if playbook.Status == "pending" {
		s.repo.UpdateStatus(ctx, playbook.ID, "scanned")
	}

	logEntry := buildPlaybookScanLog(playbook.ID, triggerType, playbook.Variables, scannedVars, scanner, newCount, removedCount)
	s.repo.CreateScanLog(ctx, logEntry)
	logger.Sync_("PLAYBOOK").Info("%s | 文件: %d | 变量: %d | 新增: %d | 移除: %d",
		playbook.Name, logEntry.FilesScanned, logEntry.VariablesFound, logEntry.NewCount, logEntry.RemovedCount)
	return logEntry, nil
}

func buildPlaybookScanLog(playbookID uuid.UUID, triggerType string, oldVars, scannedVars model.JSONArray, scanner *VariableScanner, newCount, removedCount int) *model.PlaybookScanLog {
	return &model.PlaybookScanLog{
		PlaybookID:     playbookID,
		TriggerType:    triggerType,
		FilesScanned:   len(scanner.scannedFiles),
		VariablesFound: len(scanner.variables),
		NewCount:       newCount,
		RemovedCount:   removedCount,
		Details: model.JSON{
			"files":             getMapKeys(scanner.scannedFiles),
			"new_variables":     getNewVariableNames(oldVars, scannedVars),
			"removed_variables": getRemovedVariableNames(oldVars, scannedVars),
		},
	}
}

// UpdateUserVariables 更新用户变量配置
func (s *Service) UpdateUserVariables(ctx context.Context, playbookID uuid.UUID, variables model.JSONArray) error {
	playbook, err := s.repo.GetByID(ctx, playbookID)
	if err != nil {
		return err
	}

	playbook.Variables = variables
	if err := s.repo.Update(ctx, playbook); err != nil {
		return err
	}

	s.notifyRelatedTasks(ctx, playbookID, variables)
	return nil
}

// CheckPlaybooksAfterRepoSync 仓库同步后检查关联的 Playbooks
func (s *Service) CheckPlaybooksAfterRepoSync(ctx context.Context, repositoryID uuid.UUID) error {
	playbooks, err := s.repo.ListByRepositoryID(ctx, repositoryID)
	if err != nil {
		return err
	}
	gitRepo, err := s.gitRepo.GetByID(ctx, repositoryID)
	if err != nil {
		return err
	}

	for _, playbook := range playbooks {
		if s.markInvalidPlaybookIfMissing(ctx, gitRepo.LocalPath, playbook) {
			continue
		}
		if _, err := s.ScanVariables(ctx, playbook.ID, "repo_sync"); err != nil {
			logger.Sync_("PLAYBOOK").Warn("%s 变量扫描失败: %v", playbook.Name, err)
		}
	}
	return nil
}

func (s *Service) markInvalidPlaybookIfMissing(ctx context.Context, repoPath string, playbook model.Playbook) bool {
	fullPath := filepath.Join(repoPath, playbook.FilePath)
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		return false
	}

	s.repo.UpdateStatus(ctx, playbook.ID, "invalid")
	logger.Sync_("PLAYBOOK").Warn("%s 入口文件不存在，标记为 invalid", playbook.Name)
	return true
}
