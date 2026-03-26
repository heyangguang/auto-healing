package database

import (
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedSiteMessagesFillsMissingRecords(t *testing.T) {
	db := newTestDB(t)
	useTestDB(t, db)
	createSiteMessagesTable(t, db)

	messages := buildSeedSiteMessages(time.Now().AddDate(0, 0, 90))
	existing := model.SiteMessage{
		ID:       uuid.New(),
		Category: messages[0].Category,
		Title:    messages[0].Title,
		Content:  messages[0].Content,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := SeedSiteMessages(); err != nil {
		t.Fatalf("SeedSiteMessages() error = %v", err)
	}

	var count int64
	if err := db.Model(&model.SiteMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if got, want := int(count), len(messages); got != want {
		t.Fatalf("site message count = %d, want %d", got, want)
	}

	var firstTitleCount int64
	if err := db.Model(&model.SiteMessage{}).Where("category = ? AND title = ?", messages[0].Category, messages[0].Title).Count(&firstTitleCount).Error; err != nil {
		t.Fatalf("Count(first title) error = %v", err)
	}
	if firstTitleCount != 1 {
		t.Fatalf("duplicate first seed inserted, count = %d", firstTitleCount)
	}
}

func TestSeedCommandBlacklistReturnsErrorWhenSchemaMissing(t *testing.T) {
	db := newTestDB(t)
	useTestDB(t, db)

	err := SeedCommandBlacklist()
	if err == nil {
		t.Fatal("SeedCommandBlacklist() error = nil, want schema failure")
	}
	if !strings.Contains(err.Error(), "command_blacklist") {
		t.Fatalf("error = %q, want command_blacklist hint", err)
	}
}

func TestMigrateModelIfMissingReturnsParseError(t *testing.T) {
	db := newTestDB(t)

	_, err := migrateModelIfMissing(db, 42)
	if err == nil {
		t.Fatal("migrateModelIfMissing() error = nil, want parse failure")
	}
	if !strings.Contains(err.Error(), "解析模型") {
		t.Fatalf("error = %q, want parse hint", err)
	}
}

func TestSyncRolePermissionsRemovesObsoletePermissions(t *testing.T) {
	db := newTestDB(t)
	useTestDB(t, db)
	createRolePermissionTable(t, db)

	roleID := uuid.New()
	keepPermID := uuid.New()
	removePermID := uuid.New()
	if err := db.Exec(`INSERT INTO role_permissions (id, role_id, permission_id) VALUES (?, ?, ?), (?, ?, ?)`,
		uuid.New().String(), roleID.String(), keepPermID.String(),
		uuid.New().String(), roleID.String(), removePermID.String(),
	).Error; err != nil {
		t.Fatalf("seed role_permissions error = %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return syncRolePermissions(tx, roleID, []string{"keep"}, map[string]uuid.UUID{"keep": keepPermID})
	})
	if err != nil {
		t.Fatalf("syncRolePermissions() error = %v", err)
	}

	var count int64
	if err := db.Table("role_permissions").Where("role_id = ? AND permission_id = ?", roleID.String(), removePermID.String()).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("obsolete permission remained, count = %d", count)
	}
}

func newTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("AutoMigrate() error = %v", err)
		}
	}
	return db
}

func useTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func createSiteMessagesTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	schema := `
CREATE TABLE site_messages (
	id TEXT PRIMARY KEY,
	tenant_id TEXT,
	target_tenant_id TEXT,
	category TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at DATETIME,
	expires_at DATETIME
)`
	if err := db.Exec(schema).Error; err != nil {
		t.Fatalf("create site_messages table error = %v", err)
	}
}

func createRolePermissionTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	schema := `
CREATE TABLE role_permissions (
	id TEXT PRIMARY KEY,
	role_id TEXT NOT NULL,
	permission_id TEXT NOT NULL
)`
	if err := db.Exec(schema).Error; err != nil {
		t.Fatalf("create role_permissions table error = %v", err)
	}
}
