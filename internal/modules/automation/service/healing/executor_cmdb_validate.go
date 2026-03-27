package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

type cmdbValidationConfig struct {
	InputKey       string
	OutputKey      string
	FailOnNotFound bool
	SkipMissing    bool
	Hosts          []string
}

func (e *FlowExecutor) prepareCMDBValidation(config map[string]interface{}, flowContext model.JSON) cmdbValidationConfig {
	prepared := cmdbValidationConfig{
		InputKey:       "hosts",
		OutputKey:      "validated_hosts",
		FailOnNotFound: true,
		SkipMissing:    false,
	}
	if value, ok := config["input_key"].(string); ok && value != "" {
		prepared.InputKey = value
	}
	if value, ok := config["output_key"].(string); ok && value != "" {
		prepared.OutputKey = value
	}
	if value, ok := config["fail_on_not_found"].(bool); ok {
		prepared.FailOnNotFound = value
	}
	if value, ok := config["skip_missing"].(bool); ok {
		prepared.SkipMissing = value
	}
	prepared.Hosts = cmdbHostsFromContext(flowContext, prepared.InputKey)
	return prepared
}

func cmdbHostsFromContext(flowContext model.JSON, inputKey string) []string {
	if flowContext == nil {
		return nil
	}
	switch hostList := flowContext[inputKey].(type) {
	case []interface{}:
		hosts := make([]string, 0, len(hostList))
		for _, host := range hostList {
			if hostStr, ok := host.(string); ok {
				hosts = append(hosts, hostStr)
			}
		}
		return hosts
	case []string:
		return hostList
	default:
		return nil
	}
}

func (e *FlowExecutor) validateHostsWithCMDB(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, prepared cmdbValidationConfig) ([]map[string]interface{}, []map[string]interface{}, error) {
	var validatedHosts []map[string]interface{}
	var invalidHosts []map[string]interface{}

	for _, host := range prepared.Hosts {
		validatedHost, invalidHost, err := e.validateSingleCMDBHost(ctx, instance, node, prepared, host)
		if err != nil {
			return nil, append(invalidHosts, invalidHost), err
		}
		if validatedHost != nil {
			validatedHosts = append(validatedHosts, validatedHost)
		}
		if invalidHost != nil {
			invalidHosts = append(invalidHosts, invalidHost)
		}
	}
	return validatedHosts, invalidHosts, nil
}

func (e *FlowExecutor) validateSingleCMDBHost(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, prepared cmdbValidationConfig, host string) (map[string]interface{}, map[string]interface{}, error) {
	logger.Exec("NODE").Debug("验证主机: %s", host)
	cmdbItem, err := e.cmdbRepo.FindByNameOrIP(ctx, host)
	if err != nil {
		return e.handleMissingCMDBHost(ctx, instance, node, prepared, host)
	}
	if isInvalidCMDBStatus(cmdbItem.Status) {
		return e.handleInvalidCMDBStatus(ctx, instance, node, prepared, host, cmdbItem)
	}
	return buildValidatedCMDBHost(instance, host, cmdbItem), nil, nil
}

func (e *FlowExecutor) storeCMDBValidationResult(ctx context.Context, instance *model.FlowInstance, nodeID string, prepared cmdbValidationConfig, validatedHosts, invalidHosts []map[string]interface{}) error {
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	summary := cmdbValidationSummary(prepared.Hosts, validatedHosts, invalidHosts)
	instance.Context[prepared.OutputKey] = validatedHosts
	instance.Context["validation_summary"] = summary
	if len(invalidHosts) > 0 {
		instance.Context["invalid_hosts"] = invalidHosts
	}
	if err := e.persistInstance(ctx, instance, "持久化 CMDB 验证结果"); err != nil {
		return err
	}

	e.logNode(ctx, instance.ID, nodeID, model.NodeTypeCMDBValidator, model.LogLevelInfo, "CMDB 验证成功", map[string]interface{}{
		"input": map[string]interface{}{
			"hosts":     prepared.Hosts,
			"input_key": prepared.InputKey,
		},
		"process": cmdbValidationProcessLogs(prepared, validatedHosts, invalidHosts),
		"output": map[string]interface{}{
			prepared.OutputKey: validatedHosts,
			"invalid_hosts":    invalidHosts,
		},
	})

	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}
	if existing, ok := instance.NodeStates[nodeID].(map[string]interface{}); ok {
		existing["validated_hosts"] = validatedHosts
		existing["invalid_hosts"] = invalidHosts
		existing["validation_summary"] = summary
		instance.NodeStates[nodeID] = existing
		if err := e.persistNodeStates(ctx, instance, "持久化 CMDB 节点状态"); err != nil {
			return err
		}
	}
	return nil
}

