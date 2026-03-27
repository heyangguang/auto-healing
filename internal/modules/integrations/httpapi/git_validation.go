package httpapi

import (
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

var allowedGitAuthTypes = map[string]bool{
	"none":     true,
	"token":    true,
	"password": true,
	"ssh_key":  true,
}

func validateGitCreateRequest(req *CreateRepoRequest) error {
	if err := validateGitAuthType(req.AuthType); err != nil {
		return err
	}
	syncInterval := req.SyncInterval
	if syncInterval == "" {
		syncInterval = "1h"
	}
	if err := validateGitSyncConfig(req.SyncEnabled, syncInterval); err != nil {
		return err
	}
	return validateGitMaxFailures(req.MaxFailures)
}

func validateGitUpdateRequest(current *model.GitRepository, req *UpdateRepoRequest) error {
	authType := current.AuthType
	if req.AuthType != "" {
		authType = req.AuthType
	}
	if err := validateGitAuthType(authType); err != nil {
		return err
	}

	syncEnabled := current.SyncEnabled
	if req.SyncEnabled != nil {
		syncEnabled = *req.SyncEnabled
	}

	syncInterval := current.SyncInterval
	if req.SyncInterval != nil && *req.SyncInterval != "" {
		syncInterval = *req.SyncInterval
	}

	if err := validateGitSyncConfig(syncEnabled, syncInterval); err != nil {
		return err
	}
	return validateGitMaxFailures(req.MaxFailures)
}

func validateGitAuthType(authType string) error {
	if authType == "" {
		return nil
	}
	if !allowedGitAuthTypes[authType] {
		return fmt.Errorf("不支持的认证方式: %s", authType)
	}
	return nil
}

func validateGitSyncConfig(enabled bool, interval string) error {
	if !enabled {
		return nil
	}
	if interval == "" {
		return fmt.Errorf("开启定时同步时必须提供 sync_interval")
	}
	if _, err := time.ParseDuration(interval); err != nil {
		return fmt.Errorf("无效的 sync_interval: %w", err)
	}
	return nil
}

func validateGitMaxFailures(maxFailures *int) error {
	if maxFailures != nil && *maxFailures < 0 {
		return fmt.Errorf("最大连续失败次数不能为负数")
	}
	return nil
}
