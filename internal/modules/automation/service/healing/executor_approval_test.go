package healing

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMarkInstanceWaitingApprovalUsesExistingWaitingState(t *testing.T) {
	db := newHealingTestDB(t)
	mustExecHealing(t, db, `
		CREATE TABLE flow_instances (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			flow_id TEXT,
			status TEXT NOT NULL,
			node_states TEXT,
			updated_at DATETIME
		);
	`)
	mustExecHealing(t, db, `
		CREATE TABLE healing_flows (
			id TEXT PRIMARY KEY NOT NULL,
			name TEXT,
			nodes TEXT,
			edges TEXT
		);
	`)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	instanceID := uuid.MustParse("34343434-3434-3434-3434-343434343434")
	flowID := uuid.MustParse("78787878-7878-7878-7878-787878787878")
	ctx := repository.WithTenantID(context.Background(), tenantID)
	mustExecHealing(t, db, `INSERT INTO healing_flows (id, name, nodes, edges) VALUES (?, ?, ?, ?)`, flowID.String(), "flow", "[]", "[]")
	mustExecHealing(t, db, `INSERT INTO flow_instances (id, tenant_id, flow_id, status, node_states) VALUES (?, ?, ?, ?, ?)`, instanceID.String(), tenantID.String(), flowID.String(), model.FlowInstanceStatusWaitingApproval, "{}")

	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	executor := &FlowExecutor{instanceRepo: automationrepo.NewFlowInstanceRepository()}

	instance := &model.FlowInstance{ID: instanceID, NodeStates: model.JSON{}}
	task := &model.ApprovalTask{ID: uuid.MustParse("56565656-5656-5656-5656-565656565656")}
	settings := approvalSettings{title: "approval", timeoutAt: time.Now().Add(time.Hour)}
	if err := executor.markInstanceWaitingApproval(ctx, instance, "approval-node", task, settings); err != nil {
		t.Fatalf("markInstanceWaitingApproval(): %v", err)
	}

	var nodeStates string
	if err := db.Table("flow_instances").Select("node_states").Where("id = ?", instanceID.String()).Scan(&nodeStates).Error; err != nil {
		t.Fatalf("read node_states: %v", err)
	}
	if nodeStates == "" || nodeStates == "{}" {
		t.Fatalf("node_states after approval = %s, want persisted approval payload", nodeStates)
	}
}

func newHealingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "healing.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func mustExecHealing(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
