package healing

import (
	"context"
	"fmt"
)

func (e *DryRunExecutor) executeHostExtractorNodeDryRun(result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) {
	sourceField, _ := config["source_field"].(string)
	result.Process = append(result.Process, fmt.Sprintf("读取配置 source_field: %s", sourceField))
	if sourceField == "" {
		result.Status = "error"
		result.Message = "主机提取失败: 未配置 source_field（数据源字段）"
		result.Process = append(result.Process, "错误: source_field 未配置")
		return
	}

	hosts := e.extractHosts(flowContext, config)
	result.Process = append(result.Process, fmt.Sprintf("从字段 %s 提取主机", sourceField))
	if len(hosts) == 0 {
		result.Status = "error"
		result.Message = fmt.Sprintf("主机提取失败: 从字段「%s」提取的主机列表为空", sourceField)
		result.Process = append(result.Process, "错误: 提取的主机列表为空")
		return
	}

	result.Process = append(result.Process, fmt.Sprintf("成功提取 %d 个主机: %v", len(hosts), hosts))
	result.Message = fmt.Sprintf("提取主机: %v", hosts)
	result.Output["hosts"] = hosts

	outputKey := "hosts"
	if value, ok := config["output_key"].(string); ok && value != "" {
		outputKey = value
	}
	flowContext[outputKey] = hosts
	result.Process = append(result.Process, fmt.Sprintf("写入上下文 %s", outputKey))
}

func (e *DryRunExecutor) executeCMDBValidatorNodeDryRun(ctx context.Context, result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) {
	inputKey := "hosts"
	if value, ok := config["input_key"].(string); ok && value != "" {
		inputKey = value
	}
	outputKey := "validated_hosts"
	if value, ok := config["output_key"].(string); ok && value != "" {
		outputKey = value
	}
	result.Process = append(result.Process, fmt.Sprintf("读取配置 input_key: %s, output_key: %s", inputKey, outputKey))

	hosts := e.getHostsFromContext(flowContext, inputKey)
	result.Process = append(result.Process, fmt.Sprintf("从上下文 %s 获取 %d 个主机", inputKey, len(hosts)))
	if len(hosts) == 0 {
		result.Status = "error"
		result.Message = fmt.Sprintf("CMDB 验证失败: 输入主机列表为空（来源: %s）", inputKey)
		result.Process = append(result.Process, "错误: 输入主机列表为空")
		return
	}

	validatedHosts, invalidHosts := e.validateDryRunHostsWithCMDB(ctx, result, hosts)
	if len(validatedHosts) == 0 {
		result.Status = "error"
		result.Message = fmt.Sprintf("CMDB 验证失败: 所有主机验证失败 %v", invalidHosts)
		result.Process = append(result.Process, "错误: 所有主机验证失败")
		return
	}

	result.Message = fmt.Sprintf("CMDB 验证通过: %d/%d 台主机", len(validatedHosts), len(hosts))
	result.Output["validated_hosts"] = validatedHosts
	result.Output["invalid_hosts"] = invalidHosts
	result.Process = append(result.Process, fmt.Sprintf("验证完成: %d 通过, %d 失败", len(validatedHosts), len(invalidHosts)))
	flowContext[outputKey] = validatedHosts
	result.Process = append(result.Process, fmt.Sprintf("写入上下文 %s", outputKey))
}

func (e *DryRunExecutor) validateDryRunHostsWithCMDB(ctx context.Context, result *DryRunNodeResult, hosts []string) ([]map[string]interface{}, []string) {
	result.Process = append(result.Process, "开始查询 CMDB 数据库")
	validatedHosts := make([]map[string]interface{}, 0, len(hosts))
	invalidHosts := make([]string, 0)

	for _, host := range hosts {
		cmdbItem, err := e.cmdbRepo.FindByNameOrIP(ctx, host)
		if err != nil {
			invalidHosts = append(invalidHosts, host)
			result.Process = append(result.Process, fmt.Sprintf("主机 %s: 未在 CMDB 找到", host))
			continue
		}
		if cmdbItem.Status == "maintenance" || cmdbItem.Status == "offline" {
			invalidHosts = append(invalidHosts, host)
			result.Process = append(result.Process, fmt.Sprintf("主机 %s: 状态为 %s，跳过", host, cmdbItem.Status))
			continue
		}

		result.Process = append(result.Process, fmt.Sprintf("主机 %s: 验证通过 (IP: %s, 状态: %s)", host, cmdbItem.IPAddress, cmdbItem.Status))
		validatedHosts = append(validatedHosts, map[string]interface{}{
			"original_name": host,
			"cmdb_name":     cmdbItem.Name,
			"ip":            cmdbItem.IPAddress,
			"status":        cmdbItem.Status,
		})
	}
	return validatedHosts, invalidHosts
}
