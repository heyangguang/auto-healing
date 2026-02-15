package playbook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Service Playbook 服务
type Service struct {
	repo          *repository.PlaybookRepository
	gitRepo       *repository.GitRepositoryRepository
	executionRepo *repository.ExecutionRepository
}

// NewService 创建 Playbook 服务
func NewService() *Service {
	return &Service{
		repo:          repository.NewPlaybookRepository(),
		gitRepo:       repository.NewGitRepositoryRepository(),
		executionRepo: repository.NewExecutionRepository(),
	}
}

// ==================== CRUD ====================

// Create 创建 Playbook
func (s *Service) Create(ctx context.Context, repositoryID uuid.UUID, name, filePath, description, configMode string) (*model.Playbook, error) {
	// 验证仓库存在
	gitRepo, err := s.gitRepo.GetByID(ctx, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("仓库不存在: %w", err)
	}

	// 仓库必须已同步（状态为 synced 或 ready）
	if gitRepo.Status != "synced" && gitRepo.Status != "ready" {
		return nil, fmt.Errorf("仓库未同步，请先同步仓库")
	}

	// 验证入口文件存在
	fullPath := filepath.Join(gitRepo.LocalPath, filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("入口文件不存在: %s", filePath)
	}

	// 验证 configMode
	if configMode != "auto" && configMode != "enhanced" {
		return nil, fmt.Errorf("无效的扫描模式，必须为 auto 或 enhanced")
	}

	playbook := &model.Playbook{
		RepositoryID: repositoryID,
		Name:         name,
		Description:  description,
		FilePath:     filePath,
		ConfigMode:   configMode,
		Status:       "pending", // 创建后等待扫描
		Variables:    model.JSONArray{},
	}

	if err := s.repo.Create(ctx, playbook); err != nil {
		return nil, err
	}

	playbook.Repository = gitRepo
	return playbook, nil
}

// Get 获取 Playbook
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Playbook, error) {
	return s.repo.GetByID(ctx, id)
}

// List 列出 Playbooks（向后兼容）
func (s *Service) List(ctx context.Context, repositoryID *uuid.UUID, status string, page, pageSize int) ([]model.Playbook, int64, error) {
	return s.repo.List(ctx, repositoryID, status, page, pageSize)
}

// ListWithOptions 列出 Playbooks（支持完整查询参数）
func (s *Service) ListWithOptions(ctx context.Context, opts *repository.PlaybookListOptions) ([]model.Playbook, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 500 {
		opts.PageSize = 20
	}
	return s.repo.ListWithOptions(ctx, opts)
}

// Update 更新 Playbook
func (s *Service) Update(ctx context.Context, id uuid.UUID, name, description string) error {
	playbook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	playbook.Name = name
	playbook.Description = description
	return s.repo.Update(ctx, playbook)
}

// Delete 删除 Playbook（保护性删除）
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有关联的任务模板
	taskCount, err := s.executionRepo.CountTasksByPlaybookID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("无法删除：该 Playbook 下有 %d 个任务模板，请先删除关联的任务模板", taskCount)
	}

	return s.repo.Delete(ctx, id)
}

// SetReady 设置 Playbook 为 ready 状态（上线）
func (s *Service) SetReady(ctx context.Context, id uuid.UUID) error {
	playbook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 只有 scanned 或 outdated 状态可以上线
	if playbook.Status == "pending" {
		return fmt.Errorf("Playbook 未扫描，请先扫描变量")
	}
	if playbook.Status == "invalid" {
		return fmt.Errorf("Playbook 入口文件不存在，无法上线")
	}
	if playbook.Status == "ready" {
		return fmt.Errorf("Playbook 已经是 ready 状态")
	}

	return s.repo.UpdateStatus(ctx, id, "ready")
}

// SetOffline 设置 Playbook 为 scanned 状态（下线）
func (s *Service) SetOffline(ctx context.Context, id uuid.UUID) error {
	playbook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if playbook.Status != "ready" {
		return fmt.Errorf("只有 ready 状态的 Playbook 可以下线")
	}

	return s.repo.UpdateStatus(ctx, id, "scanned")
}

// ScannedFile 扫描过的文件信息
type ScannedFile struct {
	Path string `json:"path"`
	Type string `json:"type"` // entry, task, vars, defaults, handlers, role, include
}

