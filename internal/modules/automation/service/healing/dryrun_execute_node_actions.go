package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/google/uuid"
)

func (e *DryRunExecutor) executeExecutionNodeDryRun(ctx context.Context, result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) {
	taskTemplateID, hasTaskID := config["task_template_id"].(string)
	result.Process = append(result.Process, fmt.Sprintf("读取配置 task_template_id: %s", taskTemplateID))
	if !hasTaskID || taskTemplateID == "" {
		result.Status = "error"
		result.Message = "执行节点配置错误: 未配置 task_template_id（任务模板）"
		result.Process = append(result.Process, "错误: 未配置任务模板ID")
		return
	}

	taskUUID, err := uuid.Parse(taskTemplateID)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("执行节点配置错误: task_template_id 格式无效「%s」", taskTemplateID)
		result.Process = append(result.Process, "错误: 任务模板ID格式无效")
		return
	}
	task, err := e.taskRepo.GetTaskByID(ctx, taskUUID)
	if err != nil || task == nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("执行节点配置错误: 任务模板不存在「%s」", taskTemplateID)
		result.Process = append(result.Process, "错误: 任务模板不存在")
		return
	}

	result.Process = append(result.Process, fmt.Sprintf("任务模板验证通过: %s", task.Name))
	e.appendDryRunTaskTemplateInfo(result, task)
	finalVars := e.buildDryRunExecutionVariables(result, flowContext, config, task)
	hosts := e.validateDryRunExecutionHosts(result, flowContext, config)
	if len(hosts) == 0 {
		return
	}

	result.Message = fmt.Sprintf("将执行任务「%s」，目标主机: %v", task.Name, hosts)
	result.Output["task_template_id"] = taskTemplateID
	result.Output["task_template"] = task.Name
	result.Output["target_hosts"] = hosts
	result.Output["final_vars"] = finalVars
	result.Output["output_handle"] = "success"
	result.Process = append(result.Process, "--- 执行结果 ---")
	result.Process = append(result.Process, fmt.Sprintf("模拟执行完成，将在 %d 台主机上执行任务「%s」", len(hosts), task.Name))
}

func (e *DryRunExecutor) appendDryRunTaskTemplateInfo(result *DryRunNodeResult, task *model.ExecutionTask) {
	result.Process = append(result.Process, "--- 任务模板配置 ---")
	result.Process = append(result.Process, fmt.Sprintf("模板名称: %s", task.Name))
	if task.Description != "" {
		result.Process = append(result.Process, fmt.Sprintf("模板描述: %s", task.Description))
	}
	if task.PlaybookID != uuid.Nil {
		result.Process = append(result.Process, fmt.Sprintf("Playbook ID: %s", task.PlaybookID.String()))
	}
}

func (e *DryRunExecutor) buildDryRunExecutionVariables(result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}, task *model.ExecutionTask) map[string]interface{} {
	result.Process = append(result.Process, "--- 变量配置 ---")
	finalVars := make(map[string]interface{})
	if task.ExtraVars != nil {
		for key, value := range task.ExtraVars {
			finalVars[key] = value
		}
	}
	if extraVars, ok := config["extra_vars"].(map[string]interface{}); ok {
		for key, value := range extraVars {
			finalVars[key] = value
		}
	}

	variableMappings := make(map[string]string)
	if mappings, ok := config["variable_mappings"].(map[string]interface{}); ok {
		for key, value := range mappings {
			if expression, ok := value.(string); ok {
				variableMappings[key] = expression
			}
		}
	}

	evaluator := NewExpressionEvaluator()
	for varName, expression := range variableMappings {
		result.Process = append(result.Process, fmt.Sprintf("计算表达式 %s = %s", varName, expression))
		exprResult, err := evaluator.Evaluate(expression, flowContext)
		if err != nil {
			result.Process = append(result.Process, fmt.Sprintf("  → 计算失败: %v", err))
			continue
		}
		finalVars[varName] = exprResult
		result.Process = append(result.Process, fmt.Sprintf("  → 结果: %v", exprResult))
	}

	if len(finalVars) == 0 {
		result.Process = append(result.Process, "无额外变量")
		return finalVars
	}

	result.Process = append(result.Process, "最终变量值:")
	for key, value := range finalVars {
		result.Process = append(result.Process, fmt.Sprintf("  %s = %v", key, value))
	}
	return finalVars
}

func (e *DryRunExecutor) validateDryRunExecutionHosts(result *DryRunNodeResult, flowContext map[string]interface{}, config map[string]interface{}) []string {
	result.Process = append(result.Process, "--- 目标主机 ---")
	hostsKey := "validated_hosts"
	if value, ok := config["hosts_key"].(string); ok && value != "" {
		hostsKey = value
	}
	hosts := e.getHostsFromContext(flowContext, hostsKey)
	result.Process = append(result.Process, fmt.Sprintf("主机来源: 上下文变量 %s", hostsKey))
	result.Process = append(result.Process, fmt.Sprintf("获取到 %d 个目标主机", len(hosts)))
	if len(hosts) > 0 {
		result.Process = append(result.Process, fmt.Sprintf("主机列表: %v", hosts))
		return hosts
	}

	result.Status = "error"
	result.Message = fmt.Sprintf("执行节点失败: 目标主机列表为空（来源: %s）", hostsKey)
	result.Process = append(result.Process, "错误: 目标主机列表为空")
	return nil
}

func (e *DryRunExecutor) executeNotificationNodeDryRun(ctx context.Context, result *DryRunNodeResult, config map[string]interface{}) {
	templateID, hasTemplateID := config["template_id"].(string)
	result.Process = append(result.Process, fmt.Sprintf("读取配置 template_id: %s", templateID))
	if !hasTemplateID || templateID == "" {
		result.Status = "error"
		result.Message = "通知节点配置错误: 未配置 template_id（通知模板）"
		result.Process = append(result.Process, "错误: 未配置通知模板ID")
		return
	}

	templateUUID, err := uuid.Parse(templateID)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("通知节点配置错误: 通知模板ID格式无效「%s」", templateID)
		result.Process = append(result.Process, "错误: 通知模板ID格式无效")
		return
	}
	tpl, err := e.notificationRepo.GetTemplateByID(ctx, templateUUID)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("通知节点配置错误: 通知模板不存在「%s」", templateID)
		result.Process = append(result.Process, "错误: 通知模板不存在")
		return
	}

	channels, hasChannels := config["channel_ids"].([]interface{})
	if !hasChannels || len(channels) == 0 {
		result.Status = "error"
		result.Message = "通知节点配置错误: 未配置 channel_ids（通知渠道）"
		result.Process = append(result.Process, "错误: 未配置通知渠道")
		return
	}

	result.Process = append(result.Process, fmt.Sprintf("通知模板验证通过: %s", tpl.Name))
	result.Process = append(result.Process, fmt.Sprintf("通知渠道验证通过: %d 个渠道", len(channels)))
	result.Message = fmt.Sprintf("将发送通知「%s」到 %d 个渠道", tpl.Name, len(channels))
	result.Process = append(result.Process, "模拟发送完成")
}
