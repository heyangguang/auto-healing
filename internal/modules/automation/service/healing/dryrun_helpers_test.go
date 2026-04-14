package healing

import "testing"

func TestDryRunGetHostsFromContextSupportsValidatedHosts(t *testing.T) {
	executor := &DryRunExecutor{}
	flowContext := map[string]interface{}{
		"validated_hosts": []interface{}{
			map[string]interface{}{
				"ip_address":    "192.168.31.100",
				"original_name": "real-host-100",
			},
		},
	}

	hosts := executor.getHostsFromContext(flowContext, "validated_hosts")
	if len(hosts) != 1 || hosts[0] != "192.168.31.100" {
		t.Fatalf("hosts = %#v", hosts)
	}
}