// GetFiles 获取 Playbook 扫描过的文件列表
func (s *Service) GetFiles(ctx context.Context, id uuid.UUID) ([]ScannedFile, error) {
	playbook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 从最新的扫描日志中获取文件列表
	logs, _, err := s.repo.ListScanLogs(ctx, id, 1, 1)
	if err != nil || len(logs) == 0 {
		// 没有扫描记录，只返回入口文件
		return []ScannedFile{{Path: playbook.FilePath, Type: "entry"}}, nil
	}

	// 从扫描日志的 details.files 中提取
	latestLog := logs[0]
	fileMap := make(map[string]string) // path -> type

	// 添加入口文件
	fileMap[playbook.FilePath] = "entry"

	if latestLog.Details != nil {
		if files, ok := latestLog.Details["files"].([]interface{}); ok {
			for _, f := range files {
				if filePath, ok := f.(string); ok {
					// 提取相对路径
					relPath := filePath
					if idx := strings.Index(filePath, playbook.FilePath); idx > 0 {
						// 找到仓库根目录
						repoPath := filePath[:idx]
						relPath = strings.TrimPrefix(filePath, repoPath)
					} else {
						// 尝试提取 roles/ 或其他相对路径
						parts := strings.Split(filePath, "/repos/")
						if len(parts) > 1 {
							subParts := strings.SplitN(parts[1], "/", 2)
							if len(subParts) > 1 {
								relPath = subParts[1]
							}
						}
					}

					if relPath != "" && relPath != playbook.FilePath {
						fileType := inferFileType(relPath)
						fileMap[relPath] = fileType
					}
				}
			}
		}
	}

	// 转换为切片
	var files []ScannedFile
	for path, typ := range fileMap {
		files = append(files, ScannedFile{Path: path, Type: typ})
	}

	return files, nil
}

// inferFileType 根据文件路径推断类型
func inferFileType(path string) string {
	if strings.Contains(path, "/tasks/") {
		return "task"
	}
	if strings.Contains(path, "/vars/") {
		return "vars"
	}
	if strings.Contains(path, "/defaults/") {
		return "defaults"
	}
	if strings.Contains(path, "/handlers/") {
		return "handlers"
	}
	if strings.Contains(path, "/templates/") {
		return "template"
	}
	if strings.Contains(path, "/files/") {
		return "file"
	}
	if strings.Contains(path, "roles/") {
		return "role"
	}
	return "include"
}

// ==================== 变量扫描 ====================

