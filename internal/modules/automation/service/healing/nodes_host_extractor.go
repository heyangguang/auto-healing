package healing

import (
	"context"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/modules/automation/model"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
)

// HostExtractorConfig 主机提取配置
type HostExtractorConfig struct {
	Source  string `json:"source"`  // 提取来源: title, description, affected_ci, raw_data.xxx
	Pattern string `json:"pattern"` // 正则表达式模式
	Field   string `json:"field"`   // 如果来源是 raw_data，指定具体字段
}

// ExecuteHostExtractor 执行主机提取
func (e *NodeExecutors) ExecuteHostExtractor(_ context.Context, instance *model.FlowInstance, config map[string]interface{}) ([]string, error) {
	cfg := parseHostExtractorConfig(config)
	incident := e.getIncidentFromContext(instance)
	if incident == nil {
		return nil, nil
	}
	return extractHostsFromIncident(incident, cfg)
}

func parseHostExtractorConfig(config map[string]interface{}) *HostExtractorConfig {
	cfg := &HostExtractorConfig{
		Source:  "description",
		Pattern: `\b(?:\d{1,3}\.){3}\d{1,3}\b|[a-zA-Z][a-zA-Z0-9-]*(?:\.[a-zA-Z0-9-]+)+`,
	}
	if source, ok := config["source"].(string); ok {
		cfg.Source = source
	}
	if pattern, ok := config["pattern"].(string); ok {
		cfg.Pattern = pattern
	}
	if field, ok := config["field"].(string); ok {
		cfg.Field = field
	}
	return cfg
}

func extractHostsFromIncident(incident *platformmodel.Incident, cfg *HostExtractorConfig) ([]string, error) {
	re, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, err
	}
	return uniqueHosts(re.FindAllString(hostExtractorSourceText(incident, cfg), -1)), nil
}

func hostExtractorSourceText(incident *platformmodel.Incident, cfg *HostExtractorConfig) string {
	switch cfg.Source {
	case "title":
		return incident.Title
	case "description":
		return incident.Description
	case "affected_ci":
		return incident.AffectedCI
	case "affected_service":
		return incident.AffectedService
	case "raw_data":
		if incident.RawData != nil && cfg.Field != "" {
			if val, ok := incident.RawData[cfg.Field]; ok {
				return toString(val)
			}
		}
	}
	return incident.Description
}

func uniqueHosts(matches []string) []string {
	hostSet := make(map[string]bool)
	var hosts []string
	for _, match := range matches {
		host := strings.TrimSpace(match)
		if host == "" || hostSet[host] {
			continue
		}
		hostSet[host] = true
		hosts = append(hosts, host)
	}
	return hosts
}
