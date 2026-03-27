package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	gitclient "github.com/company/auto-healing/internal/git"
	"github.com/company/auto-healing/internal/model"
	playbookSvc "github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

const DefaultReposDir = "/var/lib/auto-healing/repos"

// Service Git 仓库服务
type Service struct {
	repo         *repository.GitRepositoryRepository
	playbookRepo *repository.PlaybookRepository
	reposDir     string
	lifecycle    *asyncLifecycle
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
		lifecycle:    newAsyncLifecycle(),
	}
}

// ValidateRepoResult 验证仓库结果
type ValidateRepoResult struct {
	Branches      []string `json:"branches"`
	DefaultBranch string   `json:"default_branch"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Size     int64      `json:"size,omitempty"`
	Path     string     `json:"path"`
	Children []FileInfo `json:"children,omitempty"`
}

// PlaybookVariable Playbook 变量定义
type PlaybookVariable struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Min         *int     `json:"min,omitempty"`
	Max         *int     `json:"max,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
}

// CreateRepo 创建仓库
// 流程：验证远程仓库 -> 创建记录 -> 首次同步
func (s *Service) CreateRepo(ctx context.Context, repo *model.GitRepository) (*model.GitRepository, error) {
	repo.LocalPath = filepath.Join(s.reposDir, uuid.New().String())
	if err := validateRepoMutation(repo.AuthType, repo.SyncEnabled, repo.SyncInterval, repo.MaxFailures); err != nil {
		return nil, err
	}

	if err := gitclient.NewClient(repo, s.reposDir).ValidateRemote(ctx); err != nil {
		return nil, fmt.Errorf("仓库验证失败: %w", err)
	}
	repo.NextSyncAt = nextRepoSyncAt(repo.SyncEnabled, repo.SyncInterval)

	repo.LocalPath = ""
	if err := s.repo.Create(ctx, repo); err != nil {
		return nil, err
	}
	if err := s.initializeRepoLocalPath(ctx, repo); err != nil {
		return nil, err
	}
	if err := s.syncRepoInternal(ctx, repo, "create"); err != nil {
		cleanupErr := cleanupRepoDirectory(repo.LocalPath)
		if deleteErr := s.repo.Delete(ctx, repo.ID); deleteErr != nil {
			return nil, errors.Join(fmt.Errorf("首次同步失败: %w", err), cleanupErr, fmt.Errorf("回滚删除失败: %w", deleteErr))
		}
		return nil, errors.Join(fmt.Errorf("首次同步失败: %w", err), cleanupErr)
	}
	return repo, nil
}

func (s *Service) initializeRepoLocalPath(ctx context.Context, repo *model.GitRepository) error {
	repo.LocalPath = filepath.Join(s.reposDir, repo.ID.String())
	if err := s.repo.UpdateLocalPath(ctx, repo.ID, repo.LocalPath); err != nil {
		if deleteErr := s.repo.Delete(ctx, repo.ID); deleteErr != nil {
			return fmt.Errorf("初始化仓库本地路径失败: %w（回滚删除失败: %v）", err, deleteErr)
		}
		return fmt.Errorf("初始化仓库本地路径失败: %w", err)
	}
	return nil
}

// GetCommits 获取仓库最近的 commit 历史
func (s *Service) GetCommits(ctx context.Context, id uuid.UUID, limit int) ([]gitclient.CommitInfo, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return gitclient.NewClient(repo, s.reposDir).GetCommits(ctx, limit)
}