// ScanVariables 深度扫描变量（完全递归 + 增强模式）
func (s *Service) ScanVariables(ctx context.Context, playbookID uuid.UUID, triggerType string) (*model.PlaybookScanLog, error) {
	playbook, err := s.repo.GetByID(ctx, playbookID)
	if err != nil {
		return nil, err
	}

	gitRepo, err := s.gitRepo.GetByID(ctx, playbook.RepositoryID)
	if err != nil {
		return nil, err
	}

	// 1. 检查增强模式：解析 .auto-healing.yml 文件
	enhancedVars := s.parseEnhancedConfig(gitRepo.LocalPath)

	// 2. 完全递归扫描代码中的变量
	scanner := &VariableScanner{
		basePath:     gitRepo.LocalPath,
		scannedFiles: make(map[string]bool),
		variables:    make(map[string]*ScannedVariable),
	}

	fullPath := filepath.Join(gitRepo.LocalPath, playbook.FilePath)
	if err := scanner.ScanFile(fullPath); err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}

	// 3. 合并：增强模式配置优先
	scannedVars := make(model.JSONArray, 0, len(scanner.variables))
	for _, v := range scanner.variables {
		varData := map[string]any{
			"name":           v.Name,
			"type":           v.Type,
			"required":       v.Required,
			"default":        v.Default,
			"description":    v.Description,
			"sources":        v.Sources,
			"primary_source": v.PrimarySource,
			"in_code":        true,
		}

		// 如果增强模式中有此变量的定义，使用增强模式的配置
		if enhanced, ok := enhancedVars[v.Name]; ok {
			if enhanced.Type != "" {
				varData["type"] = enhanced.Type
			}
			if enhanced.Description != "" {
				varData["description"] = enhanced.Description
			}
			if enhanced.Default != nil {
				varData["default"] = enhanced.Default
			}
			varData["required"] = enhanced.Required
			if len(enhanced.Enum) > 0 {
				varData["enum"] = enhanced.Enum
			}
			if enhanced.Min != nil {
				varData["min"] = enhanced.Min
			}
			if enhanced.Max != nil {
				varData["max"] = enhanced.Max
			}
			if enhanced.Pattern != "" {
				varData["pattern"] = enhanced.Pattern
			}
		}

		scannedVars = append(scannedVars, varData)
	}

	// 4. 添加增强模式中定义但代码中未发现的变量
	for name, enhanced := range enhancedVars {
		if _, exists := scanner.variables[name]; !exists {
			scannedVars = append(scannedVars, map[string]any{
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
			})
		}
	}

	// 合并用户配置
	mergedVars := s.mergeVariables(playbook.Variables, scannedVars)

	// 计算变更
	newCount, removedCount := s.countChanges(playbook.Variables, scannedVars)

	// 更新 Playbook
	if err := s.repo.UpdateVariables(ctx, playbookID, mergedVars, scannedVars); err != nil {
		return nil, err
	}

	// 更新状态
	if playbook.Status == "pending" {
		// 首次扫描成功，更新为 scanned（待上线）
		s.repo.UpdateStatus(ctx, playbookID, "scanned")
	}
	// ready 状态重新扫描后保持 ready（不需要重新上线）

	// 创建扫描日志
	log := &model.PlaybookScanLog{
		PlaybookID:     playbookID,
		TriggerType:    triggerType,
		FilesScanned:   len(scanner.scannedFiles),
		VariablesFound: len(scanner.variables),
		NewCount:       newCount,
		RemovedCount:   removedCount,
		Details: model.JSON{
			"files":             getMapKeys(scanner.scannedFiles),
			"new_variables":     getNewVariableNames(playbook.Variables, scannedVars),
			"removed_variables": getRemovedVariableNames(playbook.Variables, scannedVars),
		},
	}
	s.repo.CreateScanLog(ctx, log)

	logger.Sync_("PLAYBOOK").Info("%s | 文件: %d | 变量: %d | 新增: %d | 移除: %d",
		playbook.Name, log.FilesScanned, log.VariablesFound, log.NewCount, log.RemovedCount)

	// 检测并更新关联任务模板的 review 状态
	s.notifyRelatedTasks(ctx, playbookID, mergedVars)

	return log, nil
}

// mergeVariables 合并变量（保留用户配置）
func (s *Service) mergeVariables(userVars, scannedVars model.JSONArray) model.JSONArray {
	result := make(model.JSONArray, 0)
	userVarMap := make(map[string]map[string]any)

	// 建立用户配置索引
	for _, v := range userVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				userVarMap[name] = vm
			}
		}
	}

	scannedNameSet := make(map[string]bool)
	for _, v := range scannedVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				scannedNameSet[name] = true
			}
		}
	}

	// 处理扫描到的变量
	for _, v := range scannedVars {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name, _ := vm["name"].(string)

		// 如果用户有配置，保留用户的配置但更新来源信息
		if userVar, exists := userVarMap[name]; exists {
			merged := make(map[string]any)
			// 复制用户配置
			for k, val := range userVar {
				merged[k] = val
			}
			// 更新来源信息
			merged["sources"] = vm["sources"]
			merged["primary_source"] = vm["primary_source"]
			merged["in_code"] = true

			// 如果新扫描的类型更精确（通过 Jinja2 default 推断），则更新类型
			// 规则：如果新扫描的类型不是 string（说明是从 Jinja2 default 或直接值推断的），则采用新类型
			if newType, ok := vm["type"].(string); ok && newType != "string" {
				oldType, _ := merged["type"].(string)
				if oldType != newType {
					merged["type"] = newType
					// 如果新扫描有默认值，也更新
					if newDefault := vm["default"]; newDefault != nil {
						merged["default"] = newDefault
					}
				}
			}

			result = append(result, merged)
		} else {
			// 新变量
			vm["in_code"] = true
			result = append(result, vm)
		}
	}

	// 保留用户配置但代码中不存在的变量
	for name, userVar := range userVarMap {
		if !scannedNameSet[name] {
			userVar["in_code"] = false
			result = append(result, userVar)
		}
	}

	return result
}

