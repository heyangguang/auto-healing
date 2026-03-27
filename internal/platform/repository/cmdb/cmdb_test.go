package cmdb

import (
	"testing"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
)

func TestBuildCMDBSyncUpdatesPreservesMaintenanceStatus(t *testing.T) {
	existing := platformmodel.CMDBItem{Status: "maintenance"}
	incoming := &platformmodel.CMDBItem{Status: "active", Name: "host"}

	updates := buildCMDBSyncUpdates(existing, incoming)
	if _, exists := updates["status"]; exists {
		t.Fatal("maintenance status should not be overwritten by sync update")
	}
}
