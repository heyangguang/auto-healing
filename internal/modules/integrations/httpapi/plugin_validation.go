package httpapi

import (
	"fmt"

	"github.com/company/auto-healing/internal/model"
)

func validatePluginCreateRequest(req *CreatePluginRequest) error {
	if req.Type != "itsm" && req.Type != "cmdb" {
		return fmt.Errorf("不支持的插件类型: %s", req.Type)
	}
	return validatePluginSyncConstraints(req.SyncEnabled, req.SyncIntervalMinutes, req.MaxFailures)
}

func validatePluginUpdateRequest(current *model.Plugin, req *UpdatePluginRequest) error {
	syncEnabled := current.SyncEnabled
	if req.SyncEnabled != nil {
		syncEnabled = *req.SyncEnabled
	}

	syncInterval := current.SyncIntervalMinutes
	if req.SyncIntervalMinutes != nil {
		syncInterval = *req.SyncIntervalMinutes
	}
	return validatePluginSyncConstraints(syncEnabled, syncInterval, req.MaxFailures)
}

func validatePluginSyncConstraints(syncEnabled bool, syncInterval int, maxFailures *int) error {
	if syncEnabled && syncInterval < 1 {
		return fmt.Errorf("同步间隔最小为1分钟")
	}
	return validateNonNegativeMaxFailures(maxFailures)
}