// countChanges 计算变更数量
func (s *Service) countChanges(oldVars, newVars model.JSONArray) (newCount, removedCount int) {
	oldNames := make(map[string]bool)
	newNames := make(map[string]bool)

	for _, v := range oldVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				oldNames[name] = true
			}
		}
	}

	for _, v := range newVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				newNames[name] = true
			}
		}
	}

	for name := range newNames {
		if !oldNames[name] {
			newCount++
		}
	}

	for name := range oldNames {
		if !newNames[name] {
			removedCount++
		}
	}

	return
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

	// 检测并更新关联任务模板的 review 状态
	s.notifyRelatedTasks(ctx, playbookID, variables)

	return nil
}

// GetScanLogs 获取扫描日志
func (s *Service) GetScanLogs(ctx context.Context, playbookID uuid.UUID, page, pageSize int) ([]model.PlaybookScanLog, int64, error) {
	return s.repo.ListScanLogs(ctx, playbookID, page, pageSize)
}

// ==================== 仓库同步后检查 ====================

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
		// 检查入口文件是否存在
		fullPath := filepath.Join(gitRepo.LocalPath, playbook.FilePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// 入口文件不存在，标记为 invalid
			s.repo.UpdateStatus(ctx, playbook.ID, "invalid")
			logger.Sync_("PLAYBOOK").Warn("%s 入口文件不存在，标记为 invalid", playbook.Name)
			continue
		}

		// 重新扫描变量
		if _, err := s.ScanVariables(ctx, playbook.ID, "repo_sync"); err != nil {
			logger.Sync_("PLAYBOOK").Warn("%s 变量扫描失败: %v", playbook.Name, err)
		}
	}

	return nil
}

// CanDeleteRepository 检查仓库是否可以删除
func (s *Service) CanDeleteRepository(ctx context.Context, repositoryID uuid.UUID) (bool, int64, error) {
	count, err := s.repo.CountByRepositoryID(ctx, repositoryID)
	if err != nil {
		return false, 0, err
	}
	return count == 0, count, nil
}

// ==================== 辅助函数 ====================

func getMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func getNewVariableNames(oldVars, newVars model.JSONArray) []string {
	oldNames := make(map[string]bool)
	for _, v := range oldVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				oldNames[name] = true
			}
		}
	}

	var result []string
	for _, v := range newVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				if !oldNames[name] {
					result = append(result, name)
				}
			}
		}
	}
	return result
}

func getRemovedVariableNames(oldVars, newVars model.JSONArray) []string {
	newNames := make(map[string]bool)
	for _, v := range newVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				newNames[name] = true
			}
		}
	}

	var result []string
	for _, v := range oldVars {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				if !newNames[name] {
					result = append(result, name)
				}
			}
		}
	}
	return result
}

// ==================== 变量扫描器 ====================

// ScannedVariable 扫描到的变量
type ScannedVariable struct {
	Name          string
	Type          string
	Required      bool
	Default       any
	Description   string
	Sources       []VariableSource // 所有来源位置
	PrimarySource string           // 主来源（类型推断来源）
	HasDefault    bool             // 是否有 Jinja2 default 表达式
}

// VariableSource 变量来源位置
type VariableSource struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// VariableScanner 变量扫描器（完全递归）
type VariableScanner struct {
	basePath     string
	scannedFiles map[string]bool
	variables    map[string]*ScannedVariable
}

// ScanFile 扫描文件（递归）
func (vs *VariableScanner) ScanFile(filePath string) error {
	// 避免重复扫描
	absPath, _ := filepath.Abs(filePath)
	if vs.scannedFiles[absPath] {
		return nil
	}
	vs.scannedFiles[absPath] = true

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 解析 YAML
	var data interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		// 如果不是有效 YAML，只扫描变量引用
		vs.scanVariableReferences(string(content), filePath)
		return nil
	}

	// 扫描 YAML 结构中的变量
	vs.scanYAMLStructure(data, filePath)

	// 扫描变量引用
	vs.scanVariableReferences(string(content), filePath)

	// 递归扫描引用的文件
	vs.scanIncludes(data, filePath)

	return nil
}

