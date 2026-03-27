package cmdb

import (
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestBuildCMDBSyncUpdatesPreservesMaintenanceStatus(t *testing.T) {
	existing := model.CMDBItem{Status: "maintenance"}
	incoming := &model.CMDBItem{Status: "active", Name: "host"}

	updates := buildCMDBSyncUpdates(existing, incoming)
	if _, exists := updates["status"]; exists {
		t.Fatal("maintenance status should not be overwritten by sync update")
	}
}
