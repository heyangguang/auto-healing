package repository

import (
	"context"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"time"

	projection "github.com/company/auto-healing/internal/modules/engagement/projection"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err := countModel(newDB(), &projection.GitRepository{}, &section.ReposTotal); err != nil {
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
	if err := countModel(db, &projection.GitSyncLog{}, &total); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	if err := countModel(db.Where("status = ?", "success"), &projection.GitSyncLog{}, &success); err != nil {
		return 0, err
	}
	return float64(success) / float64(total) * 100, nil
}

func listGitRepos(query *gorm.DB) ([]GitRepoItem, error) {
	var repos []projection.GitRepository
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
	var syncs []projection.GitSyncLog
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
	tenantID, err := platformrepo.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err := countModel(newDB(), &projection.Playbook{}, &section.Total); err != nil {
		return nil, err
	}
	if err := countModel(newDB().Where("status = ?", "ready"), &projection.Playbook{}, &section.Ready); err != nil {
		return nil, err
	}
	if err := scanStatusCounts(newDB(), &projection.Playbook{}, "status", &section.ByStatus); err != nil {
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
	var scans []projection.PlaybookScanLog
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
