package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateLog 创建通知日志
func (r *NotificationRepository) CreateLog(ctx context.Context, log *model.NotificationLog) error {
	if err := FillTenantID(ctx, &log.TenantID); err != nil {
		return err
	}
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(log).Error
}

// GetLogByID 根据 ID 获取日志
func (r *NotificationRepository) GetLogByID(ctx context.Context, id uuid.UUID) (*model.NotificationLog, error) {
	var log model.NotificationLog
	queryBuilder, err := r.notificationLogsBaseQuery(ctx)
	if err != nil {
		return nil, err
	}
	err = r.preloadNotificationLogs(ctx, queryBuilder).Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// ListLogs 获取日志列表
func (r *NotificationRepository) ListLogs(ctx context.Context, opts *NotificationLogListOptions) ([]model.NotificationLog, int64, error) {
	var logs []model.NotificationLog
	queryBuilder, err := r.notificationLogsBaseQuery(ctx)
	if err != nil {
		return nil, 0, err
	}
	queryBuilder = r.applyNotificationLogFilters(queryBuilder, opts)
	total, err := countWithSession(queryBuilder)
	if err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	err = r.preloadNotificationLogs(ctx, queryBuilder.Select("notification_logs.*")).
		Offset(offset).
		Limit(opts.PageSize).
		Order(notificationLogOrderClause(opts)).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// UpdateLog 更新日志
func (r *NotificationRepository) UpdateLog(ctx context.Context, log *model.NotificationLog) error {
	return UpdateTenantScopedModel(r.db, ctx, log.ID, log)
}

// GetPendingRetryLogs 获取待重试的日志
func (r *NotificationRepository) GetPendingRetryLogs(ctx context.Context) ([]model.NotificationLog, error) {
	var logs []model.NotificationLog
	err := r.tenantDB(ctx).Preload("Channel").
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", "failed", time.Now()).
		Find(&logs).Error
	return logs, err
}

// GetPendingRetryLogsGlobal 获取所有租户待重试的日志（通知重试调度器专用）
func (r *NotificationRepository) GetPendingRetryLogsGlobal(ctx context.Context) ([]model.NotificationLog, error) {
	var logs []model.NotificationLog
	err := r.db.WithContext(ctx).
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", "failed", time.Now()).
		Find(&logs).Error
	return logs, err
}

func (r *NotificationRepository) notificationLogsBaseQuery(ctx context.Context) (*gorm.DB, error) {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return r.db.WithContext(ctx).
		Model(&model.NotificationLog{}).
		Where("notification_logs.tenant_id = ?", tenantID), nil
}

func (r *NotificationRepository) applyNotificationLogFilters(db *gorm.DB, opts *NotificationLogListOptions) *gorm.DB {
	if opts.Status != "" {
		db = db.Where("notification_logs.status = ?", opts.Status)
	}
	if opts.ChannelID != nil {
		db = db.Where("notification_logs.channel_id = ?", *opts.ChannelID)
	}
	if opts.TemplateID != nil {
		db = db.Where("notification_logs.template_id = ?", *opts.TemplateID)
	}
	if opts.ExecutionRunID != nil {
		db = db.Where("notification_logs.execution_run_id = ?", *opts.ExecutionRunID)
	}
	if !opts.Subject.IsEmpty() {
		db = applyNotificationSubjectFilter(db, opts.Subject)
	}
	if opts.CreatedAfter != nil {
		db = db.Where("notification_logs.created_at >= ?", *opts.CreatedAfter)
	}
	if opts.CreatedBefore != nil {
		db = db.Where("notification_logs.created_at <= ?", *opts.CreatedBefore)
	}
	if notificationNeedsExecutionJoin(opts) {
		db = r.joinNotificationExecutionTables(db)
		db = applyNotificationExecutionFilters(db, opts)
	}
	return db
}

func applyNotificationSubjectFilter(db *gorm.DB, subject query.StringFilter) *gorm.DB {
	if subject.Exact {
		return db.Where("notification_logs.subject = ?", subject.Value)
	}
	return db.Where("notification_logs.subject ILIKE ?", "%"+subject.Value+"%")
}

func notificationNeedsExecutionJoin(opts *NotificationLogListOptions) bool {
	return opts.TaskID != nil || !opts.TaskName.IsEmpty() || opts.TriggeredBy != ""
}

func (r *NotificationRepository) joinNotificationExecutionTables(db *gorm.DB) *gorm.DB {
	return db.Joins("LEFT JOIN execution_runs ON notification_logs.execution_run_id = execution_runs.id").
		Joins("LEFT JOIN execution_tasks ON execution_runs.task_id = execution_tasks.id")
}

func applyNotificationExecutionFilters(db *gorm.DB, opts *NotificationLogListOptions) *gorm.DB {
	if opts.TaskID != nil {
		db = db.Where("execution_runs.task_id = ?", *opts.TaskID)
	}
	if !opts.TaskName.IsEmpty() {
		if opts.TaskName.Exact {
			db = db.Where("execution_tasks.name = ?", opts.TaskName.Value)
		} else {
			db = db.Where("execution_tasks.name ILIKE ?", "%"+opts.TaskName.Value+"%")
		}
	}
	if opts.TriggeredBy != "" {
		db = db.Where("execution_runs.triggered_by = ?", opts.TriggeredBy)
	}
	return db
}

func notificationLogOrderClause(opts *NotificationLogListOptions) string {
	orderClause := "notification_logs.created_at DESC"
	if opts.SortBy == "" {
		return orderClause
	}

	order := "ASC"
	if opts.SortOrder == "desc" {
		order = "DESC"
	}
	allowedSortFields := map[string]string{
		"created_at": "notification_logs.created_at",
		"status":     "notification_logs.status",
		"subject":    "notification_logs.subject",
		"sent_at":    "notification_logs.sent_at",
	}
	if field, ok := allowedSortFields[opts.SortBy]; ok {
		orderClause = field + " " + order
	}
	return orderClause
}

func (r *NotificationRepository) preloadNotificationLogs(ctx context.Context, db *gorm.DB) *gorm.DB {
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		scoped := db.Session(&gorm.Session{})
		scoped.AddError(err)
		return scoped
	}
	return db.
		Preload("Template", "tenant_id = ?", tenantID).
		Preload("Channel", "tenant_id = ?", tenantID).
		Preload("ExecutionRun", "tenant_id = ?", tenantID).
		Preload("ExecutionRun.Task", "tenant_id = ?", tenantID)
}
