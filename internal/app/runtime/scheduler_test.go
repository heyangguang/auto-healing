package runtime

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewManagerBuildsAllSchedulers(t *testing.T) {
	manager := NewManagerWithDeps(ManagerDeps{DB: newRuntimeTestDB(t)})
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if manager.pluginScheduler == nil {
		t.Fatal("expected plugin scheduler")
	}
	if manager.executionScheduler == nil {
		t.Fatal("expected execution scheduler")
	}
	if manager.gitScheduler == nil {
		t.Fatal("expected git scheduler")
	}
	if manager.notificationScheduler == nil {
		t.Fatal("expected notification scheduler")
	}
	if manager.blacklistScheduler == nil {
		t.Fatal("expected blacklist scheduler")
	}
}

func newRuntimeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "runtime.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
