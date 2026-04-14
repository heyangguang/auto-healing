package playbook

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

func TestNormalizeEnhancedVariablesDefaultsTypeAndSkipsEmptyName(t *testing.T) {
	vars := []EnhancedVariable{
		{Name: "", Type: "number"},
		{Name: "region"},
		{Name: "retries", Type: "number"},
	}

	got := normalizeEnhancedVariables(vars, "playbooks/test.yml")

	if len(got) != 2 {
		t.Fatalf("normalizeEnhancedVariables() len = %d, want 2", len(got))
	}
	if got["region"].Type != "string" {
		t.Fatalf("region type = %q, want string", got["region"].Type)
	}
	if got["retries"].Type != "number" {
		t.Fatalf("retries type = %q, want number", got["retries"].Type)
	}
}

func TestNormalizeEnhancedVariablesFiltersByPlaybookScope(t *testing.T) {
	vars := []EnhancedVariable{
		{Name: "shared"},
		{Name: "service_only", Playbooks: []string{"playbooks/service.yml"}},
		{Name: "cpu_only", Playbooks: []string{"playbooks/cpu.yml"}},
	}

	got := normalizeEnhancedVariables(vars, "playbooks/service.yml")

	if len(got) != 2 {
		t.Fatalf("normalizeEnhancedVariables() len = %d, want 2", len(got))
	}
	if _, ok := got["shared"]; !ok {
		t.Fatal("shared not found")
	}
	if _, ok := got["service_only"]; !ok {
		t.Fatal("service_only not found")
	}
	if _, ok := got["cpu_only"]; ok {
		t.Fatal("cpu_only should be filtered out")
	}
}

func TestFindEnhancedConfigPathPrefersKnownOrder(t *testing.T) {
	dir := t.TempDir()
	second := filepath.Join(dir, ".auto-healing.yaml")
	first := filepath.Join(dir, ".auto-healing.yml")
	if err := writeFile(second); err != nil {
		t.Fatalf("write second config: %v", err)
	}
	if err := writeFile(first); err != nil {
		t.Fatalf("write first config: %v", err)
	}

	got := findEnhancedConfigPath(dir)

	if got != first {
		t.Fatalf("findEnhancedConfigPath() = %q, want %q", got, first)
	}
}

func TestBuildVariableTypeMapIgnoresInvalidEntries(t *testing.T) {
	vars := model.JSONArray{
		map[string]any{"name": "region", "type": "string"},
		map[string]any{"name": "retries"},
		map[string]any{"type": "number"},
		"bad",
	}

	got := buildVariableTypeMap(vars)

	if len(got) != 2 {
		t.Fatalf("buildVariableTypeMap() len = %d, want 2", len(got))
	}
	if got["region"] != "string" {
		t.Fatalf("region type = %q, want string", got["region"])
	}
	if got["retries"] != "" {
		t.Fatalf("retries type = %q, want empty string", got["retries"])
	}
}

func TestBuildScannedVariablesScopedModeUsesAllowList(t *testing.T) {
	scanner := &VariableScanner{
		variables: map[string]*ScannedVariable{
			"lab_script_path": {Name: "lab_script_path", Type: "string"},
			"fault_type":      {Name: "fault_type", Type: "string"},
			"disk_fill_file":  {Name: "disk_fill_file", Type: "string"},
		},
	}

	config := ParsedEnhancedConfig{
		ExposureMode: "scoped",
		Variables: map[string]*EnhancedVariable{
			"lab_script_path": {Name: "lab_script_path", Type: "string"},
			"disk_fill_file":  {Name: "disk_fill_file", Type: "string"},
		},
	}

	got := buildScannedVariables(scanner, config)
	if len(got) != 2 {
		t.Fatalf("buildScannedVariables() len = %d, want 2", len(got))
	}
	names := buildVariableTypeMap(got)
	if _, ok := names["fault_type"]; ok {
		t.Fatal("fault_type should be hidden in scoped mode")
	}
	if _, ok := names["lab_script_path"]; !ok {
		t.Fatal("lab_script_path should remain")
	}
	if _, ok := names["disk_fill_file"]; !ok {
		t.Fatal("disk_fill_file should remain")
	}
}

func TestMergeVariablesDropsMissingScannedOutEntries(t *testing.T) {
	svc := &Service{}
	userVars := model.JSONArray{
		map[string]any{"name": "lab_script_path", "type": "string"},
		map[string]any{"name": "fault_type", "type": "string"},
	}
	scannedVars := model.JSONArray{
		map[string]any{"name": "lab_script_path", "type": "string"},
	}

	got := svc.mergeVariables(userVars, scannedVars)
	if len(got) != 1 {
		t.Fatalf("mergeVariables() len = %d, want 1", len(got))
	}
	names := buildVariableTypeMap(got)
	if _, ok := names["fault_type"]; ok {
		t.Fatal("fault_type should have been removed")
	}
}

