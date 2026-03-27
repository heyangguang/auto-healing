package model

import (
	"time"

	"github.com/company/auto-healing/internal/platform/modeltypes"
	"github.com/google/uuid"
)

type GitRepository struct {
	ID                  uuid.UUID            `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID            *uuid.UUID           `json:"tenant_id,omitempty" gorm:"type:uuid;uniqueIndex:idx_git_repo_tenant_name"`
	Name                string               `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_git_repo_tenant_name"`
	URL                 string               `json:"url" gorm:"column:url;type:varchar(500);not null"`
	DefaultBranch       string               `json:"default_branch" gorm:"type:varchar(100);default:'main'"`
	AuthType            string               `json:"auth_type" gorm:"type:varchar(20);default:'none'"`
	AuthConfig          modeltypes.JSON      `json:"-" gorm:"type:jsonb"`
	LocalPath           string               `json:"local_path" gorm:"type:varchar(500)"`
	Branches            modeltypes.JSONArray `json:"branches" gorm:"type:jsonb;default:'[]'"`
	LastSyncAt          *time.Time           `json:"last_sync_at"`
	LastCommitID        string               `json:"last_commit_id" gorm:"type:varchar(40)"`
	Status              string               `json:"status" gorm:"type:varchar(20);default:'syncing'"`
	ErrorMessage        string               `json:"error_message,omitempty" gorm:"type:text"`
	SyncEnabled         bool                 `json:"sync_enabled" gorm:"default:false"`
	SyncInterval        string               `json:"sync_interval" gorm:"type:varchar(20);default:'1h'"`
	NextSyncAt          *time.Time           `json:"next_sync_at"`
	MaxFailures         int                  `json:"max_failures" gorm:"default:5"`
	ConsecutiveFailures int                  `json:"consecutive_failures" gorm:"default:0"`
	PauseReason         string               `json:"pause_reason,omitempty" gorm:"type:varchar(500)"`
	CreatedAt           time.Time            `json:"created_at" gorm:"default:now()"`
	UpdatedAt           time.Time            `json:"updated_at" gorm:"default:now()"`

	Playbooks     []Playbook `json:"playbooks,omitempty" gorm:"foreignKey:RepositoryID"`
	PlaybookCount int64      `json:"playbook_count" gorm:"-"`
}

func (GitRepository) TableName() string {
	return "git_repositories"
}

type GitSyncLog struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID     *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	RepositoryID uuid.UUID  `json:"repository_id" gorm:"type:uuid;not null"`
	TriggerType  string     `json:"trigger_type" gorm:"type:varchar(20);default:'manual'"`
	Action       string     `json:"action" gorm:"type:varchar(20);not null"`
	Status       string     `json:"status" gorm:"type:varchar(20);not null"`
	CommitID     string     `json:"commit_id,omitempty" gorm:"type:varchar(40)"`
	Branch       string     `json:"branch,omitempty" gorm:"type:varchar(100)"`
	DurationMs   int        `json:"duration_ms,omitempty" gorm:"type:integer"`
	ErrorMessage string     `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time  `json:"created_at" gorm:"default:now()"`

	Repository *GitRepository `json:"repository,omitempty" gorm:"foreignKey:RepositoryID"`
}

func (GitSyncLog) TableName() string {
	return "git_sync_logs"
}

type TokenAuthConfig struct {
	Token string `json:"token"`
}

type PasswordAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SSHKeyAuthConfig struct {
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase,omitempty"`
}
