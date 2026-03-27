package git

import (
	"context"
	"fmt"
	"os"
	"time"

	gitclient "github.com/company/auto-healing/internal/modules/integrations/gitclient"
	"github.com/company/auto-healing/internal/modules/integrations/model"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	playbookSvc "github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"gorm.io/gorm"
)

type ServiceDeps struct {
	Repo         *integrationrepo.GitRepositoryRepository
	PlaybookRepo *integrationrepo.PlaybookRepository
	ReposDir     string
	PlaybookSvc  func() *playbookSvc.Service
	Lifecycle    *asyncLifecycle
}

func DefaultServiceDepsWithDB(db *gorm.DB) ServiceDeps {
	return ServiceDeps{
		Repo:         integrationrepo.NewGitRepositoryRepositoryWithDB(db),
		PlaybookRepo: integrationrepo.NewPlaybookRepositoryWithDB(db),
		ReposDir:     defaultReposDir(),
		PlaybookSvc: func() *playbookSvc.Service {
			return playbookSvc.NewServiceWithDB(db)
		},
		Lifecycle: newAsyncLifecycle(),
	}
}

func defaultReposDir() string {
	reposDir := os.Getenv("GIT_REPOS_DIR")
	if reposDir == "" {
		return DefaultReposDir
	}
	return reposDir
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

func cleanupRepoDirectory(path string) error {
	if path == "" {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("清理仓库目录失败: %w", err)
	}
	return nil
}

func buildValidateRepoResult(client *gitclient.Client, ctx context.Context) (*ValidateRepoResult, error) {
	branches, defaultBranch, err := client.ValidateAndListBranches(ctx)
	if err != nil {
		return nil, err
	}
	return &ValidateRepoResult{Branches: branches, DefaultBranch: defaultBranch}, nil
}
