package healing

import "testing"

func TestDryRunSetVariableSupportsBatchVariables(t *testing.T) {
	executor := &DryRunExecutor{}
	result := DryRunNodeResult{
		Output:  map[string]interface{}{},
		Process: []string{},
	}
	flowContext := map[string]interface{}{}

	executor.executeSetVariableNodeDryRun(&result, flowContext, map[string]interface{}{
		"variables": map[string]interface{}{
			"execution_hosts": []interface{}{"118.196.22.79"},
		},
	})

	hosts, ok := flowContext["execution_hosts"].([]interface{})
	if !ok {
		t.Fatalf("execution_hosts type = %T, want []interface{}", flowContext["execution_hosts"])
	}
	if len(hosts) != 1 || hosts[0] != "118.196.22.79" {
		t.Fatalf("execution_hosts = %#v, want [118.196.22.79]", hosts)
	}
	if result.Status == "error" {
		t.Fatalf("status = error, message=%s", result.Message)
	}
}