// scanYAMLStructure 扫描 YAML 结构
func (vs *VariableScanner) scanYAMLStructure(data interface{}, filePath string) {
	relPath, _ := filepath.Rel(vs.basePath, filePath)

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			vs.scanYAMLStructure(item, filePath)
		}
	case map[string]interface{}:
		// 检查 vars 字段
		if vars, ok := v["vars"].(map[string]interface{}); ok {
			for name, value := range vars {
				vs.addVariable(name, value, relPath, 0)
			}
		}

		// 检查 set_fact
		if setFact, ok := v["set_fact"].(map[string]interface{}); ok {
			for name, value := range setFact {
				vs.addVariable(name, value, relPath, 0)
			}
		}

		// 递归检查其他字段
		for _, value := range v {
			vs.scanYAMLStructure(value, filePath)
		}
	}
}

// scanVariableReferences 扫描变量引用（{{ var }}）
func (vs *VariableScanner) scanVariableReferences(content string, filePath string) {
	relPath, _ := filepath.Rel(vs.basePath, filePath)

	// 匹配完整的 {{ ... }} 表达式
	re := regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)(\s*\|[^}]*)?\s*\}\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			// 排除 Ansible 内置变量
			if !isBuiltinVariable(varName) {
				// 如果有 filter（如 | default(...)），传递完整表达式用于类型推断
				var defaultValue any
				if len(match) >= 3 && match[2] != "" {
					// 完整表达式（用于解析 default 值）
					defaultValue = "{{" + varName + match[2] + "}}"
				}
				vs.addVariable(varName, defaultValue, relPath, 0)
			}
		}
	}
}

// scanIncludes 扫描 include/import 引用（递归）
func (vs *VariableScanner) scanIncludes(data interface{}, currentFile string) {
	currentDir := filepath.Dir(currentFile)

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			vs.scanIncludes(item, currentFile)
		}
	case map[string]interface{}:
		// include_tasks / import_tasks
		for _, key := range []string{"include_tasks", "import_tasks", "include", "import_playbook"} {
			if path, ok := v[key].(string); ok {
				includePath := filepath.Join(currentDir, path)
				if _, err := os.Stat(includePath); err == nil {
					vs.ScanFile(includePath)
				}
			}
		}

		// roles
		if roles, ok := v["roles"].([]interface{}); ok {
			for _, role := range roles {
				vs.scanRole(role, currentFile)
			}
		}

		// vars_files - 支持动态路径
		if varsFiles, ok := v["vars_files"].([]interface{}); ok {
			for _, vf := range varsFiles {
				if path, ok := vf.(string); ok {
					vs.scanVarsFile(path, currentFile)
				}
			}
		}

		// template 模块 - 扫描 .j2 文件
		if template, ok := v["template"].(map[string]interface{}); ok {
			if src, ok := template["src"].(string); ok {
				vs.scanTemplateFile(src, currentFile)
			}
		}
		// 简写形式 template: xxx.j2
		if src, ok := v["template"].(string); ok {
			vs.scanTemplateFile(src, currentFile)
		}

		// 递归处理其他字段
		for _, value := range v {
			vs.scanIncludes(value, currentFile)
		}
	}
}

// scanRole 扫描 role（扫描所有 Ansible 官方标准目录）
func (vs *VariableScanner) scanRole(role interface{}, currentFile string) {
	var roleName string

	switch r := role.(type) {
	case string:
		roleName = r
	case map[string]interface{}:
		if name, ok := r["role"].(string); ok {
			roleName = name
		} else if name, ok := r["name"].(string); ok {
			roleName = name
		}
	}

	if roleName == "" {
		return
	}

	// Ansible 官方标准目录
	roleDirs := []string{
		"tasks",
		"handlers",
		"vars",
		"defaults",
		"files",
		"templates",
		"meta",
	}

	roleBase := filepath.Join(vs.basePath, "roles", roleName)

	for _, dir := range roleDirs {
		dirPath := filepath.Join(roleBase, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			// 扫描目录中的所有 .yml/.yaml/.j2 文件
			entries, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
					vs.ScanFile(filepath.Join(dirPath, name))
				} else if strings.HasSuffix(name, ".j2") {
					vs.scanJinja2File(filepath.Join(dirPath, name))
				}
			}
		}
	}
}

