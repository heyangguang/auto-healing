package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DashboardRepository Dashboard 仓库
type DashboardRepository struct {
	db *gorm.DB
}

// NewDashboardRepository 创建 Dashboard 仓库
func NewDashboardRepository() *DashboardRepository {
	return &DashboardRepository{
		db: database.DB,
	}
}

// ==================== 用户配置 CRUD ====================

// GetConfigByUserID 获取用户配置
func (r *DashboardRepository) GetConfigByUserID(ctx context.Context, userID uuid.UUID) (*model.DashboardConfig, error) {
	var config model.DashboardConfig
	err := TenantDB(r.db, ctx).Where("user_id = ?", userID).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// UpsertConfig 创建或更新用户配置（使用 ON CONFLICT DO UPDATE 保证原子性）
func (r *DashboardRepository) UpsertConfig(ctx context.Context, userID uuid.UUID, configData model.JSON) error {
	tenantID := TenantIDFromContext(ctx)
	config := model.DashboardConfig{
		UserID:   userID,
		TenantID: &tenantID,
		Config:   configData,
	}
	// user_id 列有唯一约束（dashboard_configs_user_id_key）
	// ON CONFLICT(user_id) DO UPDATE 原子性地处理创建或更新，彻底消除竞态
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"config":     configData,
			"updated_at": time.Now(),
		}),
	}).Create(&config).Error
}

// ==================== Section: incidents ====================

type IncidentSection struct {
	Total             int64         `json:"total"`
	Today             int64         `json:"today"`
	ThisWeek          int64         `json:"this_week"`
	Unscanned         int64         `json:"unscanned"`
	HealingRate       float64       `json:"healing_rate"`
	ByHealingStatus   []StatusCount `json:"by_healing_status"`
	BySeverity        []StatusCount `json:"by_severity"`
	ByCategory        []StatusCount `json:"by_category"`
	ByStatus          []StatusCount `json:"by_status"`
	BySource          []StatusCount `json:"by_source"`
	Trend7d           []TrendPoint  `json:"trend_7d"`
	Trend30d          []TrendPoint  `json:"trend_30d"`
	RecentIncidents   []RecentItem  `json:"recent_incidents"`
	CriticalIncidents []RecentItem  `json:"critical_incidents"`
}

type StatusCount struct {
	Status string `json:"status" gorm:"column:status"`
	Count  int64  `json:"count" gorm:"column:count"`
}

type TrendPoint struct {
	Date  string `json:"date" gorm:"column:date"`
	Count int64  `json:"count" gorm:"column:count"`
}

type RecentItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetIncidentSection(ctx context.Context) (*IncidentSection, error) {
	section := &IncidentSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	// 总数
	newDB().Model(&model.Incident{}).Count(&section.Total)

	// 今日
	today := time.Now().Truncate(24 * time.Hour)
	newDB().Model(&model.Incident{}).Where("created_at >= ?", today).Count(&section.Today)

	// 本周
	weekAgo := time.Now().AddDate(0, 0, -7)
	newDB().Model(&model.Incident{}).Where("created_at >= ?", weekAgo).Count(&section.ThisWeek)

	// 未扫描
	newDB().Model(&model.Incident{}).Where("scanned = ?", false).Count(&section.Unscanned)

	// 按 healing_status 分组
	newDB().Model(&model.Incident{}).
		Select("healing_status as status, count(*) as count").
		Group("healing_status").
		Scan(&section.ByHealingStatus)

	// 计算自愈率
	var healed, failed int64
	for _, s := range section.ByHealingStatus {
		switch s.Status {
		case "healed":
			healed = s.Count
		case "failed":
			failed = s.Count
		}
	}
	if healed+failed > 0 {
		section.HealingRate = float64(healed) / float64(healed+failed) * 100
	}

	// 按 severity 分组
	newDB().Model(&model.Incident{}).
		Select("severity as status, count(*) as count").
		Group("severity").
		Scan(&section.BySeverity)

	// 按 category 分组 (在 SQL 层面统一处理 NULL 和空字符串)
	newDB().Model(&model.Incident{}).
		Select("COALESCE(NULLIF(category, ''), 'unknown') as status, count(*) as count").
		Group("COALESCE(NULLIF(category, ''), 'unknown')").
		Order("count DESC").
		Scan(&section.ByCategory)

	// 按原始 status 分组
	newDB().Model(&model.Incident{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&section.ByStatus)

	// 按来源插件分组
	newDB().Model(&model.Incident{}).
		Select("source_plugin_name as status, count(*) as count").
		Group("source_plugin_name").
		Scan(&section.BySource)

	// 7天趋势
	newDB().Model(&model.Incident{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at >= ?", weekAgo).
		Group("DATE(created_at)").
		Order("date").
		Scan(&section.Trend7d)

	// 30天趋势
	monthAgo := time.Now().AddDate(0, 0, -30)
	newDB().Model(&model.Incident{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at >= ?", monthAgo).
		Group("DATE(created_at)").
		Order("date").
		Scan(&section.Trend30d)

	// 最近工单
	var recentIncidents []model.Incident
	newDB().Model(&model.Incident{}).
		Order("created_at DESC").
		Limit(10).
		Find(&recentIncidents)
	for _, inc := range recentIncidents {
		section.RecentIncidents = append(section.RecentIncidents, RecentItem{
			ID: inc.ID, Title: inc.Title, Status: inc.HealingStatus, CreatedAt: inc.CreatedAt,
		})
	}

	// 紧急工单
	var criticalIncidents []model.Incident
	newDB().Model(&model.Incident{}).
		Where("severity = ?", "critical").
		Order("created_at DESC").
		Limit(10).
		Find(&criticalIncidents)
	for _, inc := range criticalIncidents {
		section.CriticalIncidents = append(section.CriticalIncidents, RecentItem{
			ID: inc.ID, Title: inc.Title, Status: inc.HealingStatus, CreatedAt: inc.CreatedAt,
		})
	}

	return section, nil
}

// ==================== Section: cmdb ====================

type CMDBSection struct {
	Total             int64             `json:"total"`
	Active            int64             `json:"active"`
	Maintenance       int64             `json:"maintenance"`
	Offline           int64             `json:"offline"`
	ActiveRate        float64           `json:"active_rate"`
	ByStatus          []StatusCount     `json:"by_status"`
	ByEnvironment     []StatusCount     `json:"by_environment"`
	ByType            []StatusCount     `json:"by_type"`
	ByOS              []StatusCount     `json:"by_os"`
	ByDepartment      []StatusCount     `json:"by_department"`
	ByManufacturer    []StatusCount     `json:"by_manufacturer"`
	RecentMaintenance []MaintenanceItem `json:"recent_maintenance"`
	OfflineAssets     []AssetItem       `json:"offline_assets"`
}

type MaintenanceItem struct {
	ID           uuid.UUID `json:"id"`
	CMDBItemName string    `json:"cmdb_item_name"`
	Action       string    `json:"action"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

type AssetItem struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	IPAddress   string    `json:"ip_address"`
	Environment string    `json:"environment"`
}

func (r *DashboardRepository) GetCMDBSection(ctx context.Context) (*CMDBSection, error) {
	section := &CMDBSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	newDB().Model(&model.CMDBItem{}).Count(&section.Total)
	newDB().Model(&model.CMDBItem{}).Where("status = ?", "active").Count(&section.Active)
	newDB().Model(&model.CMDBItem{}).Where("status = ?", "maintenance").Count(&section.Maintenance)
	newDB().Model(&model.CMDBItem{}).Where("status = ?", "offline").Count(&section.Offline)

	if section.Total > 0 {
		section.ActiveRate = float64(section.Active) / float64(section.Total) * 100
	}

	// 初始化为空切片，避免返回 null
	section.ByStatus = []StatusCount{}
	section.ByEnvironment = []StatusCount{}
	section.ByType = []StatusCount{}
	section.ByOS = []StatusCount{}
	section.ByDepartment = []StatusCount{}
	section.ByManufacturer = []StatusCount{}

	newDB().Model(&model.CMDBItem{}).Select("status, count(*) as count").Group("status").Scan(&section.ByStatus)
	newDB().Model(&model.CMDBItem{}).Select("environment as status, count(*) as count").Group("environment").Scan(&section.ByEnvironment)
	newDB().Model(&model.CMDBItem{}).Select("type as status, count(*) as count").Group("type").Scan(&section.ByType)
	newDB().Model(&model.CMDBItem{}).Select("os as status, count(*) as count").Group("os").Scan(&section.ByOS)
	newDB().Model(&model.CMDBItem{}).Select("department as status, count(*) as count").Group("department").Scan(&section.ByDepartment)
	newDB().Model(&model.CMDBItem{}).Select("manufacturer as status, count(*) as count").Group("manufacturer").Scan(&section.ByManufacturer)

	// 最近维护记录
	var logs []model.CMDBMaintenanceLog
	newDB().Model(&model.CMDBMaintenanceLog{}).Order("created_at DESC").Limit(10).Find(&logs)
	for _, l := range logs {
		section.RecentMaintenance = append(section.RecentMaintenance, MaintenanceItem{
			ID: l.ID, CMDBItemName: l.CMDBItemName, Action: l.Action, Reason: l.Reason, CreatedAt: l.CreatedAt,
		})
	}

	// 离线资产
	var offlineItems []model.CMDBItem
	newDB().Model(&model.CMDBItem{}).Where("status = ?", "offline").Order("updated_at DESC").Limit(10).Find(&offlineItems)
	for _, item := range offlineItems {
		section.OfflineAssets = append(section.OfflineAssets, AssetItem{
			ID: item.ID, Name: item.Name, Type: item.Type, IPAddress: item.IPAddress, Environment: item.Environment,
		})
	}

	return section, nil
}

// ==================== Section: healing ====================

type HealingSection struct {
	FlowsTotal          int64          `json:"flows_total"`
	FlowsActive         int64          `json:"flows_active"`
	RulesTotal          int64          `json:"rules_total"`
	RulesActive         int64          `json:"rules_active"`
	InstancesTotal      int64          `json:"instances_total"`
	InstancesRunning    int64          `json:"instances_running"`
	PendingApprovals    int64          `json:"pending_approvals"`
	PendingTriggers     int64          `json:"pending_triggers"`
	InstancesByStatus   []StatusCount  `json:"instances_by_status"`
	InstanceTrend7d     []TrendPoint   `json:"instance_trend_7d"`
	ApprovalsByStatus   []StatusCount  `json:"approvals_by_status"`
	RulesByTriggerMode  []StatusCount  `json:"rules_by_trigger_mode"`
	FlowTop10           []RankItem     `json:"flow_top10"`
	RecentInstances     []InstanceItem `json:"recent_instances"`
	PendingApprovalList []ApprovalItem `json:"pending_approval_list"`
	PendingTriggerList  []TriggerItem  `json:"pending_trigger_list"`
}

type RankItem struct {
	Name  string `json:"name" gorm:"column:name"`
	Count int64  `json:"count" gorm:"column:count"`
}

type InstanceItem struct {
	ID        uuid.UUID `json:"id"`
	FlowName  string    `json:"flow_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ApprovalItem struct {
	ID             uuid.UUID `json:"id"`
	FlowInstanceID uuid.UUID `json:"flow_instance_id"`
	NodeID         string    `json:"node_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type TriggerItem struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	AffectedCI string    `json:"affected_ci"`
	CreatedAt  time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetHealingSection(ctx context.Context) (*HealingSection, error) {
	section := &HealingSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	tenantID := TenantIDFromContext(ctx)

	newDB().Model(&model.HealingFlow{}).Count(&section.FlowsTotal)
	newDB().Model(&model.HealingFlow{}).Where("is_active = ?", true).Count(&section.FlowsActive)
	newDB().Model(&model.HealingRule{}).Count(&section.RulesTotal)
	newDB().Model(&model.HealingRule{}).Where("is_active = ?", true).Count(&section.RulesActive)
	newDB().Model(&model.FlowInstance{}).Count(&section.InstancesTotal)
	newDB().Model(&model.FlowInstance{}).Where("status = ?", "running").Count(&section.InstancesRunning)
	newDB().Model(&model.ApprovalTask{}).Where("status = ?", "pending").Count(&section.PendingApprovals)

	// 待触发工单数
	newDB().Model(&model.Incident{}).
		Where("scanned = ? AND matched_rule_id IS NOT NULL AND healing_flow_instance_id IS NULL", true).
		Count(&section.PendingTriggers)

	// 实例按状态
	newDB().Model(&model.FlowInstance{}).Select("status, count(*) as count").Group("status").Scan(&section.InstancesByStatus)

	// 实例7天趋势
	weekAgo := time.Now().AddDate(0, 0, -7)
	newDB().Model(&model.FlowInstance{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at >= ?", weekAgo).
		Group("DATE(created_at)").Order("date").
		Scan(&section.InstanceTrend7d)

	// 审批按状态
	newDB().Model(&model.ApprovalTask{}).Select("status, count(*) as count").Group("status").Scan(&section.ApprovalsByStatus)

	// 规则按触发模式
	newDB().Model(&model.HealingRule{}).Select("trigger_mode as status, count(*) as count").Group("trigger_mode").Scan(&section.RulesByTriggerMode)

	// 流程触发排行 TOP10（JOIN 需要显式 tenant_id 前缀）
	r.db.WithContext(ctx).Where("fi.tenant_id = ?", tenantID).
		Table("flow_instances fi").
		Select("hf.name as name, count(*) as count").
		Joins("JOIN healing_flows hf ON fi.flow_id = hf.id").
		Group("hf.name").
		Order("count DESC").
		Limit(10).
		Scan(&section.FlowTop10)

	// 最近实例
	var instances []model.FlowInstance
	newDB().Model(&model.FlowInstance{}).Order("created_at DESC").Limit(10).Find(&instances)
	for _, inst := range instances {
		section.RecentInstances = append(section.RecentInstances, InstanceItem{
			ID: inst.ID, FlowName: inst.FlowName, Status: inst.Status, CreatedAt: inst.CreatedAt,
		})
	}

	// 待审批列表
	var approvals []model.ApprovalTask
	newDB().Model(&model.ApprovalTask{}).Where("status = ?", "pending").Order("created_at DESC").Limit(10).Find(&approvals)
	for _, a := range approvals {
		section.PendingApprovalList = append(section.PendingApprovalList, ApprovalItem{
			ID: a.ID, FlowInstanceID: a.FlowInstanceID, NodeID: a.NodeID, Status: a.Status, CreatedAt: a.CreatedAt,
		})
	}

	// 待触发列表
	var triggers []model.Incident
	newDB().Model(&model.Incident{}).
		Where("scanned = ? AND matched_rule_id IS NOT NULL AND healing_flow_instance_id IS NULL", true).
		Order("created_at DESC").Limit(10).Find(&triggers)
	for _, t := range triggers {
		section.PendingTriggerList = append(section.PendingTriggerList, TriggerItem{
			ID: t.ID, Title: t.Title, Severity: t.Severity, AffectedCI: t.AffectedCI, CreatedAt: t.CreatedAt,
		})
	}

	return section, nil
}

// ==================== Section: execution ====================

type ExecutionSection struct {
	TasksTotal       int64         `json:"tasks_total"`
	RunsTotal        int64         `json:"runs_total"`
	SuccessRate      float64       `json:"success_rate"`
	Running          int64         `json:"running"`
	AvgDurationSec   float64       `json:"avg_duration_sec"`
	SchedulesTotal   int64         `json:"schedules_total"`
	SchedulesEnabled int64         `json:"schedules_enabled"`
	RunsByStatus     []StatusCount `json:"runs_by_status"`
	Trend7d          []TrendPoint  `json:"trend_7d"`
	Trend30d         []TrendPoint  `json:"trend_30d"`
	SchedulesByType  []StatusCount `json:"schedules_by_type"`
	TaskTop10        []RankItem    `json:"task_top10"`
	RecentRuns       []RunItem     `json:"recent_runs"`
	FailedRuns       []RunItem     `json:"failed_runs"`
}

type RunItem struct {
	ID          uuid.UUID  `json:"id"`
	TaskName    string     `json:"task_name"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (r *DashboardRepository) GetExecutionSection(ctx context.Context) (*ExecutionSection, error) {
	section := &ExecutionSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }
	tenantID := TenantIDFromContext(ctx)

	newDB().Model(&model.ExecutionTask{}).Count(&section.TasksTotal)
	newDB().Model(&model.ExecutionRun{}).Count(&section.RunsTotal)
	newDB().Model(&model.ExecutionRun{}).Where("status = ?", "running").Count(&section.Running)

	// 成功率
	var successCount int64
	newDB().Model(&model.ExecutionRun{}).Where("status = ?", "success").Count(&successCount)
	if section.RunsTotal > 0 {
		section.SuccessRate = float64(successCount) / float64(section.RunsTotal) * 100
	}

	// 平均执行时长
	newDB().Model(&model.ExecutionRun{}).
		Where("started_at IS NOT NULL AND completed_at IS NOT NULL").
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)").
		Scan(&section.AvgDurationSec)

	newDB().Model(&model.ExecutionSchedule{}).Count(&section.SchedulesTotal)
	newDB().Model(&model.ExecutionSchedule{}).Where("enabled = ?", true).Count(&section.SchedulesEnabled)

	// 按状态
	newDB().Model(&model.ExecutionRun{}).Select("status, count(*) as count").Group("status").Scan(&section.RunsByStatus)

	// 7天趋势
	weekAgo := time.Now().AddDate(0, 0, -7)
	newDB().Model(&model.ExecutionRun{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at >= ?", weekAgo).
		Group("DATE(created_at)").Order("date").
		Scan(&section.Trend7d)

	// 30天趋势
	monthAgo := time.Now().AddDate(0, 0, -30)
	newDB().Model(&model.ExecutionRun{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at >= ?", monthAgo).
		Group("DATE(created_at)").Order("date").
		Scan(&section.Trend30d)

	// 定时任务按类型
	newDB().Model(&model.ExecutionSchedule{}).Select("schedule_type as status, count(*) as count").Group("schedule_type").Scan(&section.SchedulesByType)

	// 任务执行排行 TOP10（JOIN 需要显式 tenant_id 前缀）
	r.db.WithContext(ctx).Where("er.tenant_id = ?", tenantID).
		Table("execution_runs er").
		Select("et.name as name, count(*) as count").
		Joins("JOIN execution_tasks et ON er.task_id = et.id").
		Group("et.name").
		Order("count DESC").
		Limit(10).
		Scan(&section.TaskTop10)

	// 最近执行
	var runs []model.ExecutionRun
	newDB().Model(&model.ExecutionRun{}).Preload("Task").Order("created_at DESC").Limit(10).Find(&runs)
	for _, run := range runs {
		taskName := ""
		if run.Task != nil {
			taskName = run.Task.Name
		}
		section.RecentRuns = append(section.RecentRuns, RunItem{
			ID: run.ID, TaskName: taskName, Status: run.Status, StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, CreatedAt: run.CreatedAt,
		})
	}

	// 失败执行
	var failedRuns []model.ExecutionRun
	newDB().Model(&model.ExecutionRun{}).Preload("Task").Where("status = ?", "failed").Order("created_at DESC").Limit(10).Find(&failedRuns)
	for _, run := range failedRuns {
		taskName := ""
		if run.Task != nil {
			taskName = run.Task.Name
		}
		section.FailedRuns = append(section.FailedRuns, RunItem{
			ID: run.ID, TaskName: taskName, Status: run.Status, StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, CreatedAt: run.CreatedAt,
		})
	}

	return section, nil
}

// ==================== Section: plugins ====================

type PluginSection struct {
	Total           int64         `json:"total"`
	Active          int64         `json:"active"`
	Inactive        int64         `json:"inactive"`
	Error           int64         `json:"error"`
	SyncSuccessRate float64       `json:"sync_success_rate"`
	ByStatus        []StatusCount `json:"by_status"`
	ByType          []StatusCount `json:"by_type"`
	SyncTrend7d     []TrendPoint  `json:"sync_trend_7d"`
	RecentSyncs     []SyncItem    `json:"recent_syncs"`
	ErrorPlugins    []PluginItem  `json:"error_plugins"`
	PluginOverview  []PluginItem  `json:"plugin_overview"`
}

type SyncItem struct {
	ID         uuid.UUID `json:"id"`
	PluginName string    `json:"plugin_name"`
	Status     string    `json:"status"`
	SyncType   string    `json:"sync_type"`
	StartedAt  time.Time `json:"started_at"`
}

type PluginItem struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	LastSyncAt *time.Time `json:"last_sync_at"`
}

func (r *DashboardRepository) GetPluginSection(ctx context.Context) (*PluginSection, error) {
	section := &PluginSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	newDB().Model(&model.Plugin{}).Count(&section.Total)
	newDB().Model(&model.Plugin{}).Where("status = ?", "active").Count(&section.Active)
	newDB().Model(&model.Plugin{}).Where("status = ?", "inactive").Count(&section.Inactive)
	newDB().Model(&model.Plugin{}).Where("status = ?", "error").Count(&section.Error)

	// 同步成功率
	var syncTotal, syncSuccess int64
	newDB().Model(&model.PluginSyncLog{}).Count(&syncTotal)
	newDB().Model(&model.PluginSyncLog{}).Where("status = ?", "success").Count(&syncSuccess)
	if syncTotal > 0 {
		section.SyncSuccessRate = float64(syncSuccess) / float64(syncTotal) * 100
	}

	// 初始化为空切片，避免返回 null
	section.ByStatus = []StatusCount{}
	section.ByType = []StatusCount{}

	newDB().Model(&model.Plugin{}).Select("status, count(*) as count").Group("status").Scan(&section.ByStatus)
	newDB().Model(&model.Plugin{}).Select("type as status, count(*) as count").Group("type").Scan(&section.ByType)

	// 同步7天趋势
	weekAgo := time.Now().AddDate(0, 0, -7)
	newDB().Model(&model.PluginSyncLog{}).
		Select("DATE(started_at) as date, count(*) as count").
		Where("started_at >= ?", weekAgo).
		Group("DATE(started_at)").Order("date").
		Scan(&section.SyncTrend7d)

	// 最近同步
	var syncs []model.PluginSyncLog
	newDB().Model(&model.PluginSyncLog{}).Preload("Plugin").Order("started_at DESC").Limit(10).Find(&syncs)
	for _, s := range syncs {
		name := ""
		if s.Plugin.Name != "" {
			name = s.Plugin.Name
		}
		section.RecentSyncs = append(section.RecentSyncs, SyncItem{
			ID: s.ID, PluginName: name, Status: s.Status, SyncType: s.SyncType, StartedAt: s.StartedAt,
		})
	}

	// 异常插件
	var errorPlugins []model.Plugin
	newDB().Model(&model.Plugin{}).Where("status = ?", "error").Find(&errorPlugins)
	for _, p := range errorPlugins {
		section.ErrorPlugins = append(section.ErrorPlugins, PluginItem{
			ID: p.ID, Name: p.Name, Type: p.Type, Status: p.Status, LastSyncAt: p.LastSyncAt,
		})
	}

	// 全部插件概览
	var allPlugins []model.Plugin
	newDB().Model(&model.Plugin{}).Order("name").Find(&allPlugins)
	for _, p := range allPlugins {
		section.PluginOverview = append(section.PluginOverview, PluginItem{
			ID: p.ID, Name: p.Name, Type: p.Type, Status: p.Status, LastSyncAt: p.LastSyncAt,
		})
	}

	return section, nil
}

// ==================== Section: notifications ====================

type NotificationSection struct {
	ChannelsTotal  int64          `json:"channels_total"`
	TemplatesTotal int64          `json:"templates_total"`
	LogsTotal      int64          `json:"logs_total"`
	DeliveryRate   float64        `json:"delivery_rate"`
	ByChannelType  []StatusCount  `json:"by_channel_type"`
	ByLogStatus    []StatusCount  `json:"by_log_status"`
	Trend7d        []TrendPoint   `json:"trend_7d"`
	RecentLogs     []NotifLogItem `json:"recent_logs"`
	FailedLogs     []NotifLogItem `json:"failed_logs"`
}

type NotifLogItem struct {
	ID        uuid.UUID `json:"id"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetNotificationSection(ctx context.Context) (*NotificationSection, error) {
	section := &NotificationSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	newDB().Model(&model.NotificationChannel{}).Count(&section.ChannelsTotal)
	newDB().Model(&model.NotificationTemplate{}).Count(&section.TemplatesTotal)
	newDB().Model(&model.NotificationLog{}).Count(&section.LogsTotal)

	// 送达率
	var sentCount int64
	newDB().Model(&model.NotificationLog{}).Where("status IN ?", []string{"sent", "delivered"}).Count(&sentCount)
	if section.LogsTotal > 0 {
		section.DeliveryRate = float64(sentCount) / float64(section.LogsTotal) * 100
	}

	newDB().Model(&model.NotificationChannel{}).Select("type as status, count(*) as count").Group("type").Scan(&section.ByChannelType)
	newDB().Model(&model.NotificationLog{}).Select("status, count(*) as count").Group("status").Scan(&section.ByLogStatus)

	weekAgo := time.Now().AddDate(0, 0, -7)
	newDB().Model(&model.NotificationLog{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at >= ?", weekAgo).
		Group("DATE(created_at)").Order("date").
		Scan(&section.Trend7d)

	var recentLogs []model.NotificationLog
	newDB().Model(&model.NotificationLog{}).Order("created_at DESC").Limit(10).Find(&recentLogs)
	for _, l := range recentLogs {
		section.RecentLogs = append(section.RecentLogs, NotifLogItem{
			ID: l.ID, Subject: l.Subject, Status: l.Status, CreatedAt: l.CreatedAt,
		})
	}

	var failedLogs []model.NotificationLog
	newDB().Model(&model.NotificationLog{}).Where("status = ?", "failed").Order("created_at DESC").Limit(10).Find(&failedLogs)
	for _, l := range failedLogs {
		section.FailedLogs = append(section.FailedLogs, NotifLogItem{
			ID: l.ID, Subject: l.Subject, Status: l.Status, CreatedAt: l.CreatedAt,
		})
	}

	return section, nil
}

// ==================== Section: git ====================

type GitSection struct {
	ReposTotal      int64         `json:"repos_total"`
	SyncSuccessRate float64       `json:"sync_success_rate"`
	Repos           []GitRepoItem `json:"repos"`
	RecentSyncs     []GitSyncItem `json:"recent_syncs"`
}

type GitRepoItem struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Status     string     `json:"status"`
	Branch     string     `json:"branch"`
	LastSyncAt *time.Time `json:"last_sync_at"`
}

type GitSyncItem struct {
	ID        uuid.UUID `json:"id"`
	RepoName  string    `json:"repo_name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetGitSection(ctx context.Context) (*GitSection, error) {
	section := &GitSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	newDB().Model(&model.GitRepository{}).Count(&section.ReposTotal)

	var syncTotal, syncSuccess int64
	newDB().Model(&model.GitSyncLog{}).Count(&syncTotal)
	newDB().Model(&model.GitSyncLog{}).Where("status = ?", "success").Count(&syncSuccess)
	if syncTotal > 0 {
		section.SyncSuccessRate = float64(syncSuccess) / float64(syncTotal) * 100
	}

	var repos []model.GitRepository
	newDB().Model(&model.GitRepository{}).Order("name").Find(&repos)
	for _, r := range repos {
		section.Repos = append(section.Repos, GitRepoItem{
			ID: r.ID, Name: r.Name, URL: r.URL, Status: r.Status, Branch: r.DefaultBranch, LastSyncAt: r.LastSyncAt,
		})
	}

	var syncs []model.GitSyncLog
	newDB().Model(&model.GitSyncLog{}).Preload("Repository").Order("created_at DESC").Limit(10).Find(&syncs)
	for _, s := range syncs {
		repoName := ""
		if s.Repository != nil {
			repoName = s.Repository.Name
		}
		section.RecentSyncs = append(section.RecentSyncs, GitSyncItem{
			ID: s.ID, RepoName: repoName, Status: s.Status, CreatedAt: s.CreatedAt,
		})
	}

	return section, nil
}

// ==================== Section: playbooks ====================

type PlaybookSection struct {
	Total       int64         `json:"total"`
	Ready       int64         `json:"ready"`
	ByStatus    []StatusCount `json:"by_status"`
	RecentScans []ScanItem    `json:"recent_scans"`
}

type ScanItem struct {
	ID           uuid.UUID `json:"id"`
	PlaybookName string    `json:"playbook_name"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func (r *DashboardRepository) GetPlaybookSection(ctx context.Context) (*PlaybookSection, error) {
	section := &PlaybookSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	newDB().Model(&model.Playbook{}).Count(&section.Total)
	newDB().Model(&model.Playbook{}).Where("status = ?", "ready").Count(&section.Ready)
	newDB().Model(&model.Playbook{}).Select("status, count(*) as count").Group("status").Scan(&section.ByStatus)

	var scans []model.PlaybookScanLog
	newDB().Model(&model.PlaybookScanLog{}).Preload("Playbook").Order("created_at DESC").Limit(10).Find(&scans)
	for _, s := range scans {
		pbName := ""
		if s.Playbook != nil {
			pbName = s.Playbook.Name
		}
		section.RecentScans = append(section.RecentScans, ScanItem{
			ID: s.ID, PlaybookName: pbName, Status: s.TriggerType, CreatedAt: s.CreatedAt,
		})
	}

	return section, nil
}

// ==================== Section: secrets ====================

type SecretsSection struct {
	Total      int64         `json:"total"`
	Active     int64         `json:"active"`
	ByType     []StatusCount `json:"by_type"`
	ByAuthType []StatusCount `json:"by_auth_type"`
}

func (r *DashboardRepository) GetSecretsSection(ctx context.Context) (*SecretsSection, error) {
	section := &SecretsSection{}
	newDB := func() *gorm.DB { return TenantDB(r.db, ctx) }

	newDB().Model(&model.SecretsSource{}).Count(&section.Total)
	newDB().Model(&model.SecretsSource{}).Where("status = ?", "active").Count(&section.Active)
	newDB().Model(&model.SecretsSource{}).Select("type as status, count(*) as count").Group("type").Scan(&section.ByType)
	newDB().Model(&model.SecretsSource{}).Select("auth_type as status, count(*) as count").Group("auth_type").Scan(&section.ByAuthType)

	return section, nil
}

// ==================== Section: users ====================

type UsersSection struct {
	Total        int64       `json:"total"`
	Active       int64       `json:"active"`
	RolesTotal   int64       `json:"roles_total"`
	RecentLogins []LoginItem `json:"recent_logins"`
}

type LoginItem struct {
	ID          uuid.UUID  `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	LastLoginAt *time.Time `json:"last_login_at"`
	LastLoginIP string     `json:"last_login_ip"`
}

func (r *DashboardRepository) GetUsersSection(ctx context.Context) (*UsersSection, error) {
	section := &UsersSection{}
	// users 和 roles 表是全局资源，没有 tenant_id 列，使用普通 DB
	userDB := func() *gorm.DB { return r.db.WithContext(ctx) }

	userDB().Model(&model.User{}).Count(&section.Total)
	userDB().Model(&model.User{}).Where("status = ?", "active").Count(&section.Active)
	userDB().Model(&model.Role{}).Count(&section.RolesTotal)

	var users []model.User
	userDB().Model(&model.User{}).Where("last_login_at IS NOT NULL").Order("last_login_at DESC").Limit(10).Find(&users)
	for _, u := range users {
		section.RecentLogins = append(section.RecentLogins, LoginItem{
			ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, LastLoginAt: u.LastLoginAt, LastLoginIP: u.LastLoginIP,
		})
	}

	return section, nil
}
