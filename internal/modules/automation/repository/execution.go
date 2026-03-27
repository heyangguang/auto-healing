package repository

import (
	"context"
	"time"

	qf "github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExecutionRepository 执行任务仓库
type ExecutionRepository struct {
	db *gorm.DB
}

// TaskListOptions 任务列表筛选选项
type TaskListOptions struct {
	PlaybookID     *uuid.UUID
	Name           qf.StringFilter
	Description    qf.StringFilter
	ExecutorType   string
	Status         string
	NeedsReview    *bool
	TargetHosts    string
	PlaybookName   string
	RepositoryName string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	SortBy         string
	SortOrder      string
	HasRuns        *bool
	MinRunCount    *int
	LastRunStatus  string
	Page           int
	PageSize       int
}

// RunListOptions 执行记录列表筛选选项
type RunListOptions struct {
	TaskID        *uuid.UUID
	RunID         string
	TaskName      qf.StringFilter
	Status        string
	TriggeredBy   string
	StartedAfter  *time.Time
	StartedBefore *time.Time
	Page          int
	PageSize      int
}

func NewExecutionRepositoryWithDB(db *gorm.DB) *ExecutionRepository {
	return &ExecutionRepository{db: db}
}

func (r *ExecutionRepository) tenantDB(ctx context.Context) *gorm.DB {
	return TenantDB(r.db, ctx)
}

func countWithSession(query *gorm.DB) (int64, error) {
	var total int64
	err := query.Session(&gorm.Session{}).Count(&total).Error
	return total, err
}
