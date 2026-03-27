package repository

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
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

	if err := countModel(db, &model.NotificationChannel{}, &section.ChannelsTotal); err != nil {
		return nil, err
	}
	if err := countModel(db, &model.NotificationTemplate{}, &section.TemplatesTotal); err != nil {
		return nil, err
	}
	if err := countModel(db, &model.NotificationLog{}, &section.LogsTotal); err != nil {
		return nil, err
	}
	rate, err := calculateDeliveryRate(db, section.LogsTotal)
	if err != nil {
		return nil, err
	}
	section.DeliveryRate = rate
	if err := scanStatusCounts(db, &model.NotificationChannel{}, "type", &section.ByChannelType); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(db, &model.NotificationLog{}, "status", &section.ByLogStatus); err != nil {
		return nil, err
	}
	if err := scanTrendPoints(db, &model.NotificationLog{}, "created_at", time.Now().AddDate(0, 0, -7), &section.Trend7d); err != nil {
		return nil, err
	}
	recentLogs, err := listNotificationLogs(db.Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentLogs = recentLogs
	failedLogs, err := listNotificationLogs(db.Where("status = ?", "failed").Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.FailedLogs = failedLogs
	return section, nil
}

func calculateDeliveryRate(db *gorm.DB, total int64) (float64, error) {
	if total == 0 {
		return 0, nil
	}
	var sentCount int64
	if err := countModel(db.Where("status IN ?", []string{"sent", "delivered"}), &model.NotificationLog{}, &sentCount); err != nil {
		return 0, err
	}
	return float64(sentCount) / float64(total) * 100, nil
}

func listNotificationLogs(query *gorm.DB) ([]NotifLogItem, error) {
	var logs []model.NotificationLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	items := make([]NotifLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, NotifLogItem{
			ID:        log.ID,
			Subject:   log.Subject,
			Status:    log.Status,
			CreatedAt: log.CreatedAt,
		})
	}
	return items, nil
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
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err := countModel(newDB(), &model.GitRepository{}, &section.ReposTotal); err != nil {
		return nil, err
	}
	rate, err := calculateGitSyncRate(newDB())
	if err != nil {
		return nil, err
	}
	section.SyncSuccessRate = rate
	repos, err := listGitRepos(newDB().Order("name"))
	if err != nil {
		return nil, err
	}
	section.Repos = repos
	syncs, err := listGitSyncs(newDB().Preload("Repository", "tenant_id = ?", tenantID).Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentSyncs = syncs
	return section, nil
}

func calculateGitSyncRate(db *gorm.DB) (float64, error) {
	var total, success int64
	if err := countModel(db, &model.GitSyncLog{}, &total); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	if err := countModel(db.Where("status = ?", "success"), &model.GitSyncLog{}, &success); err != nil {
		return 0, err
	}
	return float64(success) / float64(total) * 100, nil
}

func listGitRepos(query *gorm.DB) ([]GitRepoItem, error) {
	var repos []model.GitRepository
	if err := query.Find(&repos).Error; err != nil {
		return nil, err
	}
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
	return items, nil
}

func listGitSyncs(query *gorm.DB) ([]GitSyncItem, error) {
	var syncs []model.GitSyncLog
	if err := query.Find(&syncs).Error; err != nil {
		return nil, err
	}
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
	return items, nil
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
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	tenantID, err := RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err := countModel(newDB(), &model.Playbook{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "ready"), &model.Playbook{}, &section.Ready); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &model.Playbook{}, "status", &section.ByStatus); err != nil {
		return nil, err
	}
	scans, err := listPlaybookScans(newDB().Preload("Playbook", "tenant_id = ?", tenantID).Order("created_at DESC").Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentScans = scans
	return section, nil
}

func listPlaybookScans(query *gorm.DB) ([]ScanItem, error) {
	var scans []model.PlaybookScanLog
	if err := query.Find(&scans).Error; err != nil {
		return nil, err
	}
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
	return items, nil
}

type SecretsSection struct {
	Total      int64         `json:"total"`
	Active     int64         `json:"active"`
	ByType     []StatusCount `json:"by_type"`
	ByAuthType []StatusCount `json:"by_auth_type"`
}

func (r *DashboardRepository) GetSecretsSection(ctx context.Context) (*SecretsSection, error) {
	section := &SecretsSection{}
	newDB := func() *gorm.DB { return r.tenantDB(ctx) }
	if err := countModel(newDB(), &model.SecretsSource{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "active"), &model.SecretsSource{}, &section.Active); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &model.SecretsSource{}, "type", &section.ByType); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &model.SecretsSource{}, "auth_type", &section.ByAuthType); err != nil {
		return nil, err
	}
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

	if err := countModel(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ?", tenantID).
		Distinct("users.id"), &model.User{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ? AND users.status = ?", tenantID, "active").
		Distinct("users.id"), &model.User{}, &section.Active); err != nil {
		return nil, err
	}
	if err := countModel(db.Model(&model.Role{}).
		Where("scope = ?", "tenant").
		Where("tenant_id IS NULL OR tenant_id = ?", tenantID), &model.Role{}, &section.RolesTotal); err != nil {
		return nil, err
	}
	recentLogins, err := listRecentLogins(db.Table("users").
		Joins("JOIN user_tenant_roles utr ON utr.user_id = users.id").
		Where("utr.tenant_id = ? AND users.last_login_at IS NOT NULL", tenantID).
		Distinct("users.id").
		Order("users.last_login_at DESC").
		Limit(10))
	if err != nil {
		return nil, err
	}
	section.RecentLogins = recentLogins
	return section, nil
}

func listRecentLogins(query *gorm.DB) ([]LoginItem, error) {
	var users []model.User
	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}
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
	return items, nil
}
