package playbook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
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

// Create 创建 Playbook
func (s *Service) Create(ctx context.Context, repositoryID uuid.UUID, name, filePath, description, configMode string) (*model.Playbook, error) {
	gitRepo, err := s.gitRepo.GetByID(ctx, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("仓库不存在: %w", err)
	}
	if gitRepo.Status != "synced" && gitRepo.Status != "ready" {
		return nil, fmt.Errorf("仓库未同步，请先同步仓库")
	}
	if _, err := os.Stat(filepath.Join(gitRepo.LocalPath, filePath)); os.IsNotExist(err) {
		return nil, fmt.Errorf("入口文件不存在: %s", filePath)
	}
	if configMode != "auto" && configMode != "enhanced" {
		return nil, fmt.Errorf("无效的扫描模式，必须为 auto 或 enhanced")
	}

	playbook := &model.Playbook{
		RepositoryID: repositoryID,
		Name:         name,
		Description:  description,
		FilePath:     filePath,
		ConfigMode:   configMode,
		Status:       "pending",
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
	Type string `json:"type"`
}

// GetFiles 获取 Playbook 扫描过的文件列表
func (s *Service) GetFiles(ctx context.Context, id uuid.UUID) ([]ScannedFile, error) {
	playbook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	logs, _, err := s.repo.ListScanLogs(ctx, id, 1, 1)
	if err != nil || len(logs) == 0 {
		return []ScannedFile{{Path: playbook.FilePath, Type: "entry"}}, nil
	}

	fileMap := map[string]string{playbook.FilePath: "entry"}
	appendScannedFiles(fileMap, logs[0], playbook.FilePath)

	files := make([]ScannedFile, 0, len(fileMap))
	for path, typ := range fileMap {
		files = append(files, ScannedFile{Path: path, Type: typ})
	}
	return files, nil
}

func appendScannedFiles(fileMap map[string]string, log model.PlaybookScanLog, playbookPath string) {
	if log.Details == nil {
		return
	}
	files, ok := log.Details["files"].([]interface{})
	if !ok {
		return
	}

	for _, item := range files {
		filePath, ok := item.(string)
		if !ok {
			continue
		}
		relPath := toRelativeScanPath(filePath, playbookPath)
		if relPath != "" && relPath != playbookPath {
			fileMap[relPath] = inferFileType(relPath)
		}
	}
}

func toRelativeScanPath(filePath, playbookPath string) string {
	if idx := strings.Index(filePath, playbookPath); idx > 0 {
		return strings.TrimPrefix(filePath, filePath[:idx])
	}
	parts := strings.Split(filePath, "/repos/")
	if len(parts) > 1 {
		subParts := strings.SplitN(parts[1], "/", 2)
		if len(subParts) > 1 {
			return subParts[1]
		}
	}
	return filePath
}

func inferFileType(path string) string {
	switch {
	case strings.Contains(path, "/tasks/"):
		return "task"
	case strings.Contains(path, "/vars/"):
		return "vars"
	case strings.Contains(path, "/defaults/"):
		return "defaults"
	case strings.Contains(path, "/handlers/"):
		return "handlers"
	case strings.Contains(path, "/templates/"):
		return "template"
	case strings.Contains(path, "/files/"):
		return "file"
	case strings.Contains(path, "roles/"):
		return "role"
	default:
		return "include"
	}
}

// GetScanLogs 获取扫描日志
func (s *Service) GetScanLogs(ctx context.Context, playbookID uuid.UUID, page, pageSize int) ([]model.PlaybookScanLog, int64, error) {
	return s.repo.ListScanLogs(ctx, playbookID, page, pageSize)
}

// CanDeleteRepository 检查仓库是否可以删除
func (s *Service) CanDeleteRepository(ctx context.Context, repositoryID uuid.UUID) (bool, int64, error) {
	count, err := s.repo.CountByRepositoryID(ctx, repositoryID)
	if err != nil {
		return false, 0, err
	}
	return count == 0, count, nil
}

// GetStats 获取 Playbook 统计信息
func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}
