package playbook

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/company/auto-healing/internal/model"
)

func TestNormalizeEnhancedVariablesDefaultsTypeAndSkipsEmptyName(t *testing.T) {
	vars := []EnhancedVariable{
		{Name: "", Type: "number"},
		{Name: "region"},
		{Name: "retries", Type: "number"},
	}

	got := normalizeEnhancedVariables(vars)

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

func writeFile(path string) error {
	return os.WriteFile(path, []byte("variables: []\n"), 0o644)
}
