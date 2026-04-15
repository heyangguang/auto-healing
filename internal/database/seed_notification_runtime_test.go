package database

import (
	"strings"
	"testing"
)

func TestNotificationAlignmentSQLCleansWeComAndFlowLogs(t *testing.T) {
	joined := strings.Join(notificationAlignmentSQL, "\n")

	if !strings.Contains(joined, "config - 'mentioned_list' - 'mentioned_mobile_list'") {
		t.Fatal("notification alignment is missing wecom mention cleanup")
	}
	if !strings.Contains(joined, "name ILIKE '%执行任务-开始%'") {
		t.Fatal("notification alignment is missing execution_started template correction")
	}
	if !strings.Contains(joined, "'manual_notification'") {
		t.Fatal("notification alignment is missing manual_notification migration")
	}
	if !strings.Contains(joined, "UPDATE execution_runs") {
		t.Fatal("notification alignment is missing triggered_by cleanup")
	}
	if !strings.Contains(joined, "workflow_instance_id = COALESCE") {
		t.Fatal("notification alignment is missing flow notification relation backfill")
	}
	if !strings.Contains(joined, "regexp_replace(COALESCE(n.subject, ''), '^\\[[^]]+\\]\\s*', '')") {
		t.Fatal("notification alignment is missing incident title normalization")
	}
}
