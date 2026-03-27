package repository

import (
	"testing"

	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"gorm.io/gorm"
)

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

var WithTenantID = platformrepo.WithTenantID
