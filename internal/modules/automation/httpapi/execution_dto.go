package httpapi

import (
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
)

// ==================== 执行任务 DTO ====================

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name               string                        `json:"name"`
	PlaybookID         uuid.UUID                     `json:"playbook_id"`
	TargetHosts        string                        `json:"target_hosts"`
	ExtraVars          map[string]any                `json:"extra_vars"`
	ExecutorType       string                        `json:"executor_type"`
	NotificationConfig *model.TaskNotificationConfig `json:"notification_config"`
	Description        string                        `json:"description"`
	SecretsSourceIDs   []uuid.UUID                   `json:"secrets_source_ids"`
}

// ToModel 转换为 Model
func (r *CreateTaskRequest) ToModel() *model.ExecutionTask {
	executorType := r.ExecutorType
	if executorType == "" {
		executorType = "local"
	}

	// 转换 SecretsSourceIDs 为 StringArray
	var secretIDs model.StringArray
	if len(r.SecretsSourceIDs) > 0 {
		secretIDs = make(model.StringArray, len(r.SecretsSourceIDs))
		for i, id := range r.SecretsSourceIDs {
			secretIDs[i] = id.String()
		}
	} else {
		secretIDs = model.StringArray{}
	}

	return &model.ExecutionTask{
		Name:               r.Name,
		PlaybookID:         r.PlaybookID,
		TargetHosts:        r.TargetHosts,
		ExtraVars:          model.JSON(r.ExtraVars),
		ExecutorType:       executorType,
		NotificationConfig: r.NotificationConfig,
		Description:        r.Description,
		SecretsSourceIDs:   secretIDs,
	}
}

// ExecuteTaskRequest 执行任务请求
type ExecuteTaskRequest struct {
	TriggeredBy      string         `json:"triggered_by"`
	SecretsSourceID  *uuid.UUID     `json:"secrets_source_id"`
	SecretsSourceIDs []uuid.UUID    `json:"secrets_source_ids"`
	ExtraVars        map[string]any `json:"extra_vars"`
	TargetHosts      string         `json:"target_hosts"`      // 覆盖目标主机
	SkipNotification bool           `json:"skip_notification"` // 跳过本次通知（全局）
}

// GetSecretsSourceIDs 获取密钥源ID列表（兼容处理）
func (r *ExecuteTaskRequest) GetSecretsSourceIDs() []uuid.UUID {
	if len(r.SecretsSourceIDs) > 0 {
		return r.SecretsSourceIDs
	}
	if r.SecretsSourceID != nil {
		return []uuid.UUID{*r.SecretsSourceID}
	}
	return nil
}

// GetTriggeredBy 获取触发者
func (r *ExecuteTaskRequest) GetTriggeredBy() string {
	if r.TriggeredBy != "" {
		return r.TriggeredBy
	}
	return "manual"
}
