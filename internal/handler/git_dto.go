package handler

import (
	"github.com/company/auto-healing/internal/model"
)

// ==================== Git 仓库 DTO ====================

// CreateRepoRequest 创建仓库请求
type CreateRepoRequest struct {
	Name          string     `json:"name" binding:"required"`
	URL           string     `json:"url" binding:"required"`
	DefaultBranch string     `json:"default_branch"`
	AuthType      string     `json:"auth_type"`
	AuthConfig    model.JSON `json:"auth_config"`
	SyncEnabled   bool       `json:"sync_enabled"`
	SyncInterval  string     `json:"sync_interval"`
	MaxFailures   *int       `json:"max_failures"`
}

// ToModel 转换为 Model
func (r *CreateRepoRequest) ToModel() *model.GitRepository {
	defaultBranch := r.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	authType := r.AuthType
	if authType == "" {
		authType = "none"
	}
	syncInterval := r.SyncInterval
	if syncInterval == "" {
		syncInterval = "1h"
	}

	maxFailures := 5
	if r.MaxFailures != nil {
		maxFailures = *r.MaxFailures
	}

	return &model.GitRepository{
		Name:          r.Name,
		URL:           r.URL,
		DefaultBranch: defaultBranch,
		AuthType:      authType,
		AuthConfig:    r.AuthConfig,
		Status:        "pending",
		SyncEnabled:   r.SyncEnabled,
		SyncInterval:  syncInterval,
		MaxFailures:   maxFailures,
	}
}

// UpdateRepoRequest 更新仓库请求
type UpdateRepoRequest struct {
	DefaultBranch string     `json:"default_branch"`
	AuthType      string     `json:"auth_type"`
	AuthConfig    model.JSON `json:"auth_config"`
	SyncEnabled   *bool      `json:"sync_enabled"`
	SyncInterval  *string    `json:"sync_interval"`
	MaxFailures   *int       `json:"max_failures"`
}

// ApplyTo 应用更新到模型
func (r *UpdateRepoRequest) ApplyTo(repo *model.GitRepository) {
	if r.DefaultBranch != "" {
		repo.DefaultBranch = r.DefaultBranch
	}
	if r.AuthType != "" {
		repo.AuthType = r.AuthType
	}
	if r.AuthConfig != nil {
		repo.AuthConfig = r.AuthConfig
	}
	if r.SyncEnabled != nil {
		// 重新启用同步时，重置失败计数
		if *r.SyncEnabled && !repo.SyncEnabled {
			repo.ConsecutiveFailures = 0
			repo.PauseReason = ""
		}
		repo.SyncEnabled = *r.SyncEnabled
	}
	if r.SyncInterval != nil {
		repo.SyncInterval = *r.SyncInterval
	}
	if r.MaxFailures != nil {
		repo.MaxFailures = *r.MaxFailures
	}
}

// ValidateRepoRequest 验证仓库请求（创建前获取分支）
type ValidateRepoRequest struct {
	URL        string     `json:"url" binding:"required"`
	AuthType   string     `json:"auth_type"`
	AuthConfig model.JSON `json:"auth_config"`
}
type ActivateRepoRequest struct {
	MainPlaybook string             `json:"main_playbook"`
	ConfigMode   string             `json:"config_mode"`
	Variables    []PlaybookVariable `json:"variables,omitempty"`
}

// PlaybookVariable Playbook 变量定义
type PlaybookVariable struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Min         *int     `json:"min,omitempty"`
	Max         *int     `json:"max,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
}
