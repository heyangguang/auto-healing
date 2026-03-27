package httpapi

import (
	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/platform/modeltypes"
)

// ==================== Secrets Source DTO ====================

// CreateSourceRequest 创建密钥源请求
type CreateSourceRequest struct {
	Name      string          `json:"name" binding:"required"`
	Type      string          `json:"type" binding:"required"`
	AuthType  string          `json:"auth_type" binding:"required"`
	Config    modeltypes.JSON `json:"config" binding:"required"`
	IsDefault bool            `json:"is_default"`
	Priority  int             `json:"priority"`
}

// ToModel 转换为 Model
func (r *CreateSourceRequest) ToModel() *secretsmodel.SecretsSource {
	return &secretsmodel.SecretsSource{
		Name:      r.Name,
		Type:      r.Type,
		AuthType:  r.AuthType,
		Config:    r.Config,
		IsDefault: r.IsDefault,
		Priority:  r.Priority,
		Status:    "active",
	}
}

// UpdateSourceRequest 更新密钥源请求
type UpdateSourceRequest struct {
	Config    modeltypes.JSON `json:"config"`
	IsDefault *bool           `json:"is_default"`
	Priority  *int            `json:"priority"`
	Status    string          `json:"status"`
}

// ApplyTo 应用更新到模型
func (r *UpdateSourceRequest) ApplyTo(source *secretsmodel.SecretsSource) {
	if r.Config != nil {
		source.Config = r.Config
	}
	if r.IsDefault != nil {
		source.IsDefault = *r.IsDefault
	}
	if r.Priority != nil {
		source.Priority = *r.Priority
	}
	if r.Status != "" {
		source.Status = r.Status
	}
}
