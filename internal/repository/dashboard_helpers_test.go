package repository

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCountModelPropagatesErrors(t *testing.T) {
	db := testDB(t)
	dest := int64(0)
	err := countModel(db.Model(&model.NotificationChannel{}), &model.NotificationChannel{}, &dest)
	if err == nil {
		t.Fatalf("expected countModel to fail when table missing")
	}
}

func TestScanStatusCountsPropagatesErrors(t *testing.T) {
	db := testDB(t)
	var dest []StatusCount
	err := scanStatusCounts(db.Model(&model.NotificationChannel{}), &model.NotificationChannel{}, "type", &dest)
	if err == nil {
		t.Fatalf("scanStatusCounts should return error when query fails")
	}
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}