// scanVarsFile 扫描变量文件（支持动态路径）
func (vs *VariableScanner) scanVarsFile(path string, currentFile string) {
	currentDir := filepath.Dir(currentFile)

	// 检查是否包含 Jinja2 变量（动态路径）
	if strings.Contains(path, "{{") && strings.Contains(path, "}}") {
		// 动态路径：提取目录部分，扫描该目录下所有 .yml 文件
		// 例如：vars/{{ env }}.yml -> 扫描 vars/ 下所有 .yml
		dirPart := filepath.Dir(path)
		varsDir := filepath.Join(currentDir, dirPart)

		if info, err := os.Stat(varsDir); err == nil && info.IsDir() {
			entries, err := os.ReadDir(varsDir)
			if err == nil {
				for _, entry := range entries {
					if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml")) {
						fullPath := filepath.Join(varsDir, entry.Name())
						vs.ScanFile(fullPath)
					}
				}
			}
		}
	} else {
		// 静态路径：直接扫描
		varsPath := filepath.Join(currentDir, path)
		if _, err := os.Stat(varsPath); err == nil {
			vs.ScanFile(varsPath)
		}
	}
}

// scanTemplateFile 扫描 Jinja2 模板文件中的变量
func (vs *VariableScanner) scanTemplateFile(src string, currentFile string) {
	currentDir := filepath.Dir(currentFile)

	// 查找模板文件的可能路径
	searchPaths := []string{
		filepath.Join(currentDir, src),
		filepath.Join(currentDir, "templates", src),
		filepath.Join(vs.basePath, "templates", src),
	}

	// 如果在 role 中，也检查 role 的 templates 目录
	if strings.Contains(currentFile, "/roles/") {
		parts := strings.Split(currentFile, "/roles/")
		if len(parts) > 1 {
			roleParts := strings.SplitN(parts[1], "/", 2)
			if len(roleParts) > 0 {
				rolePath := filepath.Join(parts[0], "roles", roleParts[0], "templates", src)
				searchPaths = append(searchPaths, rolePath)
			}
		}
	}

	for _, templatePath := range searchPaths {
		if _, err := os.Stat(templatePath); err == nil {
			vs.scanJinja2File(templatePath)
			return
		}
	}
}

// scanJinja2File 扫描 .j2 文件中的变量引用
func (vs *VariableScanner) scanJinja2File(filePath string) {
	// 避免重复扫描
	absPath, _ := filepath.Abs(filePath)
	if vs.scannedFiles[absPath] {
		return
	}
	vs.scannedFiles[absPath] = true

	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	// 扫描 Jinja2 变量引用
	vs.scanVariableReferences(string(content), filePath)
}

// addVariable 添加变量
func (vs *VariableScanner) addVariable(name string, defaultValue any, sourceFile string, sourceLine int) {
	if existing, exists := vs.variables[name]; exists {
		// 变量已存在，添加新来源
		existing.Sources = append(existing.Sources, VariableSource{File: sourceFile, Line: sourceLine})

		// 检查是否需要更新类型
		// 如果当前有 Jinja2 default 值，且现有类型是 string 或基于变量名推断的，则更新类型
		if defaultValue != nil {
			if strVal, ok := defaultValue.(string); ok {
				if newType := parseJinja2Default(strVal); newType != "" && existing.Type != newType {
					// Jinja2 default 推断出了更好的类型，更新类型和主来源
					existing.Type = newType
					existing.Default = defaultValue
					existing.PrimarySource = sourceFile
					existing.HasDefault = true
				}
			}
		}
		return
	}

	// 检查是否有 Jinja2 default 表达式
	hasDefault := false
	if strVal, ok := defaultValue.(string); ok {
		if parseJinja2Default(strVal) != "" {
			hasDefault = true
		}
	}

	vs.variables[name] = &ScannedVariable{
		Name:          name,
		Type:          inferTypeSmartly(name, defaultValue),
		Default:       defaultValue,
		Sources:       []VariableSource{{File: sourceFile, Line: sourceLine}},
		PrimarySource: sourceFile,
		HasDefault:    hasDefault,
	}
}

// inferTypeSmartly 智能推断类型
// 优先级：1. 直接值类型 2. Jinja2 default 表达式 3. 变量名启发式 4. 默认 string
func inferTypeSmartly(name string, value any) string {
	// 1. 直接值类型判断
	if value != nil {
		switch v := value.(type) {
		case bool:
			return "boolean"
		case int, int64, float64:
			return "number"
		case []interface{}:
			return "list"
		case map[string]interface{}:
			return "object"
		case string:
			// 2. 解析 Jinja2 default 表达式
			if inferredType := parseJinja2Default(v); inferredType != "" {
				return inferredType
			}
		}
	}

	// 3. 基于变量名启发式推断
	if inferredType := inferTypeByName(name); inferredType != "" {
		return inferredType
	}

	// 4. 默认返回 string
	return "string"
}

