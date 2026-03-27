package healing

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestMarkIncidentsScannedWithoutRulesSyncsSkippedStatusWithTenantContext(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			scanned BOOLEAN,
			updated_at DATETIME
		);
	`)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.MustParse("71717171-7171-7171-7171-717171717171")
	incidentID := uuid.MustParse("81818181-8181-8181-8181-818181818181")
	mustExecHealing(t, db, `INSERT INTO incidents (id, tenant_id, healing_status, scanned) VALUES (?, ?, ?, ?)`, incidentID.String(), tenantID.String(), "pending", false)

	scheduler := &Scheduler{incidentRepo: incidentrepo.NewIncidentRepository()}
	scheduler.markIncidentsScannedWithoutRules(context.Background(), []model.Incident{{
		ID:       incidentID,
		TenantID: &tenantID,
	}})

	assertIncidentState(t, db, incidentID, "skipped", true)
}

func TestProcessIncidentWithoutMatchedRuleMarksIncidentSkipped(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
		CREATE TABLE incidents (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			healing_status TEXT,
			scanned BOOLEAN,
			updated_at DATETIME
		);
	`)

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	tenantID := uuid.MustParse("91919191-9191-9191-9191-919191919191")
	incidentID := uuid.MustParse("a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1")
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	mustExecHealing(t, db, `INSERT INTO incidents (id, tenant_id, healing_status, scanned) VALUES (?, ?, ?, ?)`, incidentID.String(), tenantID.String(), "pending", false)

	scheduler := &Scheduler{}
	incident := &model.Incident{ID: incidentID, TenantID: &tenantID}
	scheduler.incidentRepo = incidentrepo.NewIncidentRepository()
	scheduler.processIncident(ctx, incident, nil)

	assertIncidentState(t, db, incidentID, "skipped", true)
}

func assertIncidentState(t *testing.T, db *gorm.DB, incidentID uuid.UUID, wantStatus string, wantScanned bool) {
	t.Helper()
	type row struct {
		HealingStatus string
		Scanned       bool
	}
	var incidentRow row
	if err := db.Table("incidents").Select("healing_status, scanned").Where("id = ?", incidentID.String()).Scan(&incidentRow).Error; err != nil {
		t.Fatalf("read incident: %v", err)
	}
	if incidentRow.HealingStatus != wantStatus {
		t.Fatalf("healing_status = %s, want %s", incidentRow.HealingStatus, wantStatus)
	}
	if incidentRow.Scanned != wantScanned {
		t.Fatalf("scanned = %v, want %v", incidentRow.Scanned, wantScanned)
	}
}
