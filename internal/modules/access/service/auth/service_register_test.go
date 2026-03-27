package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/company/auto-healing/internal/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegisterTxRollsBackWhenAttachTenantFails(t *testing.T) {
	db := openAuthTestDB(t)
	mustExecAuthTest(t, db, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT,
			phone TEXT,
			avatar_url TEXT,
			status TEXT,
			last_login_at DATETIME,
			last_login_ip TEXT,
			password_changed_at DATETIME,
			failed_login_count INTEGER,
			locked_until DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			is_platform_admin BOOLEAN
		);
	`)
	mustExecAuthTest(t, db, `
		CREATE TABLE roles (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT NOT NULL,
			display_name TEXT,
			description TEXT,
			is_system BOOLEAN,
			scope TEXT,
			tenant_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	svc := &Service{
		userRepo:   accessrepo.NewUserRepositoryWithDB(db),
		roleRepo:   accessrepo.NewRoleRepositoryWithDB(db),
		tenantRepo: accessrepo.NewTenantRepositoryWithDB(db),
		db:         db,
	}
	user := &model.User{
		ID:           uuid.New(),
		Username:     "rollback-user",
		Email:        "rollback@example.com",
		PasswordHash: "hashed",
		Status:       "active",
	}
	tenantID := uuid.New()

	err := svc.registerTx(context.Background(), user, nil, &tenantID)
	if err == nil {
		t.Fatal("registerTx() error = nil, want tenant attach failure")
	}

	var count int64
	if err := db.Table("users").Count(&count).Error; err != nil {
		t.Fatalf("count users error = %v", err)
	}
	if count != 0 {
		t.Fatalf("users count = %d, want 0 after rollback", count)
	}
}

func openAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db error = %v", err)
	}
	return db
}

func mustExecAuthTest(t *testing.T, db *gorm.DB, sql string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql error = %v\nsql=%s", err, sql)
	}
}
