package healing

import (
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func (e *FlowExecutor) resolveExecutionTargetHosts(instance *model.FlowInstance, config map[string]interface{}) (string, string) {
	hostsKey := "validated_hosts"
	if value, ok := config["hosts_key"].(string); ok && value != "" {
		hostsKey = value
	}

	hostIPs := executionHostIPs(instance.Context, hostsKey)
	if len(hostIPs) == 0 {
		return hostsKey, ""
	}
	targetHosts := strings.Join(hostIPs, ",")
	logger.Exec("ANSIBLE").Info("[%s] 使用 context 中的主机: %s", shortID(instance), targetHosts)
	return hostsKey, targetHosts
}

func executionHostIPs(flowContext model.JSON, hostsKey string) []string {
	if flowContext == nil {
		return nil
	}
	hostIPs := collectExecutionHosts(flowContext[hostsKey])
	if len(hostIPs) > 0 {
		return hostIPs
	}
	return collectExecutionHosts(flowContext["hosts"])
}

func collectExecutionHosts(raw interface{}) []string {
	var hostIPs []string
	switch list := raw.(type) {
	case []string:
		for _, host := range list {
			if host != "" {
				hostIPs = append(hostIPs, host)
			}
		}
	case []interface{}:
		for _, host := range list {
			switch typed := host.(type) {
			case map[string]interface{}:
				if ip, ok := typed["ip_address"].(string); ok && ip != "" {
					hostIPs = append(hostIPs, ip)
				} else if name, ok := typed["original_name"].(string); ok && name != "" {
					hostIPs = append(hostIPs, name)
				}
			case string:
				if typed != "" {
					hostIPs = append(hostIPs, typed)
				}
			}
		}
	case []map[string]interface{}:
		for _, host := range list {
			if ip, ok := host["ip_address"].(string); ok && ip != "" {
				hostIPs = append(hostIPs, ip)
			} else if name, ok := host["original_name"].(string); ok && name != "" {
				hostIPs = append(hostIPs, name)
			}
		}
	}
	return hostIPs
}
