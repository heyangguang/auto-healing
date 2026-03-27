package runtime

import "testing"

func TestNewManagerBuildsAllSchedulers(t *testing.T) {
	manager := NewManager()
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
