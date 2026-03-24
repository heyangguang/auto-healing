package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/secrets"
	"github.com/google/uuid"
)

// Service 密钥服务
type Service struct {
	repo *repository.SecretsSourceRepository
}

// NewService 创建密钥服务
func NewService() *Service {
	return &Service{
		repo: repository.NewSecretsSourceRepository(),
	}
}

// CreateSource 创建密钥源
func (s *Service) CreateSource(ctx context.Context, source *model.SecretsSource) (*model.SecretsSource, error) {
	// 验证配置
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

// GetSource 获取密钥源
func (s *Service) GetSource(ctx context.Context, id uuid.UUID) (*model.SecretsSource, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateSource 更新密钥源
func (s *Service) UpdateSource(ctx context.Context, id uuid.UUID, config model.JSON, isDefault *bool, priority *int, status string) (*model.SecretsSource, error) {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 被任务/调度引用中的密钥源，不允许修改实际配置，避免任务在未感知的情况下切到另一套凭据语义。
	if config != nil && !jsonEqual(config, source.Config) {
		refCount, err := s.countSourceReferences(ctx, id)
		if err != nil {
			return nil, err
		}
		if refCount > 0 {
			return nil, fmt.Errorf("无法更新：该密钥源已被 %d 个任务或调度引用，请先解除引用后再修改配置", refCount)
		}
	}

	if config != nil {
		source.Config = config
	}
	requestedDefault := false
	if isDefault != nil {
		requestedDefault = *isDefault
		source.IsDefault = *isDefault
		if requestedDefault {
			// 交给 SetDefault 事务统一收敛成单默认源
			source.IsDefault = false
		}
	}
	if priority != nil {
		if *priority < 0 {
			return nil, fmt.Errorf("优先级不能小于 0")
		}
		source.Priority = *priority
	}
	if status != "" {
		if status != "active" && status != "inactive" {
			return nil, fmt.Errorf("无效的状态: %s", status)
		}
		source.Status = status
	}

	// 验证新配置
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

func (s *Service) countSourceReferences(ctx context.Context, id uuid.UUID) (int64, error) {
	taskCount, err := s.repo.CountTasksUsingSource(ctx, id.String())
	if err != nil {
		return 0, fmt.Errorf("检查关联任务模板失败: %w", err)
	}

	scheduleCount, err := s.repo.CountSchedulesUsingSource(ctx, id.String())
	if err != nil {
		return 0, fmt.Errorf("检查关联调度任务失败: %w", err)
	}

	return taskCount + scheduleCount, nil
}

func jsonEqual(a, b model.JSON) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

// DeleteSource 删除密钥源（保护性删除）
func (s *Service) DeleteSource(ctx context.Context, id uuid.UUID) error {
	// 检查是否被任务模板的 secrets_source_ids 引用
	taskCount, err := s.repo.CountTasksUsingSource(ctx, id.String())
	if err != nil {
		return fmt.Errorf("检查关联任务模板失败: %w", err)
	}
	if taskCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个任务模板使用此密钥源，请先修改这些任务模板的密钥源配置", taskCount)
	}

	// 检查是否被调度任务的 secrets_source_ids 引用
	scheduleCount, err := s.repo.CountSchedulesUsingSource(ctx, id.String())
	if err != nil {
		return fmt.Errorf("检查关联调度任务失败: %w", err)
	}
	if scheduleCount > 0 {
		return fmt.Errorf("无法删除：有 %d 个调度任务使用此密钥源，请先修改这些调度任务的密钥源配置", scheduleCount)
	}

	return s.repo.Delete(ctx, id)
}

// ListSources 列出密钥源
func (s *Service) ListSources(ctx context.Context, sourceType, status string, isDefault *bool) ([]model.SecretsSource, error) {
	return s.repo.List(ctx, sourceType, status, isDefault)
}

// TestConnection 测试连接（更新 last_test_at）
func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) error {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	provider, err := secrets.NewProvider(source)
	if err != nil {
		_ = s.repo.UpdateTestResult(ctx, id, false)
		return err
	}

	testErr := provider.TestConnection(ctx)

	// 更新最后测试结果
	_ = s.repo.UpdateTestResult(ctx, id, testErr == nil)

	return testErr
}

// TestQuery 测试能否获取指定主机的凭据（仅作查询工具，不影响启用）
func (s *Service) TestQuery(ctx context.Context, id uuid.UUID, hostname, ipAddress string) (*model.Secret, error) {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("密钥源不存在")
	}

	provider, err := secrets.NewProvider(source)
	if err != nil {
		return nil, fmt.Errorf("创建提供者失败: %w", err)
	}

	query := model.SecretQuery{
		Hostname:  hostname,
		IPAddress: ipAddress,
		AuthType:  source.AuthType,
	}

	secret, err := provider.GetSecret(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("获取凭据失败: %w", err)
	}

	logger.Auth("SECRET").Info("测试成功: %s | 密钥源: %s", hostname, source.Name)
	return secret, nil
}

// QuerySecret 查询密钥（可选指定 source_id，不指定则使用默认密钥源）
func (s *Service) QuerySecret(ctx context.Context, query model.SecretQuery) (*model.Secret, error) {
	var source *model.SecretsSource
	var err error

	if query.SourceID != "" {
		// 指定了 source_id，使用指定的密钥源
		sourceID, parseErr := uuid.Parse(query.SourceID)
		if parseErr != nil {
			return nil, fmt.Errorf("无效的密钥源ID: %v", parseErr)
		}
		source, err = s.repo.GetByID(ctx, sourceID)
		if err != nil {
			return nil, fmt.Errorf("密钥源不存在: %v", err)
		}
		if source.Status != "active" {
			return nil, fmt.Errorf("密钥源未启用")
		}
	} else {
		// 未指定 source_id，使用默认密钥源
		source, err = s.repo.GetDefault(ctx)
		if err != nil {
			return nil, fmt.Errorf("未找到可用的默认密钥源，请先创建密钥源或指定 source_id")
		}
		logger.Auth("SECRET").Info("使用默认密钥源: %s (ID: %s)", source.Name, source.ID)
	}

	provider, err := secrets.NewProvider(source)
	if err != nil {
		return nil, err
	}

	secret, err := provider.GetSecret(ctx, query)
	if err != nil {
		logger.Auth("SECRET").Error("失败: %s | 密钥源: %s | 错误: %v", query.Hostname, source.Name, err)
		return nil, err
	}

	logger.Auth("SECRET").Info("完成: %s | 源类型: %s | 来源: %s | 认证类型: %s",
		query.Hostname, source.Type, source.Name, secret.AuthType)
	return secret, nil
}

// Enable 启用密钥源（先测试连接，通过后才启用）
func (s *Service) Enable(ctx context.Context, id uuid.UUID) error {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("密钥源不存在")
	}

	if source.Status == "active" {
		return fmt.Errorf("密钥源已经是启用状态")
	}

	// 先测试连接
	provider, err := secrets.NewProvider(source)
	if err != nil {
		return fmt.Errorf("配置错误: %w", err)
	}

	if err := provider.TestConnection(ctx); err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}

	return s.repo.UpdateStatus(ctx, id, "active")
}

// Disable 禁用密钥源
func (s *Service) Disable(ctx context.Context, id uuid.UUID) error {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("密钥源不存在")
	}

	if source.Status == "inactive" {
		return fmt.Errorf("密钥源已经是禁用状态")
	}

	return s.repo.UpdateStatus(ctx, id, "inactive")
}

// ==================== 统计 ====================

// GetStats 获取密钥源统计信息
func (s *Service) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStats(ctx)
}
