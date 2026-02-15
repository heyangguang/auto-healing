package model

import (
	"time"

	"github.com/google/uuid"
)

// GitRepository Git仓库模型（只负责代码同步）
type GitRepository struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name          string     `json:"name" gorm:"type:varchar(100);not null;unique"`
	URL           string     `json:"url" gorm:"column:url;type:varchar(500);not null"`
	DefaultBranch string     `json:"default_branch" gorm:"type:varchar(100);default:'main'"`
	AuthType      string     `json:"auth_type" gorm:"type:varchar(20);default:'none'"` // none, token, password, ssh_key
	AuthConfig    JSON       `json:"-" gorm:"type:jsonb"`                              // 认证配置（不输出到 JSON）
	LocalPath     string     `json:"local_path" gorm:"type:varchar(500)"`
	Branches      JSONArray  `json:"branches" gorm:"type:jsonb;default:'[]'"`
	LastSyncAt    *time.Time `json:"last_sync_at"`
	LastCommitID  string     `json:"last_commit_id" gorm:"type:varchar(40)"`           // 最后同步的 commit ID
	Status        string     `json:"status" gorm:"type:varchar(20);default:'syncing'"` // syncing / synced / error
	ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`

	// 定时同步配置
	SyncEnabled  bool       `json:"sync_enabled" gorm:"default:false"`
	SyncInterval string     `json:"sync_interval" gorm:"type:varchar(20);default:'1h'"` // 同步间隔，如 10s, 5m, 1h
	NextSyncAt   *time.Time `json:"next_sync_at"`

	// 连续失败自动暂停
	MaxFailures         int    `json:"max_failures" gorm:"default:5"`                   // 最大连续失败次数，0=不启用自动暂停
	ConsecutiveFailures int    `json:"consecutive_failures" gorm:"default:0"`           // 当前连续失败次数
	PauseReason         string `json:"pause_reason,omitempty" gorm:"type:varchar(500)"` // 自动暂停原因

	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`

	// 关联
	Playbooks []Playbook `json:"playbooks,omitempty" gorm:"foreignKey:RepositoryID"`

	// 计算字段（非持久化）
	PlaybookCount int64 `json:"playbook_count" gorm:"-"`
}

// TableName 表名
func (GitRepository) TableName() string {
	return "git_repositories"
}

// GitSyncLog Git 仓库同步日志
type GitSyncLog struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RepositoryID uuid.UUID `json:"repository_id" gorm:"type:uuid;not null"`
	TriggerType  string    `json:"trigger_type" gorm:"type:varchar(20);default:'manual'"` // manual / scheduled
	Action       string    `json:"action" gorm:"type:varchar(20);not null"`               // clone / pull
	Status       string    `json:"status" gorm:"type:varchar(20);not null"`               // success / failed
	CommitID     string    `json:"commit_id,omitempty" gorm:"type:varchar(40)"`
	Branch       string    `json:"branch,omitempty" gorm:"type:varchar(100)"`
	DurationMs   int       `json:"duration_ms,omitempty" gorm:"type:integer"`
	ErrorMessage string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at" gorm:"default:now()"`

	// 关联
	Repository *GitRepository `json:"repository,omitempty" gorm:"foreignKey:RepositoryID"`
}

// TableName 表名
func (GitSyncLog) TableName() string {
	return "git_sync_logs"
}

// TokenAuthConfig Token 认证配置
type TokenAuthConfig struct {
	Token string `json:"token"`
}

// PasswordAuthConfig 用户名密码认证配置
type PasswordAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SSHKeyAuthConfig SSH 密钥认证配置
type SSHKeyAuthConfig struct {
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase,omitempty"`
}

// VariableConfig 变量配置结构
type VariableConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // string, number, boolean, list, object
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
	Description string `json:"description"`
	SourceFile  string `json:"source_file"` // 来源文件
	SourceLine  int    `json:"source_line"` // 来源行号
	InCode      bool   `json:"in_code"`     // 是否在代码中存在
}

