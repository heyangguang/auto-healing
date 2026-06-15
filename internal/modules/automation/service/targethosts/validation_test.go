package targethosts

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateActiveCMDBHostsAcceptsKnownActiveHosts(t *testing.T) {
	db := openValidationTestDB(t)
	tenantID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	insertValidationCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")

	err := ValidateActiveCMDBHosts(ctx, cmdbrepo.NewCMDBItemRepositoryWithDB(db), "118.196.22.79:2222,e2e-target-01")
	if err != nil {
		t.Fatalf("ValidateActiveCMDBHosts() error = %v", err)
	}
}

func TestNormalizeActiveCMDBHostsDeduplicatesAliasesByCMDBItem(t *testing.T) {
	db := openValidationTestDB(t)
	tenantID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	repo := cmdbrepo.NewCMDBItemRepositoryWithDB(db)
	insertValidationCMDBItem(t, db, tenantID, "e2e-target-01", "118.196.22.79", "active")

	normalized, err := NormalizeActiveCMDBHosts(ctx, repo, "e2e-target-01,118.196.22.79,e2e-target-01")
	if err != nil {
		t.Fatalf("NormalizeActiveCMDBHosts() error = %v", err)
	}
	if normalized != "e2e-target-01" {
		t.Fatalf("normalized = %q, want e2e-target-01", normalized)
	}
}

func TestValidateActiveCMDBHostsRejectsMissingOrInactiveHosts(t *testing.T) {
	db := openValidationTestDB(t)
	tenantID := uuid.New()
	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	insertValidationCMDBItem(t, db, tenantID, "inactive-host", "10.0.0.9", "maintenance")

	err := ValidateActiveCMDBHosts(ctx, cmdbrepo.NewCMDBItemRepositoryWithDB(db), "127.0.0.1,10.0.0.9")
	if err == nil {
		t.Fatal("ValidateActiveCMDBHosts() error = nil, want rejection")
	}
	if !strings.Contains(err.Error(), "127.0.0.1") || !strings.Contains(err.Error(), "10.0.0.9") {
		t.Fatalf("error %q should include rejected hosts", err.Error())
	}
}

func TestValidateActiveCMDBHostsRejectsEmptyTargets(t *testing.T) {
	if err := ValidateActiveCMDBHosts(context.Background(), nil, " , ; "); err == nil {
		t.Fatal("ValidateActiveCMDBHosts() error = nil, want empty target rejection")
	}
}

func openValidationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "target-host-validation.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			external_id TEXT,
			name TEXT,
			type TEXT,
			status TEXT,
			ip_address TEXT,
			hostname TEXT,
			raw_data TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create cmdb_items table: %v", err)
	}
	return db
}

func insertValidationCMDBItem(t *testing.T, db *gorm.DB, tenantID uuid.UUID, name, ipAddress, status string) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO cmdb_items (id, tenant_id, external_id, name, type, status, ip_address, hostname, raw_data)
		VALUES (?, ?, ?, ?, 'server', ?, ?, ?, '{}')
	`, uuid.NewString(), tenantID.String(), name, name, status, ipAddress, name).Error; err != nil {
		t.Fatalf("insert cmdb item: %v", err)
	}
}
