package healing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/model"
)

type hostExtractionConfig struct {
	SourceField  string
	ExtractMode  string
	OutputKey    string
	SplitBy      string
	RegexPattern string
	RegexGroup   int
	JSONPath     string
}

func (e *FlowExecutor) prepareHostExtraction(config map[string]interface{}) hostExtractionConfig {
	prepared := hostExtractionConfig{
		SourceField: "affected_ci",
		ExtractMode: "direct",
		OutputKey:   "hosts",
		SplitBy:     ",",
	}
	if value, ok := config["source_field"].(string); ok && value != "" {
		prepared.SourceField = value
	}
	if value, ok := config["extract_mode"].(string); ok && value != "" {
		prepared.ExtractMode = value
	}
	if value, ok := config["output_key"].(string); ok && value != "" {
		prepared.OutputKey = value
	}
	if value, ok := config["split_by"].(string); ok && value != "" {
		prepared.SplitBy = value
	}
	if value, ok := config["regex_pattern"].(string); ok {
		prepared.RegexPattern = value
	}
	if value, ok := config["regex_group"].(float64); ok {
		prepared.RegexGroup = int(value)
	}
	if value, ok := config["json_path"].(string); ok {
		prepared.JSONPath = value
	}
	return prepared
}

func (e *FlowExecutor) hostSourceValue(instance *model.FlowInstance, sourceField string) string {
	if instance.Context == nil {
		return ""
	}
	incident, ok := instance.Context["incident"].(map[string]interface{})
	if !ok {
		return ""
	}

	current := nestedValue(incident, sourceField)
	if current == nil {
		return ""
	}
	switch value := current.(type) {
	case string:
		return value
	default:
		if payload, err := json.Marshal(value); err == nil {
			return string(payload)
		}
		return ""
	}
}

func nestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, part := range parts {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = currentMap[part]
		if current == nil {
			return nil
		}
	}
	return current
}

func (e *FlowExecutor) extractHostsByMode(config hostExtractionConfig, sourceValue string) ([]string, error) {
	switch config.ExtractMode {
	case "direct":
		return []string{strings.TrimSpace(sourceValue)}, nil
	case "split":
		return splitHosts(sourceValue, config.SplitBy), nil
	case "regex":
		return regexHosts(sourceValue, config.RegexPattern, config.RegexGroup)
	case "json_path":
		return jsonPathHosts(sourceValue, config.JSONPath)
	default:
		return nil, fmt.Errorf("不支持的提取模式: %s", config.ExtractMode)
	}
}

func splitHosts(sourceValue, splitBy string) []string {
	parts := strings.Split(sourceValue, splitBy)
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			hosts = append(hosts, part)
		}
	}
	return hosts
}

func regexHosts(sourceValue, pattern string, group int) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("正则模式下必须指定 regex_pattern")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("正则表达式编译失败: %v", err)
	}

	matches := re.FindAllStringSubmatch(sourceValue, -1)
	var hosts []string
	for _, match := range matches {
		if group < len(match) {
			host := strings.TrimSpace(match[group])
			if host != "" {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts, nil
}

func jsonPathHosts(sourceValue, jsonPath string) ([]string, error) {
	if jsonPath == "" {
		return nil, fmt.Errorf("json_path 模式下必须指定 json_path")
	}

	var jsonData interface{}
	if err := json.Unmarshal([]byte(sourceValue), &jsonData); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	switch typed := jsonData.(type) {
	case []interface{}:
		var hosts []string
		for _, item := range typed {
			switch value := item.(type) {
			case string:
				hosts = append(hosts, value)
			case map[string]interface{}:
				if name, ok := value["name"].(string); ok {
					hosts = append(hosts, name)
				} else if host, ok := value["host"].(string); ok {
					hosts = append(hosts, host)
				}
			}
		}
		return hosts, nil
	case string:
		return []string{typed}, nil
	default:
		return nil, nil
	}
}

func (e *FlowExecutor) storeExtractedHosts(ctx context.Context, instance *model.FlowInstance, nodeID string, prepared hostExtractionConfig, sourceValue string, hosts []string) error {
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	instance.Context[prepared.OutputKey] = hosts
	if err := e.persistInstance(ctx, instance, "持久化主机提取结果"); err != nil {
		return err
	}

	e.logNode(ctx, instance.ID, nodeID, model.NodeTypeHostExtractor, model.LogLevelInfo, "主机提取成功", map[string]interface{}{
		"input": map[string]interface{}{
			"context":      instance.Context,
			"source_field": prepared.SourceField,
		},
		"process": []string{
			fmt.Sprintf("读取配置 source_field: %s, extract_mode: %s", prepared.SourceField, prepared.ExtractMode),
			fmt.Sprintf("从工单数据提取源值: %s", sourceValue),
			fmt.Sprintf("使用 %s 模式提取主机", prepared.ExtractMode),
			fmt.Sprintf("成功提取 %d 个主机: %v", len(hosts), hosts),
			fmt.Sprintf("写入上下文 %s", prepared.OutputKey),
		},
		"output": map[string]interface{}{
			prepared.OutputKey: hosts,
		},
	})

	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	existing := map[string]interface{}{
		"extracted_hosts": hosts,
		"source_field":    prepared.SourceField,
		"extract_mode":    prepared.ExtractMode,
		"host_count":      len(hosts),
	}
	if current, ok := instance.NodeStates[nodeID].(map[string]interface{}); ok {
		for key, value := range existing {
			current[key] = value
		}
		instance.NodeStates[nodeID] = current
	} else {
		instance.NodeStates[nodeID] = existing
	}
	return e.persistNodeStates(ctx, instance, "持久化主机提取节点状态")
}
