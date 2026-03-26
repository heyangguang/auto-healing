package git

import (
	"fmt"
	"time"
)

var allowedRepoAuthTypes = map[string]bool{
	"none":     true,
	"token":    true,
	"password": true,
	"ssh_key":  true,
}

func validateRepoMutation(authType string, syncEnabled bool, syncInterval string, maxFailures int) error {
	if authType == "" {
		authType = "none"
	}
	if !allowedRepoAuthTypes[authType] {
		return fmt.Errorf("不支持的认证方式: %s", authType)
	}
	if maxFailures < 0 {
		return fmt.Errorf("最大连续失败次数不能为负数")
	}
	if !syncEnabled {
		return nil
	}
	if syncInterval == "" {
		return fmt.Errorf("开启定时同步时必须提供 sync_interval")
	}
	duration, err := time.ParseDuration(syncInterval)
	if err != nil || duration <= 0 {
		return fmt.Errorf("无效的 sync_interval: %s", syncInterval)
	}
	return nil
}
