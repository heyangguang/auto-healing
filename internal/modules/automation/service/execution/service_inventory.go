package execution

import (
	"context"
	"fmt"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/secrets"
	"github.com/google/uuid"
)

type sourceProvider struct {
	source   *model.SecretsSource
	provider secrets.Provider
}

func (s *Service) prepareRunInventory(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, workDir string, params *executeParams) (string, error) {
	if len(params.secretsSourceIDs) == 0 {
		return s.prepareBasicInventory(ctx, runID, workDir, params.targetHosts)
	}

	providers := s.loadSecretProviders(ctx, runID, params.secretsSourceIDs)
	if len(providers) == 0 {
		s.finalizeRunFailure(ctx, runID, "没有可用的密钥源", nil)
		s.appendDetachedLog(ctx, runID, "error", "prepare", "没有可用的密钥源", nil)
		return "", fmt.Errorf("没有可用的密钥源")
	}
	return s.prepareAuthenticatedInventory(ctx, runID, task, workDir, params.targetHosts, providers)
}

func (s *Service) loadSecretProviders(ctx context.Context, runID uuid.UUID, sourceIDs []uuid.UUID) []sourceProvider {
	providers := make([]sourceProvider, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		source, err := s.secretsRepo.GetByID(ctx, sourceID)
		if err != nil {
			s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("获取密钥源 %s 失败: %v", sourceID, err), nil)
			continue
		}
		if source.Status != "active" {
			s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("密钥源 %s 未启用，已跳过", source.Name), nil)
			continue
		}

		provider, err := secrets.NewProvider(source)
		if err != nil {
			s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("创建密钥提供者失败: %v", err), nil)
			continue
		}

		providers = append(providers, sourceProvider{source: source, provider: provider})
		s.appendLog(ctx, runID, "info", "prepare", fmt.Sprintf("使用密钥源: %s (类型: %s, 认证: %s)", source.Name, source.Type, source.AuthType), nil)
	}
	return providers
}

func (s *Service) prepareAuthenticatedInventory(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, workDir, targetHosts string, providers []sourceProvider) (string, error) {
	credentials, err := s.buildHostCredentials(ctx, runID, task, workDir, targetHosts, providers)
	if err != nil {
		s.finalizeRunFailure(ctx, runID, err.Error(), nil)
		s.appendDetachedLog(ctx, runID, "error", "prepare", fmt.Sprintf("构建主机凭据失败: %v", err), nil)
		return "", err
	}

	inventoryPath, err := ansible.WriteInventoryFile(workDir, ansible.GenerateInventoryWithAuth(credentials, "targets"))
	if err != nil {
		s.finalizeRunFailure(ctx, runID, err.Error(), nil)
		s.appendDetachedLog(ctx, runID, "error", "prepare", fmt.Sprintf("生成 inventory 失败: %v", err), nil)
		return "", err
	}
	s.appendLog(ctx, runID, "info", "prepare", fmt.Sprintf("Inventory 已生成（含 %d 台主机认证信息）", len(credentials)), nil)
	return inventoryPath, nil
}

func (s *Service) buildHostCredentials(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, workDir, targetHosts string, providers []sourceProvider) ([]ansible.HostCredential, error) {
	hosts := strings.Split(targetHosts, ",")
	credentials := make([]ansible.HostCredential, 0, len(hosts))
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}

		credential, err := s.resolveHostCredential(ctx, runID, task, workDir, host, providers)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

func (s *Service) resolveHostCredential(ctx context.Context, runID uuid.UUID, task *model.ExecutionTask, workDir, host string, providers []sourceProvider) (ansible.HostCredential, error) {
	for _, sp := range providers {
		query := s.buildSecretQuery(ctx, host, sp.source.AuthType)
		secret, err := sp.provider.GetSecret(ctx, query)
		if err != nil {
			if err == secrets.ErrSecretNotFound {
				continue
			}
			return ansible.HostCredential{}, fmt.Errorf("查询主机 %s 的密钥源 %s 失败: %w", host, sp.source.Name, err)
		}

		credential, err := s.buildCredentialFromSecret(task, workDir, host, secret)
		if err != nil {
			return ansible.HostCredential{}, err
		}
		s.appendLog(ctx, runID, "debug", "prepare", fmt.Sprintf("主机 %s 使用密钥源 %s (%s)", host, sp.source.Name, sp.source.AuthType), nil)
		return credential, nil
	}

	s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("主机 %s 在所有密钥源中都未找到凭据，将使用默认认证", host), nil)
	return ansible.HostCredential{Host: host}, nil
}

func (s *Service) buildSecretQuery(ctx context.Context, host, authType string) model.SecretQuery {
	ipAddress := host
	hostname := host

	if cmdbItem, err := s.cmdbRepo.FindByNameOrIP(ctx, host); err == nil {
		ipAddress = cmdbItem.IPAddress
		hostname = cmdbItem.Hostname
		if hostname == "" {
			hostname = cmdbItem.Name
		}
	}

	return model.SecretQuery{
		Hostname:  hostname,
		IPAddress: ipAddress,
		AuthType:  authType,
	}
}

func (s *Service) buildCredentialFromSecret(task *model.ExecutionTask, workDir, host string, secret *model.Secret) (ansible.HostCredential, error) {
	credential := ansible.HostCredential{
		Host:     host,
		AuthType: secret.AuthType,
		Username: secret.Username,
	}

	switch {
	case secret.AuthType == "ssh_key" && secret.PrivateKey != "":
		keyFileName := fmt.Sprintf("key_%s", strings.ReplaceAll(host, ".", "_"))
		keyPath, err := ansible.WriteKeyFile(workDir, keyFileName, secret.PrivateKey)
		if err != nil {
			return ansible.HostCredential{}, err
		}
		if task.ExecutorType == "docker" {
			credential.KeyFile = "/workspace/" + keyFileName
		} else {
			credential.KeyFile = keyPath
		}
	case secret.AuthType == "password":
		credential.Password = secret.Password
	}

	return credential, nil
}

func (s *Service) prepareBasicInventory(ctx context.Context, runID uuid.UUID, workDir, targetHosts string) (string, error) {
	inventoryPath, err := ansible.WriteInventoryFile(workDir, ansible.GenerateInventory(targetHosts, "targets", nil))
	if err != nil {
		s.finalizeRunFailure(ctx, runID, err.Error(), nil)
		s.appendDetachedLog(ctx, runID, "error", "prepare", fmt.Sprintf("生成 inventory 失败: %v", err), nil)
		return "", err
	}
	s.appendLog(ctx, runID, "info", "prepare", "Inventory 已生成", nil)
	return inventoryPath, nil
}