func (e *FlowExecutor) handleMissingCMDBHost(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, prepared cmdbValidationConfig, host string) (map[string]interface{}, map[string]interface{}, error) {
	invalidHost := map[string]interface{}{"original_name": host, "valid": false, "reason": "not_found_in_cmdb"}
	logger.Exec("NODE").Warn("主机未在 CMDB 找到: %s", host)
	if !prepared.SkipMissing && prepared.FailOnNotFound {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{"host": host, "reason": "not_found_in_cmdb"})
		return nil, invalidHost, fmt.Errorf("主机 %s 未在 CMDB 找到", host)
	}
	return nil, invalidHost, nil
}

func isInvalidCMDBStatus(status string) bool {
	return status == "maintenance" || status == "offline"
}

func (e *FlowExecutor) handleInvalidCMDBStatus(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, prepared cmdbValidationConfig, host string, cmdbItem *model.CMDBItem) (map[string]interface{}, map[string]interface{}, error) {
	reason := invalidCMDBStatusReason(cmdbItem.Status)
	invalidHost := map[string]interface{}{
		"original_name":      host,
		"valid":              false,
		"reason":             reason,
		"status":             cmdbItem.Status,
		"maintenance_reason": cmdbItem.MaintenanceReason,
	}
	logger.Exec("NODE").Warn("主机状态异常: %s, status=%s", host, cmdbItem.Status)
	if !prepared.SkipMissing && prepared.FailOnNotFound {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{"host": host, "reason": reason, "status": cmdbItem.Status})
		return nil, invalidHost, fmt.Errorf("主机 %s 状态异常: %s", host, cmdbItem.Status)
	}
	return nil, invalidHost, nil
}

func invalidCMDBStatusReason(status string) string {
	if status == "offline" {
		return "offline_status"
	}
	return "maintenance_status"
}

func buildValidatedCMDBHost(instance *model.FlowInstance, host string, cmdbItem *model.CMDBItem) map[string]interface{} {
	ipAddress := cmdbItem.IPAddress
	if ipAddress == "" {
		ipAddress = host
	}
	logger.Exec("NODE").Info("[%s] 主机验证成功: %s -> IP=%s", shortID(instance), host, ipAddress)
	return map[string]interface{}{
		"original_name": host,
		"ip_address":    ipAddress,
		"name":          cmdbItem.Name,
		"hostname":      cmdbItem.Hostname,
		"status":        cmdbItem.Status,
		"environment":   cmdbItem.Environment,
		"os":            cmdbItem.OS,
		"os_version":    cmdbItem.OSVersion,
		"owner":         cmdbItem.Owner,
		"location":      cmdbItem.Location,
		"valid":         true,
		"cmdb_id":       cmdbItem.ID.String(),
	}
}

func cmdbValidationSummary(hosts []string, validatedHosts, invalidHosts []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"total":   len(hosts),
		"valid":   len(validatedHosts),
		"invalid": len(invalidHosts),
	}
}

func cmdbValidationProcessLogs(prepared cmdbValidationConfig, validatedHosts, invalidHosts []map[string]interface{}) []string {
	processLogs := []string{
		fmt.Sprintf("读取配置 input_key: %s, output_key: %s", prepared.InputKey, prepared.OutputKey),
		fmt.Sprintf("从上下文获取 %d 个主机", len(prepared.Hosts)),
		"开始查询 CMDB 数据库",
	}
	for _, host := range validatedHosts {
		processLogs = append(processLogs, fmt.Sprintf("主机 %v: 验证通过", host["original_name"]))
	}
	for _, host := range invalidHosts {
		processLogs = append(processLogs, fmt.Sprintf("主机 %v: %v", host["original_name"], host["reason"]))
	}
	processLogs = append(processLogs, fmt.Sprintf("验证完成: %d 通过, %d 失败", len(validatedHosts), len(invalidHosts)))
	processLogs = append(processLogs, fmt.Sprintf("写入上下文 %s", prepared.OutputKey))
	return processLogs
}
