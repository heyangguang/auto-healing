package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// appStartTime 应用启动时间（用于计算 uptime）
var appStartTime = time.Now()

// WorkbenchRepository 工作台仓库
type WorkbenchRepository struct {
	db *gorm.DB
}

// NewWorkbenchRepository 创建工作台仓库
func NewWorkbenchRepository() *WorkbenchRepository {
	return &WorkbenchRepository{db: database.DB}
}

// ==================== 系统健康 ====================

// SystemHealth 系统健康状态
type SystemHealth struct {
	Status        string  `json:"status"`
	Version       string  `json:"version"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	Environment   string  `json:"environment"`
	APILatencyMs  float64 `json:"api_latency_ms"`
	DBLatencyMs   float64 `json:"db_latency_ms"`
}

// GetSystemHealth 获取系统健康状态
func (r *WorkbenchRepository) GetSystemHealth(ctx context.Context) (*SystemHealth, error) {
	// 测量 DB 延迟
	dbStart := time.Now()
	sqlDB, err := r.db.DB()
	if err != nil {
		return &SystemHealth{
			Status:        "down",
			Version:       config.GetAppConfig().Version,
			UptimeSeconds: int64(time.Since(appStartTime).Seconds()),
			Environment:   config.GetAppConfig().Env,
			DBLatencyMs:   -1,
		}, nil
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return &SystemHealth{
			Status:        "degraded",
			Version:       config.GetAppConfig().Version,
			UptimeSeconds: int64(time.Since(appStartTime).Seconds()),
			Environment:   config.GetAppConfig().Env,
			DBLatencyMs:   -1,
		}, nil
	}
	dbLatency := float64(time.Since(dbStart).Microseconds()) / 1000.0

	status := "healthy"
	if dbLatency > 100 {
		status = "degraded"
	}

	return &SystemHealth{
		Status:        status,
		Version:       config.GetAppConfig().Version,
		UptimeSeconds: int64(time.Since(appStartTime).Seconds()),
		Environment:   config.GetAppConfig().Env,
		APILatencyMs:  dbLatency / 2, // 粗估 API 延迟
		DBLatencyMs:   dbLatency,
	}, nil
}

// ==================== 自愈统计 ====================

// HealingStats 今日自愈统计
type HealingStats struct {
	TodaySuccess int64 `json:"today_success"`
	TodayFailed  int64 `json:"today_failed"`
}

// GetHealingStats 获取今日自愈统计
func (r *WorkbenchRepository) GetHealingStats(ctx context.Context) (*HealingStats, error) {
	today := time.Now().Truncate(24 * time.Hour)
	var stats HealingStats

	// 今日成功
	TenantDB(r.db, ctx).Model(&model.FlowInstance{}).
		Where("created_at >= ? AND status = ?", today, "completed").
		Count(&stats.TodaySuccess)

	// 今日失败
	TenantDB(r.db, ctx).Model(&model.FlowInstance{}).
		Where("created_at >= ? AND status = ?", today, "failed").
		Count(&stats.TodayFailed)

	return &stats, nil
}

// ==================== 工单统计 ====================

// WorkbenchIncidentStats 工单统计（工作台专用）
type WorkbenchIncidentStats struct {
	PendingCount   int64 `json:"pending_count"`
	Last7DaysTotal int64 `json:"last_7_days_total"`
}

// GetIncidentStats 获取工单统计
func (r *WorkbenchRepository) GetIncidentStats(ctx context.Context) (*WorkbenchIncidentStats, error) {
	var stats WorkbenchIncidentStats

	// 待处理（未扫描 或 扫描后状态为 pending）
	TenantDB(r.db, ctx).Model(&model.Incident{}).
		Where("scanned = ? OR healing_status = ?", false, "pending").
		Count(&stats.PendingCount)

	// 近7天总计
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	TenantDB(r.db, ctx).Model(&model.Incident{}).
		Where("created_at >= ?", sevenDaysAgo).
		Count(&stats.Last7DaysTotal)

	return &stats, nil
}

// ==================== 主机统计 ====================

// HostStats 主机统计
type HostStats struct {
	OnlineCount  int64 `json:"online_count"`
	OfflineCount int64 `json:"offline_count"`
}

// GetHostStats 获取主机统计
func (r *WorkbenchRepository) GetHostStats(ctx context.Context) (*HostStats, error) {
	var stats HostStats

	TenantDB(r.db, ctx).Model(&model.CMDBItem{}).
		Where("status = ?", "active").
		Count(&stats.OnlineCount)

	TenantDB(r.db, ctx).Model(&model.CMDBItem{}).
		Where("status != ?", "active").
		Count(&stats.OfflineCount)

	return &stats, nil
}

// ==================== 资源概览 ====================

// ResourceCount 通用资源计数
type ResourceCount struct {
	Total   int64  `json:"total"`
	Enabled *int64 `json:"enabled,omitempty"`
	Offline *int64 `json:"offline,omitempty"`
	// 自定义字段
	NeedsReview *int64  `json:"needs_review,omitempty"`
	Channels    *int64  `json:"channels,omitempty"`
	Types       *string `json:"types,omitempty"`
	Admins      *int64  `json:"admins,omitempty"`
}

// ResourceOverview 资源概览
type ResourceOverview struct {
	Flows                 ResourceCount `json:"flows"`
	Rules                 ResourceCount `json:"rules"`
	Hosts                 ResourceCount `json:"hosts"`
	Playbooks             ResourceCount `json:"playbooks"`
	Schedules             ResourceCount `json:"schedules"`
	NotificationTemplates ResourceCount `json:"notification_templates"`
	Secrets               ResourceCount `json:"secrets"`
	Users                 ResourceCount `json:"users"`
}

// GetResourceOverview 获取各模块资源总数（按权限过滤子模块）
func (r *WorkbenchRepository) GetResourceOverview(ctx context.Context, permissions []string) (*ResourceOverview, error) {
	overview := &ResourceOverview{}

	// Flows — healing:flows:view
	if repoHasPermission(permissions, "healing:flows:view") {
		TenantDB(r.db, ctx).Model(&model.HealingFlow{}).Count(&overview.Flows.Total)
		var flowsEnabled int64
		TenantDB(r.db, ctx).Model(&model.HealingFlow{}).Where("is_active = ?", true).Count(&flowsEnabled)
		overview.Flows.Enabled = &flowsEnabled
	}

	// Rules — healing:rules:view
	if repoHasPermission(permissions, "healing:rules:view") {
		TenantDB(r.db, ctx).Model(&model.HealingRule{}).Count(&overview.Rules.Total)
		var rulesEnabled int64
		TenantDB(r.db, ctx).Model(&model.HealingRule{}).Where("is_active = ?", true).Count(&rulesEnabled)
		overview.Rules.Enabled = &rulesEnabled
	}

	// Hosts — plugin:list
	if repoHasPermission(permissions, "plugin:list") {
		TenantDB(r.db, ctx).Model(&model.CMDBItem{}).Count(&overview.Hosts.Total)
		var hostsOffline int64
		TenantDB(r.db, ctx).Model(&model.CMDBItem{}).Where("status != ?", "active").Count(&hostsOffline)
		overview.Hosts.Offline = &hostsOffline
	}

	// Playbooks — plugin:list
	if repoHasPermission(permissions, "plugin:list") {
		TenantDB(r.db, ctx).Model(&model.Playbook{}).Count(&overview.Playbooks.Total)
		var pbNeedsReview int64
		TenantDB(r.db, ctx).Model(&model.Playbook{}).Where("status = ?", "draft").Count(&pbNeedsReview)
		overview.Playbooks.NeedsReview = &pbNeedsReview
	}

	// Schedules — task:list
	if repoHasPermission(permissions, "task:list") {
		TenantDB(r.db, ctx).Model(&model.ExecutionSchedule{}).Count(&overview.Schedules.Total)
		var schedEnabled int64
		TenantDB(r.db, ctx).Model(&model.ExecutionSchedule{}).Where("enabled = ?", true).Count(&schedEnabled)
		overview.Schedules.Enabled = &schedEnabled
	}

	// Notification Templates — template:list
	if repoHasPermission(permissions, "template:list") {
		TenantDB(r.db, ctx).Model(&model.NotificationTemplate{}).Count(&overview.NotificationTemplates.Total)
		var channelCount int64
		TenantDB(r.db, ctx).Model(&model.NotificationChannel{}).Count(&channelCount)
		overview.NotificationTemplates.Channels = &channelCount
	}

	// Secrets — plugin:list
	if repoHasPermission(permissions, "plugin:list") {
		TenantDB(r.db, ctx).Model(&model.SecretsSource{}).Count(&overview.Secrets.Total)
		var secretTypes []string
		TenantDB(r.db, ctx).Model(&model.SecretsSource{}).
			Distinct("type").Pluck("type", &secretTypes)
		typesStr := joinTypes(secretTypes)
		overview.Secrets.Types = &typesStr
	}

	// Users — user:list
	if repoHasPermission(permissions, "user:list") {
		tenantID := TenantIDFromContext(ctx)
		r.db.WithContext(ctx).Raw(`
			SELECT COUNT(DISTINCT user_id) FROM user_tenant_roles WHERE tenant_id = ?
		`, tenantID).Count(&overview.Users.Total)
		var adminCount int64
		r.db.WithContext(ctx).Raw(`
			SELECT COUNT(DISTINCT utr.user_id) FROM user_tenant_roles utr
			JOIN roles r ON utr.role_id = r.id
			WHERE utr.tenant_id = ? AND (r.name = 'admin' OR r.name = 'super_admin')
		`, tenantID).Count(&adminCount)
		overview.Users.Admins = &adminCount
	}

	return overview, nil
}

// repoHasPermission 检查用户是否有指定权限（含通配符匹配）
func repoHasPermission(userPermissions []string, required string) bool {
	for _, p := range userPermissions {
		if p == "*" {
			return true
		}
		if p == required {
			return true
		}
		if strings.HasSuffix(p, ":*") {
			module := strings.TrimSuffix(p, ":*")
			if strings.HasPrefix(required, module+":") {
				return true
			}
		}
	}
	return false
}

// joinTypes 拼接类型字符串
func joinTypes(types []string) string {
	if len(types) == 0 {
		return ""
	}
	result := ""
	for i, t := range types {
		if i > 0 {
			result += " + "
		}
		// 大写首字母
		switch t {
		case "ssh":
			result += "SSH"
		case "api":
			result += "API"
		default:
			result += t
		}
	}
	return result
}

// ==================== 活动动态 ====================

// ActivityItem 活动动态项
type ActivityItem struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// GetRecentActivities 从审计日志获取最近的活动动态
func (r *WorkbenchRepository) GetRecentActivities(ctx context.Context, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		limit = 10
	}

	var logs []model.AuditLog
	err := TenantDB(r.db, ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	items := make([]ActivityItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, ActivityItem{
			ID:        log.ID,
			Type:      mapResourceTypeToActivityType(log.ResourceType),
			Text:      buildActivityText(log.Action, log.ResourceType, log.ResourceName),
			CreatedAt: log.CreatedAt,
		})
	}
	return items, nil
}

// mapResourceTypeToActivityType 将审计资源类型映射为活动类型
func mapResourceTypeToActivityType(resourceType string) string {
	switch resourceType {
	case "execution_task", "execution_run":
		return "execution"
	case "healing_flow":
		return "flow"
	case "healing_rule":
		return "rule"
	case "auth":
		return "system"
	default:
		return "system"
	}
}

// buildActivityText 构建活动描述文本
func buildActivityText(action, resourceType, resourceName string) string {
	// 登录/登出特殊处理
	if resourceType == "auth" {
		switch action {
		case "login":
			return "用户登录系统"
		case "logout":
			return "用户退出系统"
		}
	}

	actionMap := map[string]string{
		"create":     "创建",
		"update":     "更新",
		"delete":     "删除",
		"execute":    "执行",
		"enable":     "启用",
		"disable":    "禁用",
		"activate":   "激活",
		"deactivate": "停用",
		"approve":    "审批通过",
		"reject":     "审批拒绝",
		"dismiss":    "忽略",
		"login":      "登录",
		"logout":     "退出",
	}

	typeMap := map[string]string{
		"execution_task":   "执行任务",
		"execution_run":    "执行运行",
		"healing_flow":     "自愈流程",
		"healing_rule":     "自愈规则",
		"healing_instance": "自愈实例",
		"cmdb_item":        "资产",
		"plugin":           "插件",
		"playbook":         "Playbook",
		"schedule":         "定时任务",
		"notification":     "通知",
		"secrets":          "密钥",
		"user":             "用户",
		"role":             "角色",
		"site_message":     "站内信",
		"tenant":           "租户",
	}

	actionText := actionMap[action]
	if actionText == "" {
		actionText = action
	}

	typeText := typeMap[resourceType]
	if typeText == "" {
		typeText = resourceType
	}

	if resourceName != "" {
		return fmt.Sprintf("%s%s：%s", actionText, typeText, resourceName)
	}
	return fmt.Sprintf("%s了%s", actionText, typeText)
}

// ==================== 定时任务日历 ====================

// CalendarTask 日历任务项
type CalendarTask struct {
	Name       string `json:"name"`
	Time       string `json:"time"`
	ScheduleID string `json:"schedule_id"`
}

// GetScheduleCalendar 获取指定月份的定时任务日历
func (r *WorkbenchRepository) GetScheduleCalendar(ctx context.Context, year, month int) (map[string][]CalendarTask, error) {
	// 获取所有启用的 cron 定时任务
	var schedules []model.ExecutionSchedule
	err := TenantDB(r.db, ctx).
		Where("enabled = ? AND schedule_type = ?", true, model.ScheduleTypeCron).
		Where("schedule_expr IS NOT NULL AND schedule_expr != ''").
		Preload("Task").
		Find(&schedules).Error
	if err != nil {
		return nil, err
	}

	// 计算月份的起止时间
	loc := time.Now().Location()
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	result := make(map[string][]CalendarTask)

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for _, schedule := range schedules {
		if schedule.ScheduleExpr == nil || *schedule.ScheduleExpr == "" {
			continue
		}

		sched, err := parser.Parse(*schedule.ScheduleExpr)
		if err != nil {
			continue
		}

		name := schedule.Name
		if schedule.Task != nil {
			name = schedule.Name
		}

		// 遍历月份中的每一天，找出该 cron 表达式的执行时间
		current := startOfMonth.Add(-time.Second) // 从前一秒开始，这样 Next 能返回当月第一秒
		for {
			next := sched.Next(current)
			if next.After(endOfMonth) || next.Equal(endOfMonth) {
				break
			}

			dateKey := next.Format("2006-01-02")
			timeStr := next.Format("15:04")

			result[dateKey] = append(result[dateKey], CalendarTask{
				Name:       name,
				Time:       timeStr,
				ScheduleID: schedule.ID.String(),
			})

			current = next
		}
	}

	// 同时处理 once 类型的定时任务
	var onceSchedules []model.ExecutionSchedule
	err = TenantDB(r.db, ctx).
		Where("enabled = ? AND schedule_type = ?", true, model.ScheduleTypeOnce).
		Where("scheduled_at IS NOT NULL AND scheduled_at >= ? AND scheduled_at < ?", startOfMonth, endOfMonth).
		Find(&onceSchedules).Error
	if err != nil {
		return nil, err
	}

	for _, schedule := range onceSchedules {
		if schedule.ScheduledAt == nil {
			continue
		}

		dateKey := schedule.ScheduledAt.In(loc).Format("2006-01-02")
		timeStr := schedule.ScheduledAt.In(loc).Format("15:04")

		result[dateKey] = append(result[dateKey], CalendarTask{
			Name:       schedule.Name,
			Time:       timeStr,
			ScheduleID: schedule.ID.String(),
		})
	}

	return result, nil
}

// ==================== 系统公告 ====================

// AnnouncementItem 公告项
type AnnouncementItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// GetAnnouncements 获取系统公告列表
func (r *WorkbenchRepository) GetAnnouncements(ctx context.Context, limit int, userCreatedAt time.Time) ([]AnnouncementItem, error) {
	if limit <= 0 {
		limit = 5
	}

	query := TenantDB(r.db, ctx).
		Where("category = ?", model.SiteMessageCategoryAnnouncement)

	// 只显示用户创建时间之后的公告（新用户不看旧公告）
	if !userCreatedAt.IsZero() {
		query = query.Where("created_at >= ?", userCreatedAt)
	}

	var messages []model.SiteMessage
	err := query.
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	items := make([]AnnouncementItem, 0, len(messages))
	for _, msg := range messages {
		items = append(items, AnnouncementItem{
			ID:        msg.ID,
			Title:     msg.Title,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		})
	}
	return items, nil
}

// ==================== 用户收藏 ====================

// FavoriteItem 收藏项
type FavoriteItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Path  string `json:"path"`
}