// Playbook Playbook 模板（管理入口文件和变量配置）
type Playbook struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RepositoryID uuid.UUID `json:"repository_id" gorm:"type:uuid;not null"`
	Name         string    `json:"name" gorm:"type:varchar(200);not null"`
	Description  string    `json:"description,omitempty" gorm:"type:text"`
	FilePath     string    `json:"file_path" gorm:"type:varchar(500);not null"` // 主入口文件

	// 变量配置
	Variables        JSONArray  `json:"variables" gorm:"type:jsonb;default:'[]'"`         // 用户确认的变量配置
	ScannedVariables JSONArray  `json:"scanned_variables" gorm:"type:jsonb;default:'[]'"` // 自动扫描的原始变量
	LastScannedAt    *time.Time `json:"last_scanned_at"`
	ConfigMode       string     `json:"config_mode" gorm:"type:varchar(20)"` // auto / enhanced，首次扫描时设置

	// 状态: pending / ready / error / invalid
	Status string `json:"status" gorm:"type:varchar(20);default:'pending'"`

	// 其他配置
	Tags                  JSONArray  `json:"tags" gorm:"type:jsonb;default:'[]'"`
	DefaultExtraVars      JSON       `json:"default_extra_vars" gorm:"type:jsonb;default:'{}'"`
	DefaultTimeoutMinutes int        `json:"default_timeout_minutes" gorm:"default:60"`
	CreatedBy             *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt             time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	Repository *GitRepository `json:"repository,omitempty" gorm:"foreignKey:RepositoryID"`
}

// TableName 表名
func (Playbook) TableName() string {
	return "playbooks"
}

// PlaybookScanLog Playbook 扫描日志
type PlaybookScanLog struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PlaybookID  uuid.UUID `json:"playbook_id" gorm:"type:uuid;not null"`
	TriggerType string    `json:"trigger_type" gorm:"type:varchar(20);not null"` // manual / repo_sync

	// 扫描统计
	FilesScanned   int `json:"files_scanned" gorm:"default:0"`
	VariablesFound int `json:"variables_found" gorm:"default:0"`
	NewCount       int `json:"new_count" gorm:"default:0"`
	RemovedCount   int `json:"removed_count" gorm:"default:0"`

	// 详细变更
	Details   JSON      `json:"details" gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`

	// 关联
	Playbook *Playbook `json:"playbook,omitempty" gorm:"foreignKey:PlaybookID"`
}

// TableName 表名
func (PlaybookScanLog) TableName() string {
	return "playbook_scan_logs"
}

// ExecutionTask 执行任务模板
// 通过 PlaybookID 关联 Playbook，再通过 Playbook 获取仓库
type ExecutionTask struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name               string     `json:"name" gorm:"type:varchar(200)"`             // 任务名称
	PlaybookID         uuid.UUID  `json:"playbook_id" gorm:"type:uuid;not null"`     // 关联的 Playbook
	WorkflowInstanceID *uuid.UUID `json:"workflow_instance_id,omitempty" gorm:"-"`   // 临时字段，不存储
	NodeExecutionID    *uuid.UUID `json:"node_execution_id,omitempty" gorm:"-"`      // 临时字段，不存储
	TargetHosts        string     `json:"target_hosts" gorm:"type:text;not null"`    // 目标主机
	ExtraVars          JSON       `json:"extra_vars" gorm:"type:jsonb;default:'{}'"` // 变量值

	// 执行器配置
	ExecutorType     string      `json:"executor_type" gorm:"type:varchar(20);default:'local'"` // local / docker
	Description      string      `json:"description" gorm:"type:text"`                          // 任务描述
	SecretsSourceIDs StringArray `json:"secrets_source_ids" gorm:"type:jsonb;default:'[]'"`     // 关联的密钥源 ID 列表

	// 通知配置
	NotificationConfig *TaskNotificationConfig `json:"notification_config,omitempty" gorm:"type:jsonb"`

	// Playbook 变量变更检测
	PlaybookVariablesSnapshot JSONArray `json:"playbook_variables_snapshot" gorm:"type:jsonb;default:'[]'"` // Playbook 变量快照
	NeedsReview               bool      `json:"needs_review" gorm:"default:false"`                          // 是否需要审核
	ChangedVariables          JSONArray `json:"changed_variables" gorm:"type:jsonb;default:'[]'"`           // 变更的变量名列表

	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:now()"`

	// 关联
	Playbook      *Playbook      `json:"playbook,omitempty" gorm:"foreignKey:PlaybookID"`
	Runs          []ExecutionRun `json:"runs,omitempty" gorm:"foreignKey:TaskID"`
	ScheduleCount int            `json:"schedule_count" gorm:"-"` // 关联的定时任务数量（非持久化，查询时填充）
}

// TableName 表名
func (ExecutionTask) TableName() string {
	return "execution_tasks"
}
