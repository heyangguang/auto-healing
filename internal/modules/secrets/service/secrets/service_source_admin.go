package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	secretsapi "github.com/company/auto-healing/internal/modules/secrets/providerapi"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) CreateSource(ctx context.Context, source *model.SecretsSource) (*model.SecretsSource, error) {
	if _, err := secretsapi.NewProvider(source); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}
	requestedDefault := source.IsDefault
	var finalSource *model.SecretsSource
	if requestedDefault {
		source.IsDefault = false
	}
	if err := s.repo.Transaction(ctx, func(repoTx *secretsrepo.SecretsSourceRepository) error {
		if err := repoTx.Create(ctx, source); err != nil {
			return err
		}
		if requestedDefault {
			if err := repoTx.SetDefault(ctx, source.ID); err != nil {
				return err
			}
		}
		if err := repoTx.EnsureActiveDefault(ctx); err != nil {
			return err
		}
		refreshedSource, err := repoTx.GetByID(ctx, source.ID)
		if err != nil {
			return err
		}
		finalSource = refreshedSource
		return nil
	}); err != nil {
		return nil, err
	}
	return finalSource, nil
}

func (s *Service) GetSource(ctx context.Context, id uuid.UUID) (*model.SecretsSource, error) {
	source, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
	}
	return source, err
}

func (s *Service) UpdateSource(ctx context.Context, id uuid.UUID, config model.JSON, isDefault *bool, priority *int, status string) (*model.SecretsSource, error) {
	var finalSource *model.SecretsSource
	if err := s.repo.Transaction(ctx, func(repoTx *secretsrepo.SecretsSourceRepository) error {
		source, err := repoTx.GetByIDForUpdate(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
			}
			return err
		}
		configChanged := config != nil && !jsonEqual(config, source.Config)
		if configChanged {
			source.Config = config
		}
		requestedDefault, err := applySourceAdminChanges(source, isDefault, priority, status)
		if err != nil {
			return err
		}
		if _, err := secretsapi.NewProvider(source); err != nil {
			return fmt.Errorf("配置验证失败: %w", err)
		}
		if configChanged {
			refCount, err := countSourceReferencesWithRepo(repoTx, ctx, id)
			if err != nil {
				return err
			}
			if refCount > 0 {
				return fmt.Errorf("%w: 该密钥源已被 %d 个任务或调度引用，请先解除引用后再修改配置", ErrSecretsSourceInUse, refCount)
			}
		}
		if err := repoTx.Update(ctx, source); err != nil {
			return err
		}
		if requestedDefault {
			if err := repoTx.SetDefault(ctx, id); err != nil {
				return err
			}
		}
		if err := repoTx.EnsureActiveDefault(ctx); err != nil {
			return err
		}
		refreshedSource, err := repoTx.GetByID(ctx, id)
		if err != nil {
			return err
		}
		finalSource = refreshedSource
		return nil
	}); err != nil {
		return nil, err
	}
	return finalSource, nil
}

func applySourceAdminChanges(source *model.SecretsSource, isDefault *bool, priority *int, status string) (bool, error) {
	requestedDefault := false
	if isDefault != nil {
		requestedDefault = *isDefault
		source.IsDefault = *isDefault
		if requestedDefault {
			source.IsDefault = false
		}
	}
	if priority != nil {
		if *priority < 0 {
			return false, fmt.Errorf("%w: 优先级不能小于 0", ErrSecretsSourceInvalidInput)
		}
		source.Priority = *priority
	}
	if status != "" {
		if status != "active" && status != "inactive" {
			return false, fmt.Errorf("%w: 无效的状态: %s", ErrSecretsSourceInvalidInput, status)
		}
		source.Status = status
	}
	if requestedDefault && source.Status != "active" {
		return false, fmt.Errorf("%w: 默认密钥源必须为启用状态", ErrDefaultSourceMustBeActive)
	}
	if source.Status != "active" {
		source.IsDefault = false
	}
	return requestedDefault, nil
}

func (s *Service) DeleteSource(ctx context.Context, id uuid.UUID) error {
	return s.repo.Transaction(ctx, func(repoTx *secretsrepo.SecretsSourceRepository) error {
		if _, err := repoTx.GetByID(ctx, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
			}
			return err
		}
		refCount, err := countSourceReferencesWithRepo(repoTx, ctx, id)
		if err != nil {
			return err
		}
		if refCount > 0 {
			return fmt.Errorf("%w: 有 %d 个任务模板或调度任务使用此密钥源，请先修改这些配置", ErrSecretsSourceInUse, refCount)
		}
		if err := repoTx.Delete(ctx, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
			}
			return err
		}
		return repoTx.EnsureActiveDefault(ctx)
	})
}

func (s *Service) ListSources(ctx context.Context, sourceType, status string, isDefault *bool) ([]model.SecretsSource, error) {
	return s.repo.List(ctx, sourceType, status, isDefault)
}

func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) error {
	source, err := s.GetSource(ctx, id)
	if err != nil {
		return err
	}
	provider, err := secretsapi.NewProvider(source)
	if err != nil {
		return s.persistTestConnectionResult(ctx, id, false, err)
	}
	testErr := provider.TestConnection(ctx)
	return s.persistTestConnectionResult(ctx, id, testErr == nil, testErr)
}

func (s *Service) persistTestConnectionResult(ctx context.Context, id uuid.UUID, success bool, baseErr error) error {
	if err := s.repo.UpdateTestResult(ctx, id, success); err != nil {
		if baseErr != nil {
			return errors.Join(baseErr, fmt.Errorf("更新测试结果失败: %w", err))
		}
		return fmt.Errorf("更新测试结果失败: %w", err)
	}
	return baseErr
}

func (s *Service) Enable(ctx context.Context, id uuid.UUID) error {
	return s.repo.Transaction(ctx, func(repoTx *secretsrepo.SecretsSourceRepository) error {
		source, err := repoTx.GetByIDForUpdate(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
			}
			return err
		}
		if source.Status == "active" {
			return fmt.Errorf("%w: %s", ErrSecretsSourceAlreadyActive, source.Name)
		}
		provider, err := secretsapi.NewProvider(source)
		if err != nil {
			return fmt.Errorf("配置错误: %w", err)
		}
		if err := provider.TestConnection(ctx); err != nil {
			return fmt.Errorf("连接测试失败: %w", err)
		}
		if err := repoTx.UpdateStatus(ctx, id, "active"); err != nil {
			return err
		}
		return repoTx.EnsureActiveDefault(ctx)
	})
}

func (s *Service) Disable(ctx context.Context, id uuid.UUID) error {
	return s.repo.Transaction(ctx, func(repoTx *secretsrepo.SecretsSourceRepository) error {
		source, err := repoTx.GetByIDForUpdate(ctx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
			}
			return err
		}
		if source.Status == "inactive" {
			return fmt.Errorf("%w: %s", ErrSecretsSourceAlreadyInactive, source.Name)
		}
		source.Status = "inactive"
		source.IsDefault = false
		if err := repoTx.Update(ctx, source); err != nil {
			return err
		}
		return repoTx.EnsureActiveDefault(ctx)
	})
}

func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}

func countSourceReferencesWithRepo(repo *secretsrepo.SecretsSourceRepository, ctx context.Context, id uuid.UUID) (int64, error) {
	taskCount, err := repo.CountTasksUsingSource(ctx, id.String())
	if err != nil {
		return 0, fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	scheduleCount, err := repo.CountSchedulesUsingSource(ctx, id.String())
	if err != nil {
		return 0, fmt.Errorf("检查关联调度任务失败: %w", err)
	}
	return taskCount + scheduleCount, nil
}
