package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func createUserActivitySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE user_recents (
			id TEXT PRIMARY KEY NOT NULL,
			user_id TEXT NOT NULL,
			menu_key TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			accessed_at DATETIME,
			tenant_id TEXT
		);
	`)
	mustExec(t, db, `CREATE UNIQUE INDEX idx_user_recent ON user_recents(user_id, menu_key, tenant_id);`)
}

func TestUserActivityUpsertRecentReturnsDeleteErrorWhenTrimFails(t *testing.T) {
	db := newSQLiteTestDB(t)
	createUserActivitySchema(t, db)

	repo := &UserActivityRepository{db: db}
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Now().UTC()

	for i := 0; i < maxRecentItems; i++ {
		mustExec(t, db, `
			INSERT INTO user_recents (id, user_id, menu_key, name, path, accessed_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, uuid.NewString(), userID.String(), "menu_"+uuid.NewString(), "name", "/path", now.Add(-time.Duration(i+1)*time.Minute), tenantID.String())
	}
	mustExec(t, db, `
		CREATE TRIGGER block_recent_delete
		BEFORE DELETE ON user_recents
		BEGIN
			SELECT RAISE(FAIL, 'delete blocked');
		END;
	`)

	err := repo.UpsertRecent(WithTenantID(context.Background(), tenantID), &model.UserRecent{
		ID:      uuid.New(),
		UserID:  userID,
		MenuKey: "current",
		Name:    "Current",
		Path:    "/current",
	})
	if err == nil {
		t.Fatal("UpsertRecent() error = nil, want delete failure")
	}
	if !strings.Contains(err.Error(), "delete blocked") {
		t.Fatalf("UpsertRecent() error = %v, want delete blocked", err)
	}
}

func TestUserActivityUpsertRecentTrimsOnlyCurrentTenant(t *testing.T) {
	db := newSQLiteTestDB(t)
	createUserActivitySchema(t, db)

	repo := &UserActivityRepository{db: db}
	tenantA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	userID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	now := time.Now().UTC()

	seedTenantRecents(t, db, tenantA, userID, now)
	seedTenantRecents(t, db, tenantB, userID, now)

	if err := repo.UpsertRecent(WithTenantID(context.Background(), tenantA), &model.UserRecent{
		ID:      uuid.New(),
		UserID:  userID,
		MenuKey: "tenant-a-new",
		Name:    "Tenant A New",
		Path:    "/tenant-a/new",
	}); err != nil {
		t.Fatalf("UpsertRecent() error = %v", err)
	}

	countA := countTenantRecents(t, db, tenantA, userID)
	countB := countTenantRecents(t, db, tenantB, userID)
	if countA != maxRecentItems {
		t.Fatalf("tenant A count = %d, want %d", countA, maxRecentItems)
	}
	if countB != maxRecentItems {
		t.Fatalf("tenant B count = %d, want %d", countB, maxRecentItems)
	}
}

func TestUserActivityUpsertRecentRecoversFromDuplicateCreate(t *testing.T) {
	db := newSQLiteTestDB(t)
	createUserActivitySchema(t, db)

	tenantID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	userID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	ctx := WithTenantID(context.Background(), tenantID)
	existingID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	now := time.Now().UTC().Add(-time.Minute)
	mustExec(t, db, `
		INSERT INTO user_recents (id, user_id, menu_key, name, path, accessed_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, existingID.String(), userID.String(), "same-menu", "old", "/old", now, tenantID.String())

	recent := &model.UserRecent{
		ID:       uuid.New(),
		UserID:   userID,
		MenuKey:  "same-menu",
		Name:     "new",
		Path:     "/same",
		TenantID: &tenantID,
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return upsertRecentRecord(tx, ctx, recent)
	})
	if err != nil {
		t.Fatalf("upsertRecentRecord() error = %v", err)
	}

	var count int64
	if err := db.Table("user_recents").
		Where("tenant_id = ? AND user_id = ? AND menu_key = ?", tenantID.String(), userID.String(), "same-menu").
		Count(&count).Error; err != nil {
		t.Fatalf("count recents: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if recent.ID != existingID {
		t.Fatalf("recent.ID = %s, want existing %s", recent.ID, existingID)
	}
}

func TestUserActivityUpsertRecentFillsExistingRecordFields(t *testing.T) {
	db := newSQLiteTestDB(t)
	createUserActivitySchema(t, db)

	repo := &UserActivityRepository{db: db}
	tenantID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	existingID := uuid.MustParse("66666666-7777-8888-9999-000000000000")
	now := time.Now().UTC().Add(-time.Minute)
	mustExec(t, db, `
		INSERT INTO user_recents (id, user_id, menu_key, name, path, accessed_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, existingID.String(), userID.String(), "menu", "old", "/old", now, tenantID.String())

	recent := &model.UserRecent{
		UserID:  userID,
		MenuKey: "menu",
		Name:    "new",
		Path:    "/new",
	}
	if err := repo.UpsertRecent(WithTenantID(context.Background(), tenantID), recent); err != nil {
		t.Fatalf("UpsertRecent() error = %v", err)
	}
	if recent.ID != existingID {
		t.Fatalf("recent.ID = %s, want %s", recent.ID, existingID)
	}
	if recent.AccessedAt.IsZero() {
		t.Fatal("recent.AccessedAt should be populated")
	}
}

func seedTenantRecents(t *testing.T, db *gorm.DB, tenantID, userID uuid.UUID, now time.Time) {
	t.Helper()
	for i := 0; i < maxRecentItems; i++ {
		mustExec(t, db, `
			INSERT INTO user_recents (id, user_id, menu_key, name, path, accessed_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, uuid.NewString(), userID.String(), tenantID.String()+"_"+uuid.NewString(), "name", "/path", now.Add(-time.Duration(i+1)*time.Minute), tenantID.String())
	}
}

func countTenantRecents(t *testing.T, db *gorm.DB, tenantID, userID uuid.UUID) int64 {
	t.Helper()
	var count int64
	if err := db.Table("user_recents").
		Where("tenant_id = ? AND user_id = ?", tenantID.String(), userID.String()).
		Count(&count).Error; err != nil {
		t.Fatalf("count tenant recents: %v", err)
	}
	return count
}
