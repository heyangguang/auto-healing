package secretsources

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	secretsrepo "github.com/company/auto-healing/internal/modules/secrets/repository"
	"github.com/google/uuid"
)

func ParseStringArray(ids model.StringArray, field string) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	result := make([]uuid.UUID, 0, len(ids))
	for _, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("%s 包含非法 UUID: %s", field, idStr)
		}
		result = append(result, id)
	}
	return result, nil
}

func ValidateActive(ctx context.Context, repo *secretsrepo.SecretsSourceRepository, ids []uuid.UUID) error {
	if repo == nil || len(ids) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return fmt.Errorf("密钥源 ID 不能为空")
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("密钥源重复: %s", id)
		}
		seen[id] = struct{}{}
		source, err := repo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("密钥源不存在: %s", id)
		}
		if source.Status != "active" {
			return fmt.Errorf("密钥源未启用: %s", source.Name)
		}
	}
	return nil
}

func ValidateStringArray(ctx context.Context, repo *secretsrepo.SecretsSourceRepository, ids model.StringArray, field string) error {
	parsed, err := ParseStringArray(ids, field)
	if err != nil {
		return err
	}
	return ValidateActive(ctx, repo, parsed)
}