func TestDetectChangedVariablesReturnsAddedRemovedAndTypeChanged(t *testing.T) {
	svc := &Service{}
	snapshot := model.JSONArray{
		map[string]any{"name": "region", "type": "string"},
		map[string]any{"name": "retries", "type": "number"},
	}
	newVars := map[string]string{
		"region": "boolean",
		"host":   "string",
	}

	got := svc.detectChangedVariables(snapshot, newVars)
	sort.Strings(got)

	want := []string{"host", "region", "retries"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("detectChangedVariables()[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestRoleSearchPathsIncludesPlaybookScopedRoles(t *testing.T) {
	scanner := &VariableScanner{basePath: "/repo"}
	currentFile := "/repo/playbooks/fault_recovery_suite.yml"

	got := scanner.roleSearchPaths(currentFile, "fault_lab_context")

	want := []string{
		"/repo/roles/fault_lab_context",
		"/repo/playbooks/roles/fault_lab_context",
	}
	if len(got) != len(want) {
		t.Fatalf("roleSearchPaths() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("roleSearchPaths()[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestEvalWhenClauseUsesKnownRuntimeValue(t *testing.T) {
	vars := map[string]any{"fault_type": "service_down"}

	if !evalWhenClause("fault_type == 'service_down'", vars) {
		t.Fatal("expected matching clause to be true")
	}
	if evalWhenClause("fault_type == 'disk_full'", vars) {
		t.Fatal("expected non-matching clause to be false")
	}
}

func TestScanFileFollowsKnownExecutionPath(t *testing.T) {
	repoRoot := t.TempDir()
	playbooksDir := filepath.Join(repoRoot, "playbooks")
	serviceRoleDir := filepath.Join(playbooksDir, "roles", "fault_lab_service", "tasks")
	diskRoleDir := filepath.Join(playbooksDir, "roles", "fault_lab_disk", "tasks")

	for _, dir := range []string{playbooksDir, serviceRoleDir, diskRoleDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}

	wrapper := filepath.Join(playbooksDir, "service_down_recover.yml")
	suite := filepath.Join(playbooksDir, "fault_recovery_suite.yml")
	serviceTask := filepath.Join(serviceRoleDir, "main.yml")
	diskTask := filepath.Join(diskRoleDir, "main.yml")

	if err := os.WriteFile(wrapper, []byte("---\n- import_playbook: fault_recovery_suite.yml\n  vars:\n    fault_type: service_down\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(wrapper): %v", err)
	}
	if err := os.WriteFile(suite, []byte("---\n- hosts: all\n  tasks:\n    - ansible.builtin.include_role:\n        name: fault_lab_service\n      when: fault_type == 'service_down'\n    - ansible.builtin.include_role:\n        name: fault_lab_disk\n      when: fault_type == 'disk_full'\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(suite): %v", err)
	}
	if err := os.WriteFile(serviceTask, []byte("---\n- name: service task\n  ansible.builtin.debug:\n    msg: ok\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(serviceTask): %v", err)
	}
	if err := os.WriteFile(diskTask, []byte("---\n- name: disk task\n  ansible.builtin.debug:\n    msg: nope\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(diskTask): %v", err)
	}

	scanner := &VariableScanner{basePath: repoRoot}
	if err := scanner.ScanFile(wrapper); err != nil {
		t.Fatalf("ScanFile() error = %v", err)
	}

	relService, _ := filepath.Rel(repoRoot, serviceTask)
	relDisk, _ := filepath.Rel(repoRoot, diskTask)
	foundService := false
	foundDisk := false
	for path := range scanner.scannedFiles {
		if rel, err := filepath.Rel(repoRoot, path); err == nil {
			if rel == relService {
				foundService = true
			}
			if rel == relDisk {
				foundDisk = true
			}
		}
	}
	if !foundService {
		t.Fatal("service role task should be scanned")
	}
	if foundDisk {
		t.Fatal("disk role task should not be scanned for service_down path")
	}
}

func TestRoleScanUsesMainEntrypointInsteadOfWholeTasksDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	roleTasksDir := filepath.Join(repoRoot, "playbooks", "roles", "sample_role", "tasks")
	if err := os.MkdirAll(roleTasksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(roleTasksDir): %v", err)
	}

	mainFile := filepath.Join(roleTasksDir, "main.yml")
	extraFile := filepath.Join(roleTasksDir, "extra.yml")
	if err := os.WriteFile(mainFile, []byte("---\n- name: main task\n  ansible.builtin.debug:\n    msg: ok\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main): %v", err)
	}
	if err := os.WriteFile(extraFile, []byte("---\n- name: extra task\n  ansible.builtin.debug:\n    msg: skip\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(extra): %v", err)
	}

	scanner := &VariableScanner{basePath: repoRoot}
	scanner.scanRole("sample_role", filepath.Join(repoRoot, "playbooks", "site.yml"), map[string]any{})
	if scanner.err != nil {
		t.Fatalf("scanRole() err = %v", scanner.err)
	}

	if !scanner.scannedFiles[mainFile] {
		t.Fatal("main task should be scanned")
	}
	if scanner.scannedFiles[extraFile] {
		t.Fatal("unreferenced extra task should not be scanned")
	}
}

func writeFile(path string) error {
	return os.WriteFile(path, []byte("variables: []\n"), 0o644)
}
