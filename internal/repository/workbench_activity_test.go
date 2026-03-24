package repository

import "testing"

func TestMapResourceTypeToActivityTypeSupportsHyphenatedAuditResources(t *testing.T) {
	tests := []struct {
		resourceType string
		want         string
	}{
		{"execution-tasks", "execution"},
		{"execution-runs", "execution"},
		{"healing-flows", "flow"},
		{"healing-rules", "rule"},
		{"tenant-roles", "access"},
		{"tenant-incidents", "ops"},
	}

	for _, tc := range tests {
		if got := mapResourceTypeToActivityType(tc.resourceType); got != tc.want {
			t.Fatalf("mapResourceTypeToActivityType(%q) = %q, want %q", tc.resourceType, got, tc.want)
		}
	}
}

func TestBuildActivityTextSupportsHyphenatedAuditResources(t *testing.T) {
	text := buildActivityText("create", "tenant-roles", "ops-reviewer")
	if text != "创建角色：ops-reviewer" {
		t.Fatalf("buildActivityText(create, tenant-roles) = %q", text)
	}

	text = buildActivityText("update", "execution-tasks", "acceptance-task")
	if text != "更新执行任务：acceptance-task" {
		t.Fatalf("buildActivityText(update, execution-tasks) = %q", text)
	}
}
