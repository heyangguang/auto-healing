package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/secrets"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) CreateSource(ctx context.Context, source *model.SecretsSource) (*model.SecretsSource, error) {
	if _, err := secrets.NewProvider(source); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}
	requestedDefault := source.IsDefault
	if requestedDefault {
		source.IsDefault = false
	}
	if err := s.repo.Create(ctx, source); err != nil {
		return nil, err
	}
	if requestedDefault {
		if err := s.repo.SetDefault(ctx, source.ID); err != nil {
			return nil, err
		}
		source.IsDefault = true
	}
	return source, nil
}

func (s *Service) GetSource(ctx context.Context, id uuid.UUID) (*model.SecretsSource, error) {
	source, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrSecretsSourceNotFound, id)
	}
	return source, err
}

func (s *Service) UpdateSource(ctx context.Context, id uuid.UUID, config model.JSON, isDefault *bool, priority *int, status string) (*model.SecretsSource, error) {
	source, err := s.GetSource(ctx, id)
	if err != nil {
		return nil, err
	}
	if config != nil && !jsonEqual(config, source.Config) {
		refCount, err := s.countSourceReferences(ctx, id)
		if err != nil {
			return nil, err
		}
		if refCount > 0 {
			return nil, fmt.Errorf("%w: 该密钥源已被 %d 个任务或调度引用，请先解除引用后再修改配置", ErrSecretsSourceInUse, refCount)
		}
		source.Config = config
	}
	requestedDefault, err := applySourceAdminChanges(source, isDefault, priority, status)
	if err != nil {
		return nil, err
	}
	if _, err := secrets.NewProvider(source); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}
	if err := s.repo.Update(ctx, source); err != nil {
		return nil, err
	}
	if requestedDefault {
		if err := s.repo.SetDefault(ctx, id); err != nil {
			return nil, err
		}
		source.IsDefault = true
	}
	return source, nil
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
			return false, fmt.Errorf("优先级不能小于 0")
		}
		source.Priority = *priority
	}
	if status != "" {
		if status != "active" && status != "inactive" {
			return false, fmt.Errorf("无效的状态: %s", status)
		}
		source.Status = status
	}
	return requestedDefault, nil
}

func (s *Service) DeleteSource(ctx context.Context, id uuid.UUID) error {
	refCount, err := s.countSourceReferences(ctx, id)
	if err != nil {
		return err
	}
	if refCount > 0 {
		return fmt.Errorf("%w: 有 %d 个任务模板或调度任务使用此密钥源，请先修改这些配置", ErrSecretsSourceInUse, refCount)
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) ListSources(ctx context.Context, sourceType, status string, isDefault *bool) ([]model.SecretsSource, error) {
	return s.repo.List(ctx, sourceType, status, isDefault)
}

func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) error {
	source, err := s.GetSource(ctx, id)
	if err != nil {
		return err
	}
	provider, err := secrets.NewProvider(source)
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
	source, err := s.GetSource(ctx, id)
	if err != nil {
		return err
	}
	if source.Status == "active" {
		return fmt.Errorf("密钥源已经是启用状态")
	}
	provider, err := secrets.NewProvider(source)
	if err != nil {
		return fmt.Errorf("配置错误: %w", err)
	}
	if err := provider.TestConnection(ctx); err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}
	return s.repo.UpdateStatus(ctx, id, "active")
}

func (s *Service) Disable(ctx context.Context, id uuid.UUID) error {
	source, err := s.GetSource(ctx, id)
	if err != nil {
		return err
	}
	if source.Status == "inactive" {
		return fmt.Errorf("密钥源已经是禁用状态")
	}
	return s.repo.UpdateStatus(ctx, id, "inactive")
}

func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}