func (s *Service) GetRepo(ctx context.Context, id uuid.UUID) (*model.GitRepository, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateRepo 更新仓库
func (s *Service) UpdateRepo(ctx context.Context, id uuid.UUID, defaultBranch, authType string, authConfig model.JSON, syncEnabled *bool, syncInterval *string, maxFailures *int) (*model.GitRepository, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	oldBranch := repo.DefaultBranch
	applyRepoUpdates(repo, defaultBranch, authType, authConfig, syncEnabled, syncInterval, maxFailures)
	repo.NextSyncAt = nextRepoSyncAt(repo.SyncEnabled, repo.SyncInterval)
	if err := validateRepoMutation(repo.AuthType, repo.SyncEnabled, repo.SyncInterval, repo.MaxFailures); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateConfig(ctx, repo.ID, repository.GitRepositoryConfigUpdate{
		DefaultBranch: repo.DefaultBranch,
		AuthType:      repo.AuthType,
		AuthConfig:    repo.AuthConfig,
		SyncEnabled:   repo.SyncEnabled,
		SyncInterval:  repo.SyncInterval,
		NextSyncAt:    repo.NextSyncAt,
		MaxFailures:   repo.MaxFailures,
	}); err != nil {
		return nil, err
	}
	if defaultBranch != "" && defaultBranch != oldBranch {
		s.triggerBranchChangeSync(ctx, repo, id, oldBranch, defaultBranch)
	}
	return s.repo.GetByID(ctx, id)
}

func applyRepoUpdates(repo *model.GitRepository, defaultBranch, authType string, authConfig model.JSON, syncEnabled *bool, syncInterval *string, maxFailures *int) {
	if defaultBranch != "" {
		repo.DefaultBranch = defaultBranch
	}
	if authType != "" {
		repo.AuthType = authType
	}
	if authConfig != nil {
		repo.AuthConfig = authConfig
	}
	if syncEnabled != nil {
		repo.SyncEnabled = *syncEnabled
	}
	if syncInterval != nil && *syncInterval != "" {
		repo.SyncInterval = *syncInterval
	}
	if maxFailures != nil {
		repo.MaxFailures = *maxFailures
	}
}

func nextRepoSyncAt(syncEnabled bool, syncInterval string) *time.Time {
	if !syncEnabled {
		return nil
	}
	if duration, err := time.ParseDuration(syncInterval); err == nil {
		next := time.Now().Add(duration)
		return &next
	}
	return nil
}

// DeleteRepo 删除仓库（保护性删除）
func (s *Service) DeleteRepo(ctx context.Context, id uuid.UUID) error {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	playbookCount, err := s.playbookRepo.CountByRepositoryID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查关联 Playbook 失败: %w", err)
	}
	if playbookCount > 0 {
		return fmt.Errorf("无法删除：该仓库下有 %d 个 Playbook，请先删除关联的 Playbook", playbookCount)
	}
	if err := cleanupRepoDirectory(repo.LocalPath); err != nil {
		return err
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
	return s.repo.UpdateStatus(ctx, repo.ID, targetStatus, "")
}

// ValidateRepo 验证仓库（无需创建）
func (s *Service) ValidateRepo(ctx context.Context, url, authType string, authConfig model.JSON) (*ValidateRepoResult, error) {
	tempRepo := &model.GitRepository{URL: url, AuthType: authType, AuthConfig: authConfig}
	branches, defaultBranch, err := gitclient.NewClient(tempRepo, s.reposDir).ValidateAndListBranches(ctx)
	if err != nil {
		return nil, err
	}
	return &ValidateRepoResult{Branches: branches, DefaultBranch: defaultBranch}, nil
}

// GetSyncLogs 获取同步日志
func (s *Service) GetSyncLogs(ctx context.Context, id uuid.UUID, page, pageSize int) ([]model.GitSyncLog, int64, error) {
	return s.repo.ListSyncLogs(ctx, id, page, pageSize)
}

// GetStats 获取 Git 仓库统计信息
func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}

func (s *Service) getPlaybookService() *playbookSvc.Service {
	return playbookSvc.NewService()
}

func cleanupRepoDirectory(path string) error {
	if path == "" {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("清理仓库目录失败: %w", err)
	}
	return nil
}

// 注意：Activate、Deactivate 函数已移除
// 相关功能现在由 Playbook Service 处理
