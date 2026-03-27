package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func createPlatformSettingsSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE platform_settings (
			key TEXT PRIMARY KEY NOT NULL,
			value TEXT NOT NULL,
			type TEXT NOT NULL,
			module TEXT NOT NULL,
			label TEXT NOT NULL,
			description TEXT,
			default_value TEXT,
			updated_at DATETIME,
			updated_by TEXT
		);
	`)
}

func TestPlatformSettingsGetIntValuePanicsOnDatabaseError(t *testing.T) {
	db := newSQLiteTestDB(t)
	repo := &PlatformSettingsRepository{db: db}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if !strings.Contains(toPanicMessage(r), "读取平台设置 email.smtp_port 失败") {
			t.Fatalf("panic = %v, want db failure message", r)
		}
	}()

	_ = repo.GetIntValue(context.Background(), "email.smtp_port", 587)
}

func TestPlatformSettingsGetIntValuePanicsOnInvalidInteger(t *testing.T) {
	db := newSQLiteTestDB(t)
	createPlatformSettingsSchema(t, db)
	repo := &PlatformSettingsRepository{db: db}
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO platform_settings (key, value, type, module, label, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "email.smtp_port", "invalid", "int", "email", "SMTP 端口", now)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if !strings.Contains(toPanicMessage(r), "平台设置 email.smtp_port 不是合法整数") {
			t.Fatalf("panic = %v, want invalid int message", r)
		}
	}()

	_ = repo.GetIntValue(context.Background(), "email.smtp_port", 587)
}

func TestPlatformSettingsGetIntValueReturnsDefaultOnMissingKey(t *testing.T) {
	db := newSQLiteTestDB(t)
	createPlatformSettingsSchema(t, db)
	repo := &PlatformSettingsRepository{db: db}

	if got := repo.GetIntValue(context.Background(), "email.smtp_port", 587); got != 587 {
		t.Fatalf("GetIntValue() = %d, want 587", got)
	}
}

func toPanicMessage(value interface{}) string {
	switch typed := value.(type) {
	case error:
		return typed.Error()
	case string:
		return typed
	default:
		return ""
	}
}

func newSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "platform-settings.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
