package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	gitclient "github.com/company/auto-healing/internal/git"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	playbookSvc "github.com/company/auto-healing/internal/service/playbook"
	"github.com/google/uuid"
)

const (
	DefaultReposDir = "/var/lib/auto-healing/repos"
)

// Service Git 仓库服务
type Service struct {
	repo         *repository.GitRepositoryRepository
	playbookRepo *repository.PlaybookRepository
	reposDir     string
}

// NewService 创建 Git 仓库服务
func NewService() *Service {
	reposDir := os.Getenv("GIT_REPOS_DIR")
	if reposDir == "" {
		reposDir = DefaultReposDir
	}
	return &Service{
		repo:         repository.NewGitRepositoryRepository(),
		playbookRepo: repository.NewPlaybookRepository(),
		reposDir:     reposDir,
	}
}

// CreateRepo 创建仓库
// 流程：验证远程仓库 -> 创建记录 -> 首次同步
func (s *Service) CreateRepo(ctx context.Context, repo *model.GitRepository) (*model.GitRepository, error) {
	// 设置本地路径（临时用于验证）
	tempID := uuid.New()
	repo.LocalPath = filepath.Join(s.reposDir, tempID.String())

	// 1. 验证远程仓库（URL、认证、分支）
	client := gitclient.NewClient(repo, s.reposDir)
	if err := client.ValidateRemote(ctx); err != nil {
		return nil, fmt.Errorf("仓库验证失败: %w", err)
	}

	// 2. 如果启用同步，设置下次同步时间
	if repo.SyncEnabled {
		if duration, err := time.ParseDuration(repo.SyncInterval); err == nil {
			t := time.Now().Add(duration)
			repo.NextSyncAt = &t
		}
	}

	// 3. 创建数据库记录
	repo.LocalPath = "" // 清空临时路径
	if err := s.repo.Create(ctx, repo); err != nil {
		return nil, err
	}

	// 4. 设置正式本地路径
	repo.LocalPath = filepath.Join(s.reposDir, repo.ID.String())
	s.repo.Update(ctx, repo)

	// 5. 首次同步（克隆）
	if err := s.syncRepoInternal(ctx, repo, "create"); err != nil {
		// 同步失败，回滚：删除记录
		s.repo.Delete(ctx, repo.ID)
		return nil, fmt.Errorf("首次同步失败: %w", err)
	}

	return repo, nil
}