// parseJinja2Default 解析 Jinja2 default 表达式中的默认值类型
// 示例: "{{ threshold | default(90) }}" -> "number"
func parseJinja2Default(expr string) string {
	// 匹配 default(value) 模式
	re := regexp.MustCompile(`default\s*\(\s*([^)]+)\s*\)`)
	matches := re.FindStringSubmatch(expr)
	if len(matches) < 2 {
		return ""
	}

	defaultVal := strings.TrimSpace(matches[1])

	// 布尔值
	if defaultVal == "true" || defaultVal == "false" || defaultVal == "True" || defaultVal == "False" {
		return "boolean"
	}

	// 数字（整数或浮点数）
	if matched, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, defaultVal); matched {
		return "number"
	}

	// 空列表或非空列表
	if strings.HasPrefix(defaultVal, "[") {
		return "list"
	}

	// 空对象或非空对象
	if strings.HasPrefix(defaultVal, "{") {
		return "object"
	}

	return ""
}

// inferTypeByName 基于变量名启发式推断类型
func inferTypeByName(name string) string {
	nameLower := strings.ToLower(name)

	// Boolean 类型模式
	boolPatterns := []string{
		"enabled", "disabled", "force", "verbose", "debug",
		"compress", "allow", "require", "skip", "dry_run",
	}
	boolPrefixes := []string{"is_", "has_", "can_", "should_", "enable_", "disable_", "use_"}
	boolSuffixes := []string{"_enabled", "_disabled", "_flag", "_mode"}

	for _, pattern := range boolPatterns {
		if nameLower == pattern {
			return "boolean"
		}
	}
	for _, prefix := range boolPrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return "boolean"
		}
	}
	for _, suffix := range boolSuffixes {
		if strings.HasSuffix(nameLower, suffix) {
			return "boolean"
		}
	}

	// Number 类型模式
	numberSuffixes := []string{
		"_threshold", "_count", "_timeout", "_port", "_size",
		"_limit", "_max", "_min", "_interval", "_retries",
		"_delay", "_seconds", "_minutes", "_hours", "_days",
		"_percent", "_percentage", "_rate", "_number",
	}
	for _, suffix := range numberSuffixes {
		if strings.HasSuffix(nameLower, suffix) {
			return "number"
		}
	}

	// List 类型模式
	listSuffixes := []string{
		"_hosts", "_dirs", "_files", "_paths", "_list",
		"_items", "_servers", "_nodes", "_addresses",
	}
	for _, suffix := range listSuffixes {
		if strings.HasSuffix(nameLower, suffix) {
			return "list"
		}
	}

	return ""
}

