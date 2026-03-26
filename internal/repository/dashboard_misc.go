package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	db := r.tenantDB(ctx)

	countModel(db, &model.NotificationChannel{}, &section.ChannelsTotal)
	countModel(db, &model.NotificationTemplate{}, &section.TemplatesTotal)
	countModel(db, &model.NotificationLog{}, &section.LogsTotal)
	section.DeliveryRate = calculateDeliveryRate(db, section.LogsTotal)
	scanStatusCounts(db, &model.NotificationChannel{}, "type", &section.ByChannelType)
	scanStatusCounts(db, &model.NotificationLog{}, "status", &section.ByLogStatus)
	scanTrendPoints(db, &model.NotificationLog{}, "created_at", time.Now().AddDate(0, 0, -7), &section.Trend7d)
	section.RecentLogs = listNotificationLogs(db.Order("created_at DESC").Limit(10))
	section.FailedLogs = listNotificationLogs(db.Where("status = ?", "failed").Order("created_at DESC").Limit(10))
	return section, nil
}

func calculateDeliveryRate(db *gorm.DB, total int64) float64 {
	if total == 0 {
		return 0
	}
	var sentCount int64
	countModel(db.Where("status IN ?", []string{"sent", "delivered"}), &model.NotificationLog{}, &sentCount)
	return float64(sentCount) / float64(total) * 100
}

func listNotificationLogs(query *gorm.DB) []NotifLogItem {
	var logs []model.NotificationLog
	query.Find(&logs)
	items := make([]NotifLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, NotifLogItem{
			ID:        log.ID,
			Subject:   log.Subject,
			Status:    log.Status,
			CreatedAt: log.CreatedAt,
		})
	}
	return items
}

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
	db := r.tenantDB(ctx)

	countModel(db, &model.GitRepository{}, &section.ReposTotal)
	section.SyncSuccessRate = calculateGitSyncRate(db)
	section.Repos = listGitRepos(db.Order("name"))
	section.RecentSyncs = listGitSyncs(db.Preload("Repository").Order("created_at DESC").Limit(10))
	return section, nil
}

func calculateGitSyncRate(db *gorm.DB) float64 {
	var total, success int64
	countModel(db, &model.GitSyncLog{}, &total)
	if total == 0 {
		return 0
	}
	countModel(db.Where("status = ?", "success"), &model.GitSyncLog{}, &success)
	return float64(success) / float64(total) * 100
}

func listGitRepos(query *gorm.DB) []GitRepoItem {
	var repos []model.GitRepository
	query.Find(&repos)
	items := make([]GitRepoItem, 0, len(repos))
	for _, repo := range repos {
		items = append(items, GitRepoItem{
			ID:         repo.ID,
			Name:       repo.Name,
			URL:        repo.URL,
			Status:     repo.Status,
			Branch:     repo.DefaultBranch,
			LastSyncAt: repo.LastSyncAt,
		})
	}
	return items
}

func listGitSyncs(query *gorm.DB) []GitSyncItem {
	var syncs []model.GitSyncLog
	query.Find(&syncs)
	items := make([]GitSyncItem, 0, len(syncs))
	for _, sync := range syncs {
		repoName := ""
		if sync.Repository != nil {
			repoName = sync.Repository.Name
		}
		items = append(items, GitSyncItem{
			ID:        sync.ID,
			RepoName:  repoName,
			Status:    sync.Status,
			CreatedAt: sync.CreatedAt,
		})
	}
	return items
}

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
	db := r.tenantDB(ctx)

	countModel(db, &model.Playbook{}, &section.Total)
	countModel(db.Where("status = ?", "ready"), &model.Playbook{}, &section.Ready)
	scanStatusCounts(db, &model.Playbook{}, "status", &section.ByStatus)
	section.RecentScans = listPlaybookScans(db.Preload("Playbook").Order("created_at DESC").Limit(10))
	return section, nil
}

func listPlaybookScans(query *gorm.DB) []ScanItem {
	var scans []model.PlaybookScanLog
	query.Find(&scans)
	items := make([]ScanItem, 0, len(scans))
	for _, scan := range scans {
		playbookName := ""
		if scan.Playbook != nil {
			playbookName = scan.Playbook.Name
		}
		items = append(items, ScanItem{
			ID:           scan.ID,
			PlaybookName: playbookName,
			Status:       scan.TriggerType,
			CreatedAt:    scan.CreatedAt,
		})
	}
	return items
}

type SecretsSection struct {
	Total      int64         `json:"total"`
	Active     int64         `json:"active"`
	ByType     []StatusCount `json:"by_type"`
	ByAuthType []StatusCount `json:"by_auth_type"`
}

func (r *DashboardRepository) GetSecretsSection(ctx context.Context) (*SecretsSection, error) {
	section := &SecretsSection{}
	db := r.tenantDB(ctx)
	countModel(db, &model.SecretsSource{}, &section.Total)
	countModel(db.Where("status = ?", "active"), &model.SecretsSource{}, &section.Active)
	scanStatusCounts(db, &model.SecretsSource{}, "type", &section.ByType)
	scanStatusCounts(db, &model.SecretsSource{}, "auth_type", &section.ByAuthType)
	return section, nil
}

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
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	db := r.db.WithContext(ctx)

	db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ?", tenantID).
		Distinct("users.id").
		Count(&section.Total)
	db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ? AND users.status = ?", tenantID, "active").
		Distinct("users.id").
		Count(&section.Active)
	db.Model(&model.Role{}).
		Where("scope = ?", "tenant").
		Where("tenant_id IS NULL OR tenant_id = ?", tenantID).
		Count(&section.RolesTotal)
	section.RecentLogins = listRecentLogins(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ? AND users.last_login_at IS NOT NULL", tenantID).
		Distinct("users.id").
		Order("users.last_login_at DESC").
		Limit(10))
	return section, nil
}

func listRecentLogins(query *gorm.DB) []LoginItem {
	var users []model.User
	query.Find(&users)
	items := make([]LoginItem, 0, len(users))
	for _, user := range users {
		items = append(items, LoginItem{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			LastLoginAt: user.LastLoginAt,
			LastLoginIP: user.LastLoginIP,
		})
	}
	return items
}