// syncRepoInternal 内部同步方法（不重新获取 repo）
func (s *Service) syncRepoInternal(ctx context.Context, repo *model.GitRepository, triggerType string) error {
	s.repo.UpdateStatus(ctx, repo.ID, "syncing", "")
	startTime := time.Now()

	client := gitclient.NewClient(repo, s.reposDir)

	action := "克隆"
	var err error
	if client.Exists() {
		action = "拉取"
		err = client.Pull(ctx, repo.DefaultBranch)
	} else {
		err = client.Clone(ctx)
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	if err != nil {
		s.repo.UpdateStatus(ctx, repo.ID, "error", err.Error())
		logger.Sync_("GIT").Error("失败: %s | 操作: %s | 错误: %v", repo.Name, action, err)

		s.repo.CreateSyncLog(ctx, &model.GitSyncLog{
			RepositoryID: repo.ID,
			TriggerType:  triggerType,
			Action:       action,
			Status:       "failed",
			Branch:       repo.DefaultBranch,
			DurationMs:   durationMs,
			ErrorMessage: err.Error(),
		})
		return err
	}

	// 更新状态和同步时间
	now := time.Now()
	repo.Status = "ready"
	repo.LastSyncAt = &now
	repo.ErrorMessage = ""

	// 获取最新 commit ID
	var commitID string
	if commitID, err = client.GetLatestCommitID(ctx); err == nil {
		repo.LastCommitID = commitID
	}

	logger.Sync_("GIT").Info("完成: %s | 操作: %s | 分支: %s | Commit: %s | 耗时: %v", repo.Name, action, repo.DefaultBranch, repo.LastCommitID, duration)

	s.repo.CreateSyncLog(ctx, &model.GitSyncLog{
		RepositoryID: repo.ID,
		TriggerType:  triggerType,
		Action:       action,
		Status:       "success",
		CommitID:     commitID,
		Branch:       repo.DefaultBranch,
		DurationMs:   durationMs,
	})

	s.repo.Update(ctx, repo)
	return nil
}

// ValidateRepoResult 验证仓库结果
type ValidateRepoResult struct {
	Branches      []string `json:"branches"`
	DefaultBranch string   `json:"default_branch"`
}

// ValidateRepo 验证仓库（无需创建）
func (s *Service) ValidateRepo(ctx context.Context, url, authType string, authConfig model.JSON) (*ValidateRepoResult, error) {
	// 构建临时 repo 对象用于验证
	tempRepo := &model.GitRepository{
		URL:        url,
		AuthType:   authType,
		AuthConfig: authConfig,
	}

	client := gitclient.NewClient(tempRepo, s.reposDir)
	branches, defaultBranch, err := client.ValidateAndListBranches(ctx)
	if err != nil {
		return nil, err
	}

	return &ValidateRepoResult{
		Branches:      branches,
		DefaultBranch: defaultBranch,
	}, nil
}

// GetCommits 获取仓库最近的 commit 历史
func (s *Service) GetCommits(ctx context.Context, id uuid.UUID, limit int) ([]gitclient.CommitInfo, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	client := gitclient.NewClient(repo, s.reposDir)
	return client.GetCommits(ctx, limit)
}
func (s *Service) GetRepo(ctx context.Context, id uuid.UUID) (*model.GitRepository, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateRepo 更新仓库
func (s *Service) UpdateRepo(ctx context.Context, id uuid.UUID, defaultBranch, authType string, authConfig model.JSON, syncEnabled *bool, syncInterval *string) (*model.GitRepository, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 记录旧分支，用于判断是否需要重新同步
	oldBranch := repo.DefaultBranch

	if defaultBranch != "" {
		repo.DefaultBranch = defaultBranch
	}
	if authType != "" {
		repo.AuthType = authType
	}
	if authConfig != nil {
		repo.AuthConfig = authConfig
	}

	// 更新同步配置
	if syncEnabled != nil {
		repo.SyncEnabled = *syncEnabled
	}
	if syncInterval != nil && *syncInterval != "" {
		repo.SyncInterval = *syncInterval
	}

	// 如果启用了同步，更新下次同步时间
	if repo.SyncEnabled {
		if duration, err := time.ParseDuration(repo.SyncInterval); err == nil {
			t := time.Now().Add(duration)
			repo.NextSyncAt = &t
		}
	} else {
		repo.NextSyncAt = nil
	}

	if err := s.repo.Update(ctx, repo); err != nil {
		return nil, err
	}

	// 分支变更时自动触发同步，确保代码切换到新分支
	if defaultBranch != "" && defaultBranch != oldBranch {
		logger.Info("仓库 %s 默认分支变更: %s → %s，触发自动同步", repo.Name, oldBranch, defaultBranch)
		go s.SyncRepoWithTrigger(context.Background(), id, "branch_change")
	}

	return repo, nil
}

// DeleteRepo 删除仓库（保护性删除）
func (s *Service) DeleteRepo(ctx context.Context, id uuid.UUID) error {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否有关联的 Playbook
	playbookCount, err := s.playbookRepo.CountByRepositoryID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查关联 Playbook 失败: %w", err)
	}
	if playbookCount > 0 {
		return fmt.Errorf("无法删除：该仓库下有 %d 个 Playbook，请先删除关联的 Playbook", playbookCount)
	}

	// 删除本地文件
	if repo.LocalPath != "" {
		os.RemoveAll(repo.LocalPath)
	}

	return s.repo.Delete(ctx, id)
}

// ListRepos 列出仓库（向后兼容）
func (s *Service) ListRepos(ctx context.Context, status string) ([]model.GitRepository, error) {
	return s.repo.List(ctx, status)
}

// ListReposWithOptions 列出仓库（支持完整查询参数）
func (s *Service) ListReposWithOptions(ctx context.Context, opts *repository.GitRepoListOptions) ([]model.GitRepository, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return s.repo.ListWithOptions(ctx, opts)
}

// ResetStatus 强制重置仓库状态
func (s *Service) ResetStatus(ctx context.Context, id uuid.UUID, targetStatus string) error {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	repo.Status = targetStatus
	repo.ErrorMessage = ""
	return s.repo.Update(ctx, repo)
}

// SyncRepo 同步仓库
func (s *Service) SyncRepo(ctx context.Context, id uuid.UUID) error {
	return s.SyncRepoWithTrigger(ctx, id, "manual")
}

// SyncRepoWithTrigger 同步仓库（带触发类型）
func (s *Service) SyncRepoWithTrigger(ctx context.Context, id uuid.UUID, triggerType string) error {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 更新状态为 syncing
	s.repo.UpdateStatus(ctx, id, "syncing", "")
	startTime := time.Now()

	client := gitclient.NewClient(repo, s.reposDir)

	// 判断是克隆还是拉取
	action := "clone"
	actionName := "克隆"
	if client.Exists() {
		action = "pull"
		actionName = "拉取"
		err = client.Pull(ctx, repo.DefaultBranch)
	} else {
		err = client.Clone(ctx)
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	if err != nil {
		s.repo.UpdateStatus(ctx, id, "error", err.Error())
		logger.Sync_("GIT").Error("失败: %s | 操作: %s | 错误: %v", repo.Name, actionName, err)

		// 记录失败日志
		s.repo.CreateSyncLog(ctx, &model.GitSyncLog{
			RepositoryID: id,
			TriggerType:  triggerType,
			Action:       action,
			Status:       "failed",
			Branch:       repo.DefaultBranch,
			DurationMs:   durationMs,
			ErrorMessage: err.Error(),
		})
		return err
	}

	// 更新状态和同步时间
	now := time.Now()
	repo.Status = "ready"
	repo.LastSyncAt = &now
	repo.ErrorMessage = ""

	// 获取最新 commit ID
	var commitID string
	if commitID, err = client.GetLatestCommitID(ctx); err == nil {
		repo.LastCommitID = commitID
	}

	// 同步成功后更新分支列表缓存
	if branchList, branchErr := client.ListBranches(ctx); branchErr == nil && len(branchList) > 0 {
		s.repo.UpdateBranches(ctx, id, branchList)
		logger.Sync_("GIT").Info("更新分支缓存: %s | %d 个分支", repo.Name, len(branchList))
	}

	// 输出汇总日志
	logger.Sync_("GIT").Info("完成: %s | 操作: %s | 分支: %s | Commit: %s | 耗时: %v", repo.Name, actionName, repo.DefaultBranch, repo.LastCommitID, duration)

	// 记录成功日志
	s.repo.CreateSyncLog(ctx, &model.GitSyncLog{
		RepositoryID: id,
		TriggerType:  triggerType,
		Action:       action,
		Status:       "success",
		CommitID:     commitID,
		Branch:       repo.DefaultBranch,
		DurationMs:   durationMs,
	})

	// 检查关联的 Playbooks
	go s.checkPlaybooksAfterSync(id)

	s.repo.Update(ctx, repo)
	return nil
}

// checkPlaybooksAfterSync 同步后检查并自动扫描关联的 Playbooks
func (s *Service) checkPlaybooksAfterSync(repositoryID uuid.UUID) {
	// panic 保护
	defer func() {
		if rec := recover(); rec != nil {
			logger.Sync_("GIT").Error("checkPlaybooksAfterSync panic: %v", rec)
		}
	}()

	ctx := context.Background()

	// 获取关联的 Playbooks
	playbooks, err := s.playbookRepo.ListByRepositoryID(ctx, repositoryID)
	if err != nil {
		logger.Sync_("GIT").Warn("获取关联 Playbooks 失败: %v", err)
		return
	}

	if len(playbooks) == 0 {
		return
	}

	// 获取仓库信息
	gitRepo, err := s.repo.GetByID(ctx, repositoryID)
	if err != nil {
		return
	}

	scannedCount := 0
	for _, playbook := range playbooks {
		// 检查入口文件是否存在
		fullPath := filepath.Join(gitRepo.LocalPath, playbook.FilePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// 入口文件不存在，标记为 invalid
			s.playbookRepo.UpdateStatus(ctx, playbook.ID, "invalid")
			logger.Sync_("GIT").Warn("Playbook %s 入口文件不存在，标记为 invalid", playbook.Name)
			continue
		}

		// 只对已手动扫描过的 Playbook 触发自动扫描（LastScannedAt 不为空 或 ConfigMode 已设置）
		if playbook.LastScannedAt != nil || playbook.ConfigMode != "" {
			// 调用 Playbook 服务进行扫描
			if playbookSvc := s.getPlaybookService(); playbookSvc != nil {
				if _, err := playbookSvc.ScanVariables(ctx, playbook.ID, "repo_sync"); err != nil {
					logger.Sync_("GIT").Warn("Playbook %s 自动扫描失败: %v", playbook.Name, err)
				} else {
					scannedCount++
					logger.Sync_("GIT").Info("Playbook %s 自动扫描完成", playbook.Name)
				}
			}
		}
	}

	logger.Sync_("GIT").Info("已检查 %d 个关联 Playbooks，自动扫描 %d 个", len(playbooks), scannedCount)
}

// getPlaybookService 获取 Playbook 服务（延迟加载避免循环依赖）
func (s *Service) getPlaybookService() *playbookSvc.Service {
	return playbookSvc.NewService()
}

// GetSyncLogs 获取同步日志
func (s *Service) GetSyncLogs(ctx context.Context, id uuid.UUID, page, pageSize int) ([]model.GitSyncLog, int64, error) {
	return s.repo.ListSyncLogs(ctx, id, page, pageSize)
}

// FileInfo 文件信息
type FileInfo struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"` // file / directory
	Size     int64      `json:"size,omitempty"`
	Path     string     `json:"path"`
	Children []FileInfo `json:"children,omitempty"`
}

// PlaybookVariable Playbook 变量定义
type PlaybookVariable struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // string, int, bool, password, select, multiselect, text
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Min         *int     `json:"min,omitempty"`
	Max         *int     `json:"max,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
}

// GetFiles 获取文件树
func (s *Service) GetFiles(ctx context.Context, id uuid.UUID) ([]FileInfo, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if repo.LocalPath == "" || repo.Status != "ready" {
		return nil, fmt.Errorf("仓库未同步")
	}

	files, err := s.scanDirectory(repo.LocalPath, "")
	if err != nil {
		return nil, err
	}

	return files, nil
}

// scanDirectory 扫描目录
func (s *Service) scanDirectory(basePath, relativePath string) ([]FileInfo, error) {
	fullPath := filepath.Join(basePath, relativePath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		// 跳过 .git 目录
		if entry.Name() == ".git" {
			continue
		}

		info := FileInfo{
			Name: entry.Name(),
			Path: filepath.Join(relativePath, entry.Name()),
		}

		if entry.IsDir() {
			info.Type = "directory"
			// 递归扫描子目录
			children, _ := s.scanDirectory(basePath, info.Path)
			info.Children = children
		} else {
			info.Type = "file"
			if fi, err := entry.Info(); err == nil {
				info.Size = fi.Size()
			}
		}

		files = append(files, info)
	}

	return files, nil
}

// GetFileContent 获取文件内容
func (s *Service) GetFileContent(ctx context.Context, id uuid.UUID, path string) (string, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	if repo.LocalPath == "" {
		return "", fmt.Errorf("仓库未同步")
	}

	fullPath := filepath.Join(repo.LocalPath, path)

	// 安全检查：确保路径在仓库内
	if !filepath.HasPrefix(fullPath, repo.LocalPath) {
		return "", fmt.Errorf("非法路径")
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// ScanVariables 扫描 Playbook 变量
func (s *Service) ScanVariables(ctx context.Context, id uuid.UUID, mainPlaybook string) ([]PlaybookVariable, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if repo.LocalPath == "" {
		return nil, fmt.Errorf("仓库未同步")
	}

	// 读取主 playbook 文件
	playbookPath := filepath.Join(repo.LocalPath, mainPlaybook)
	content, err := os.ReadFile(playbookPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取 playbook: %v", err)
	}

	// 扫描 {{ var }} 模式
	variables := s.extractVariables(string(content))

	logger.Sync_("GIT").Info("仓库: %s | 入口: %s | 发现变量: %d 个", repo.Name, mainPlaybook, len(variables))

	return variables, nil
}

// extractVariables 从内容中提取变量
func (s *Service) extractVariables(content string) []PlaybookVariable {
	varMap := make(map[string]bool)
	var variables []PlaybookVariable

	// 简单的正则匹配 {{ var_name }} 或 {{ var_name | default('xxx') }}
	// 这里使用简化版本，实际可以用正则
	inVar := false
	varStart := 0

	for i := 0; i < len(content)-1; i++ {
		if content[i] == '{' && content[i+1] == '{' {
			inVar = true
			varStart = i + 2
		} else if inVar && content[i] == '}' && content[i+1] == '}' {
			varName := content[varStart:i]
			// 清理空格和过滤器
			varName = s.cleanVarName(varName)
			if varName != "" && !varMap[varName] {
				varMap[varName] = true
				variables = append(variables, PlaybookVariable{
					Name: varName,
					Type: "string",
				})
			}
			inVar = false
		}
	}

	return variables
}

// cleanVarName 清理变量名
func (s *Service) cleanVarName(raw string) string {
	// 移除空格
	raw = filepath.Clean(raw)
	var result []byte
	for _, c := range raw {
		if c == ' ' || c == '\t' || c == '\n' {
			continue
		}
		if c == '|' { // 遇到过滤器就停止
			break
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// ==================== 统计 ====================

// GetStats 获取 Git 仓库统计信息
func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}

// 注意：Activate、Deactivate 函数已移除
// 相关功能现在由 Playbook Service 处理
