package model

import (
	"time"

	"github.com/google/uuid"
)

// SecretsSource 密钥源模型
type SecretsSource struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name           string     `json:"name" gorm:"type:varchar(100);not null;unique"`
	Type           string     `json:"type" gorm:"type:varchar(20);not null"`      // vault, file, webhook
	AuthType       string     `json:"auth_type" gorm:"type:varchar(20);not null"` // ssh_key, password
	Config         JSON       `json:"config" gorm:"type:jsonb;not null"`          // 连接配置
	IsDefault      bool       `json:"is_default" gorm:"default:false"`            // 是否默认密钥源
	Priority       int        `json:"priority" gorm:"default:0"`                  // 优先级
	Status         string     `json:"status" gorm:"type:varchar(20);default:'inactive'"`
	LastTestAt     *time.Time `json:"last_test_at"`     // 最后测试时间
	LastTestResult *bool      `json:"last_test_result"` // 最后测试结果
	CreatedAt      time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"default:now()"`
}

// TableName 表名
func (SecretsSource) TableName() string {
	return "secrets_sources"
}

// VaultConfig Vault 配置
type VaultConfig struct {
	Address      string       `json:"address"`                 // Vault 地址
	SecretPath   string       `json:"secret_path"`             // 密钥基础路径
	Namespace    string       `json:"namespace"`               // 命名空间（可选）
	Auth         VaultAuth    `json:"auth"`                    // 认证配置
	QueryKey     string       `json:"query_key"`               // 查询键：ip 或 hostname
	FieldMapping FieldMapping `json:"field_mapping,omitempty"` // 字段路径映射
}

// VaultAuth Vault 认证配置
type VaultAuth struct {
	Type     string `json:"type"`      // token, approle
	Token    string `json:"token"`     // Token 认证
	RoleID   string `json:"role_id"`   // AppRole 认证
	SecretID string `json:"secret_id"` // AppRole 认证
}

// FileConfig 文件密钥源配置（只支持 ssh_key）
type FileConfig struct {
	KeyPath  string `json:"key_path"` // 密钥文件路径
	Username string `json:"username"` // SSH 用户名，默认 root
}

// WebhookConfig Webhook 密钥源配置
type WebhookConfig struct {
	URL              string       `json:"url"`                          // Webhook 基础 URL
	Method           string       `json:"method"`                       // HTTP 方法
	Auth             WebhookAuth  `json:"auth"`                         // 认证配置
	QueryKey         string       `json:"query_key"`                    // 查询键：ip 或 hostname
	Timeout          int          `json:"timeout"`                      // 超时时间（秒）
	ResponseDataPath string       `json:"response_data_path,omitempty"` // 响应数据根路径
	FieldMapping     FieldMapping `json:"field_mapping,omitempty"`      // 字段路径映射
}

// FieldMapping 字段路径映射
type FieldMapping struct {
	Username   string `json:"username"`    // username 字段路径，默认 "username"
	Password   string `json:"password"`    // password 字段路径，默认 "password"
	PrivateKey string `json:"private_key"` // private_key 字段路径，默认 "private_key"
}

// WebhookAuth Webhook 认证配置
type WebhookAuth struct {
	Type       string `json:"type"`        // none, basic, bearer, api_key
	Username   string `json:"username"`    // basic
	Password   string `json:"password"`    // basic
	Token      string `json:"token"`       // bearer
	HeaderName string `json:"header_name"` // api_key
	APIKey     string `json:"api_key"`     // api_key
}

// SecretQuery 密钥查询请求
type SecretQuery struct {
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ip_address"`
	AuthType  string `json:"auth_type"`           // ssh_key, password
	SourceID  string `json:"source_id,omitempty"` // 可选，指定密钥源ID
}

// Secret 密钥结果（统一返回结构）
type Secret struct {
	AuthType   string `json:"auth_type"`             // "ssh_key" 或 "password"
	Username   string `json:"username"`              // SSH 用户名
	PrivateKey string `json:"private_key,omitempty"` // 私钥内容（密钥方式）
	Password   string `json:"password,omitempty"`    // 密码（密码方式）
}
