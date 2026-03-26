package repository

import (
	"context"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func createDictionarySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `
		CREATE TABLE sys_dictionaries (
			id TEXT PRIMARY KEY NOT NULL,
			dict_type TEXT NOT NULL,
			dict_key TEXT NOT NULL,
			label TEXT NOT NULL,
			label_en TEXT,
			color TEXT,
			tag_color TEXT,
			badge TEXT,
			icon TEXT,
			bg TEXT,
			extra TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_system BOOLEAN NOT NULL DEFAULT FALSE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	mustExec(t, db, `CREATE UNIQUE INDEX idx_dict_type_key ON sys_dictionaries(dict_type, dict_key);`)
}

func TestDictionaryUpsertBatchUpdatesIsActive(t *testing.T) {
	db := newSQLiteTestDB(t)
	createDictionarySchema(t, db)

	repo := &DictionaryRepository{db: db}
	itemID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	now := time.Now().UTC()
	mustExec(t, db, `
		INSERT INTO sys_dictionaries (id, dict_type, dict_key, label, sort_order, is_system, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, itemID.String(), "audit_resource_platform", "plugin", "插件", 1, true, true, now, now)

	err := repo.UpsertBatch(context.Background(), []model.Dictionary{{
		ID:        itemID,
		DictType:  "audit_resource_platform",
		DictKey:   "plugin",
		Label:     "插件",
		SortOrder: 1,
		IsSystem:  true,
		IsActive:  false,
		UpdatedAt: now.Add(time.Minute),
	}})
	if err != nil {
		t.Fatalf("UpsertBatch() error = %v", err)
	}

	saved, err := repo.GetByID(context.Background(), itemID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if saved.IsActive {
		t.Fatal("saved.IsActive = true, want false after upsert")
	}
}
