package git

import (
	"context"
	"errors"
	"fmt"
	"time"

	gitclient "github.com/company/auto-healing/internal/modules/integrations/gitclient"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
)

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

	result, err := s.runRepoSync(ctx, repo)
	if err != nil {
		return s.recordFailedSync(ctx, repo, triggerType, result, err)
	}
	if err := s.persistSuccessfulRepoSync(ctx, repo, result); err != nil {
		return err
	}
	s.triggerPlaybookCheck(ctx, repo)
	return s.recordSuccessfulSync(ctx, repo.ID, repo.DefaultBranch, triggerType, result)
}

func (s *Service) syncRepoInternal(ctx context.Context, repo *model.GitRepository, triggerType string) error {
	result, err := s.runRepoSync(ctx, repo)
	if err != nil {
		return s.recordFailedSync(ctx, repo, triggerType, result, err)
	}
	if err := s.recordSuccessfulSync(ctx, repo.ID, repo.DefaultBranch, triggerType, result); err != nil {
		return err
	}
	return s.persistRepoAfterSync(ctx, repo, result)
}

type repoSyncResult struct {
	action       string
	actionName   string
	commitID     string
	duration     time.Duration
	durationMs   int
	branchList   []string
	syncedAt     time.Time
	branchListOK bool
}

func (s *Service) runRepoSync(ctx context.Context, repo *model.GitRepository) (*repoSyncResult, error) {
	if err := s.repo.UpdateStatus(ctx, repo.ID, "syncing", ""); err != nil {
		return nil, fmt.Errorf("更新仓库同步中状态失败: %w", err)
	}
	client := gitclient.NewClient(repo, s.reposDir)
	startTime := time.Now()

	result := &repoSyncResult{action: "clone", actionName: "克隆"}
	var err error
	if client.Exists() {
		result.action = "pull"
		result.actionName = "拉取"
		err = client.Pull(ctx, repo.DefaultBranch)
	} else {
		err = client.Clone(ctx)
	}

	result.duration = time.Since(startTime)
	result.durationMs = int(result.duration.Milliseconds())
	result.syncedAt = time.Now()
	if err != nil {
		return result, err
	}

	if commitID, commitErr := client.GetLatestCommitID(ctx); commitErr == nil {
		result.commitID = commitID
	}
	if branches, branchErr := client.ListBranches(ctx); branchErr == nil && len(branches) > 0 {
		result.branchList = branches
		result.branchListOK = true
	}
	return result, nil
}

func (s *Service) recordFailedSync(ctx context.Context, repo *model.GitRepository, triggerType string, result *repoSyncResult, syncErr error) error {
	if result == nil {
		result = &repoSyncResult{action: "sync", actionName: "同步"}
	}
	var persistErrs []error
	if err := s.repo.UpdateStatus(ctx, repo.ID, "error", syncErr.Error()); err != nil {
		persistErrs = append(persistErrs, fmt.Errorf("更新仓库错误状态失败: %w", err))
	}
	logger.Sync_("GIT").Error("失败: %s | 操作: %s | 错误: %v", repo.Name, result.actionName, syncErr)
	if err := s.repo.CreateSyncLog(ctx, &model.GitSyncLog{
		RepositoryID: repo.ID,
		TriggerType:  triggerType,
		Action:       result.action,
		Status:       "failed",
		Branch:       repo.DefaultBranch,
		DurationMs:   result.durationMs,
		ErrorMessage: syncErr.Error(),
	}); err != nil {
		persistErrs = append(persistErrs, fmt.Errorf("创建仓库失败同步日志失败: %w", err))
	}
	if len(persistErrs) == 0 {
		return syncErr
	}
	return errors.Join(append([]error{syncErr}, persistErrs...)...)
}

func (s *Service) persistSuccessfulRepoSync(ctx context.Context, repo *model.GitRepository, result *repoSyncResult) error {
	if err := s.persistRepoAfterSync(ctx, repo, result); err != nil {
		if updateErr := s.repo.UpdateStatus(ctx, repo.ID, "error", fmt.Sprintf("同步成功但持久化仓库状态失败: %v", err)); updateErr != nil {
			return errors.Join(err, fmt.Errorf("更新仓库错误状态失败: %w", updateErr))
		}
		logger.Sync_("GIT").Error("仓库状态持久化失败: %s | %v", repo.Name, err)
		return err
	}
	return nil
}

