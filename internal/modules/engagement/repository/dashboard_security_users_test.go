package repository

import (
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRecentTenantLoginsQueryAvoidsDistinctForPostgres(t *testing.T) {
	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "host=localhost user=postgres password=postgres dbname=auto_healing sslmode=disable"}),
		&gorm.Config{DryRun: true},
	)
	if err != nil {
		t.Fatalf("open dry-run postgres db: %v", err)
	}

	var users []model.User
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	stmt := recentTenantLoginsQuery(db, tenantID).Find(&users).Statement
	sql := stmt.SQL.String()

	if !strings.Contains(sql, "EXISTS") {
		t.Fatalf("recentTenantLoginsQuery SQL = %q, want EXISTS tenant scope", sql)
	}
	if strings.Contains(strings.ToUpper(sql), "SELECT DISTINCT") {
		t.Fatalf("recentTenantLoginsQuery SQL = %q, should not use DISTINCT", sql)
	}
}