// isBuiltinVariable 是否为 Ansible 内置变量
func isBuiltinVariable(name string) bool {
	builtins := map[string]bool{
		"item":               true,
		"ansible_facts":      true,
		"ansible_host":       true,
		"ansible_user":       true,
		"ansible_password":   true,
		"inventory_hostname": true,
		"hostvars":           true,
		"groups":             true,
		"group_names":        true,
		"play_hosts":         true,
		"ansible_play_hosts": true,
		"ansible_check_mode": true,
		"ansible_version":    true,
		"ansible_date_time":  true,
		"ansible_env":        true,
		"ansible_connection": true,
		"ansible_ssh_host":   true,
		"lookup":             true,
		"omit":               true,
		"now":                true,
		"true":               true,
		"false":              true,
	}

	// 检查前缀
	prefixes := []string{"ansible_", "hostvars", "groups"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return builtins[name]
}

// ==================== 增强模式 ====================

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

// parseEnhancedConfig 解析增强模式配置文件
func (s *Service) parseEnhancedConfig(repoPath string) map[string]*EnhancedVariable {
	result := make(map[string]*EnhancedVariable)

	// 尝试多个可能的配置文件名
	configFiles := []string{
		filepath.Join(repoPath, ".auto-healing.yml"),
		filepath.Join(repoPath, ".auto-healing.yaml"),
		filepath.Join(repoPath, "playbook.meta.yml"),
		filepath.Join(repoPath, "playbook.meta.yaml"),
	}

	var configPath string
	for _, path := range configFiles {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	if configPath == "" {
		return result // 没有找到配置文件，返回空
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		logger.Sync_("PLAYBOOK").Warn("读取增强配置文件失败: %v", err)
		return result
	}

	var config EnhancedConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		logger.Sync_("PLAYBOOK").Warn("解析增强配置文件失败: %v", err)
		return result
	}

	for i := range config.Variables {
		v := &config.Variables[i]
		if v.Name != "" {
			// 默认类型为 string
			if v.Type == "" {
				v.Type = "string"
			}
			result[v.Name] = v
		}
	}

	logger.Sync_("PLAYBOOK").Info("从 %s 加载了 %d 个增强模式变量定义", filepath.Base(configPath), len(result))
	return result
}

// ==================== 关联任务变更检测 ====================

// notifyRelatedTasks 检测并更新关联任务模板的 review 状态
func (s *Service) notifyRelatedTasks(ctx context.Context, playbookID uuid.UUID, newVariables model.JSONArray) {
	// 获取所有关联的任务模板
	tasks, err := s.executionRepo.ListTasksByPlaybookID(ctx, playbookID)
	if err != nil {
		logger.Sync_("PLAYBOOK").Warn("查询关联任务失败: %v", err)
		return
	}

	logger.Sync_("PLAYBOOK").Info("变量同步检查: Playbook %s 有 %d 个关联任务, %d 个变量", playbookID, len(tasks), len(newVariables))

	if len(tasks) == 0 {
		return
	}

	// 构建新变量名集合和类型映射
	newVarMap := make(map[string]string) // name -> type
	for _, v := range newVariables {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				varType, _ := vm["type"].(string)
				newVarMap[name] = varType
			}
		}
	}

	// 检查每个任务模板
	for _, task := range tasks {
		snapshotLen := len(task.PlaybookVariablesSnapshot)
		changedVars := s.detectChangedVariables(task.PlaybookVariablesSnapshot, newVarMap)

		logger.Sync_("PLAYBOOK").Info("任务 %s: 快照变量数=%d, Playbook变量数=%d, 检测到变更=%d",
			task.Name, snapshotLen, len(newVarMap), len(changedVars))

		if len(changedVars) > 0 {
			// 转换为 JSONArray
			changedList := make(model.JSONArray, len(changedVars))
			for i, v := range changedVars {
				changedList[i] = v
			}

			// 更新任务模板的 review 状态
			if err := s.executionRepo.UpdateTaskReviewStatus(ctx, task.ID, true, changedList); err != nil {
				logger.Sync_("PLAYBOOK").Warn("更新任务 %s review 状态失败: %v", task.ID, err)
			} else {
				logger.Sync_("PLAYBOOK").Info("任务 %s 需要审核，变更字段: %v", task.Name, changedVars)
			}
		} else if !task.NeedsReview {
			logger.Sync_("PLAYBOOK").Info("任务 %s 无变更，跳过", task.Name)
		} else {
			// 之前需要审核，现在没有变更了（例如变量恢复了），清除审核状态
			if err := s.executionRepo.UpdateTaskReviewStatus(ctx, task.ID, false, model.JSONArray{}); err != nil {
				logger.Sync_("PLAYBOOK").Warn("清除任务 %s review 状态失败: %v", task.ID, err)
			} else {
				logger.Sync_("PLAYBOOK").Info("任务 %s 变量已同步，清除审核状态", task.Name)
			}
		}
	}
}

// detectChangedVariables 检测变更的变量名
func (s *Service) detectChangedVariables(snapshot model.JSONArray, newVarMap map[string]string) []string {
	var changed []string

	// 构建快照变量映射
	snapshotVarMap := make(map[string]string) // name -> type
	for _, v := range snapshot {
		if vm, ok := v.(map[string]any); ok {
			if name, ok := vm["name"].(string); ok {
				varType, _ := vm["type"].(string)
				snapshotVarMap[name] = varType
			}
		}
	}

	// 检查类型变更
	for name, oldType := range snapshotVarMap {
		if newType, exists := newVarMap[name]; exists {
			if oldType != newType {
				changed = append(changed, name)
			}
		} else {
			// 变量被删除
			changed = append(changed, name)
		}
	}

	// 检查新增变量
	for name := range newVarMap {
		if _, exists := snapshotVarMap[name]; !exists {
			changed = append(changed, name)
		}
	}

	return changed
}