func (s *Service) persistRepoAfterSync(ctx context.Context, repo *model.GitRepository, result *repoSyncResult) error {
	repo.Status = "ready"
	repo.LastSyncAt = &result.syncedAt
	repo.ErrorMessage = ""
	repo.LastCommitID = result.commitID
	if result.branchListOK {
		if err := s.repo.UpdateBranches(ctx, repo.ID, result.branchList); err != nil {
			return fmt.Errorf("更新分支缓存失败: %w", err)
		}
		logger.Sync_("GIT").Info("更新分支缓存: %s | %d 个分支", repo.Name, len(result.branchList))
	}
	logger.Sync_("GIT").Info("完成: %s | 操作: %s | 分支: %s | Commit: %s | 耗时: %v",
		repo.Name, result.actionName, repo.DefaultBranch, repo.LastCommitID, result.duration)
	latestRepo, err := s.repo.GetByID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("刷新仓库最新配置失败: %w", err)
	}
	return s.repo.UpdateSyncState(ctx, repo.ID, repo.Status, repo.ErrorMessage, repo.LastCommitID, repo.LastSyncAt, nextRepoSyncAt(latestRepo.SyncEnabled, latestRepo.SyncInterval))
}

func (s *Service) recordSuccessfulSync(ctx context.Context, repoID uuid.UUID, branch, triggerType string, result *repoSyncResult) error {
	return s.repo.CreateSyncLog(ctx, &model.GitSyncLog{
		RepositoryID: repoID,
		TriggerType:  triggerType,
		Action:       result.action,
		Status:       "success",
		CommitID:     result.commitID,
		Branch:       branch,
		DurationMs:   result.durationMs,
	})
}

func (s *Service) triggerBranchChangeSync(ctx context.Context, repo *model.GitRepository, id uuid.UUID, oldBranch, newBranch string) {
	logger.Info("仓库 %s 默认分支变更: %s → %s，触发自动同步", repo.Name, oldBranch, newBranch)
	s.Go(func(rootCtx context.Context) {
		asyncCtx := withTenantLifecycleContext(rootCtx, repo.TenantID)
		if err := s.SyncRepoWithTrigger(asyncCtx, id, "branch_change"); err != nil {
			logger.Sync_("GIT").Error("默认分支变更自动同步失败: %v", err)
		}
	})
}

func (s *Service) triggerPlaybookCheck(ctx context.Context, repo *model.GitRepository) {
	s.Go(func(rootCtx context.Context) {
		s.checkPlaybooksAfterSync(withTenantLifecycleContext(rootCtx, repo.TenantID), repo.ID)
	})
}

func withTenantLifecycleContext(ctx context.Context, tenantID *uuid.UUID) context.Context {
	if tenantID == nil {
		return ctx
	}
	return platformrepo.WithTenantID(ctx, *tenantID)
}

func (s *Service) checkPlaybooksAfterSync(ctx context.Context, repositoryID uuid.UUID) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Sync_("GIT").Error("checkPlaybooksAfterSync panic: %v", rec)
		}
	}()

	playbooks, err := s.playbookRepo.ListByRepositoryID(ctx, repositoryID)
	if err != nil {
		logger.Sync_("GIT").Warn("获取关联 Playbooks 失败: %v", err)
		return
	}
	if len(playbooks) == 0 {
		return
	}

	gitRepo, err := s.repo.GetByID(ctx, repositoryID)
	if err != nil {
		return
	}

	scannedCount := 0
	for _, playbook := range playbooks {
		if s.markPlaybookInvalidIfMissing(ctx, gitRepo.LocalPath, playbook) {
			continue
		}
		if !playbookNeedsAutoScan(playbook) {
			continue
		}
		if _, err := s.getPlaybookService().ScanVariables(ctx, playbook.ID, "repo_sync"); err != nil {
			logger.Sync_("GIT").Warn("Playbook %s 自动扫描失败: %v", playbook.Name, err)
		} else {
			scannedCount++
			logger.Sync_("GIT").Info("Playbook %s 自动扫描完成", playbook.Name)
		}
	}

	logger.Sync_("GIT").Info("已检查 %d 个关联 Playbooks，自动扫描 %d 个", len(playbooks), scannedCount)
}

func (s *Service) markPlaybookInvalidIfMissing(ctx context.Context, repoPath string, playbook model.Playbook) bool {
	exists, err := repoFileExists(repoPath, playbook.FilePath)
	if err != nil {
		logger.Sync_("GIT").Warn("Playbook %s 入口文件路径非法: %v", playbook.Name, err)
	} else if exists {
		return false
	}
	if err := s.playbookRepo.UpdateStatus(ctx, playbook.ID, "invalid"); err != nil {
		logger.Sync_("GIT").Error("Playbook %s 标记 invalid 失败: %v", playbook.Name, err)
		return false
	}
	logger.Sync_("GIT").Warn("Playbook %s 入口文件不存在，标记为 invalid", playbook.Name)
	return true
}

func playbookNeedsAutoScan(playbook model.Playbook) bool {
	return playbook.LastScannedAt != nil || playbook.ConfigMode != ""
}
