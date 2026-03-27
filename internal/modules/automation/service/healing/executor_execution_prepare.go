package healing

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/modules/automation/service/execution"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

type preparedExecutionNode struct {
	TaskTemplateID   string
	TaskUUID         uuid.UUID
	HostsKey         string
	TargetHosts      string
	MergedExtraVars  map[string]any
	Task             *model.ExecutionTask
	TaskTemplateInfo map[string]interface{}
}

type executionOutcome struct {
	Status  string
	Message string
	Run     map[string]interface{}
}

func (e *FlowExecutor) prepareExecutionNode(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) (*preparedExecutionNode, error) {
	if instance.NodeStates == nil {
		instance.NodeStates = make(model.JSON)
	}

	taskTemplateID, err := e.executionTaskTemplateID(node.Config)
	if err != nil {
		return nil, e.failExecutionPreparation(ctx, instance, node, err, map[string]interface{}{"error": err.Error()})
	}

	taskUUID, err := uuid.Parse(taskTemplateID)
	if err != nil {
		return nil, e.failExecutionPreparation(ctx, instance, node, fmt.Errorf("无效的 task_template_id: %v", err), map[string]interface{}{
			"task_template_id": taskTemplateID,
			"error":            err.Error(),
		})
	}

	hostsKey, targetHosts := e.resolveExecutionTargetHosts(instance, node.Config)
	mergedExtraVars := e.resolveExecutionExtraVars(instance, node.Config)
	task, taskTemplateInfo := e.executionTaskTemplateInfo(ctx, taskUUID, taskTemplateID)

	return &preparedExecutionNode{
		TaskTemplateID:   taskTemplateID,
		TaskUUID:         taskUUID,
		HostsKey:         hostsKey,
		TargetHosts:      targetHosts,
		MergedExtraVars:  mergedExtraVars,
		Task:             task,
		TaskTemplateInfo: taskTemplateInfo,
	}, nil
}

func (e *FlowExecutor) executionTaskTemplateID(config map[string]interface{}) (string, error) {
	taskTemplateID, _ := config["task_template_id"].(string)
	if taskTemplateID == "" {
		return "", fmt.Errorf("必须指定 task_template_id")
	}
	return taskTemplateID, nil
}

func (e *FlowExecutor) failExecutionPreparation(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, err error, details map[string]interface{}) error {
	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "配置错误", details)
	instance.NodeStates[node.ID] = map[string]interface{}{
		"status":  "failed",
		"message": err.Error(),
	}
	if updateErr := e.persistNodeStates(ctx, instance, "持久化执行节点失败状态"); updateErr != nil {
		return fmt.Errorf("执行节点准备失败: %v; %w", err, updateErr)
	}
	return err
}

func (e *FlowExecutor) resolveExecutionExtraVars(instance *model.FlowInstance, config map[string]interface{}) map[string]any {
	mergedExtraVars := make(map[string]any)
	if nodeExtraVars, ok := config["extra_vars"].(map[string]interface{}); ok {
		for key, value := range nodeExtraVars {
			mergedExtraVars[key] = value
		}
	}

	variableMappings, ok := config["variable_mappings"].(map[string]interface{})
	if !ok || len(variableMappings) == 0 {
		return mergedExtraVars
	}

	evaluator := NewExpressionEvaluator()
	logger.Exec("ANSIBLE").Info("[%s] 处理 variable_mappings: %d 个映射", shortID(instance), len(variableMappings))
	for varName, exprRaw := range variableMappings {
		expression, ok := exprRaw.(string)
		if !ok || expression == "" {
			logger.Exec("ANSIBLE").Warn("[%s] variable_mappings 中的 %s 值无效", shortID(instance), varName)
			continue
		}

		result, err := evaluator.Evaluate(expression, instance.Context)
		if err != nil {
			logger.Exec("ANSIBLE").Warn("[%s] 计算 %s 失败: %v (表达式: %s)", shortID(instance), varName, err, expression)
			continue
		}
		mergedExtraVars[varName] = result
		logger.Exec("ANSIBLE").Debug("[%s] 变量映射: %s = %v (来自: %s)", shortID(instance), varName, result, expression)
	}
	return mergedExtraVars
}

func (e *FlowExecutor) executionTaskTemplateInfo(ctx context.Context, taskUUID uuid.UUID, taskTemplateID string) (*model.ExecutionTask, map[string]interface{}) {
	taskTemplateInfo := map[string]interface{}{"task_template_id": taskTemplateID}
	task, err := e.executionRepo.GetTaskByID(ctx, taskUUID)
	if err != nil || task == nil {
		return nil, taskTemplateInfo
	}

	taskTemplateInfo["task_template_name"] = task.Name
	taskTemplateInfo["task_description"] = task.Description
	if task.PlaybookID != uuid.Nil {
		taskTemplateInfo["playbook_id"] = task.PlaybookID.String()
	}
	if task.ExtraVars != nil {
		taskTemplateInfo["template_vars"] = task.ExtraVars
	}
	return task, taskTemplateInfo
}

func (e *FlowExecutor) markExecutionNodeRunning(ctx context.Context, instance *model.FlowInstance, nodeID string, prepared *preparedExecutionNode, startedAt time.Time) error {
	instance.NodeStates[nodeID] = map[string]interface{}{
		"status":       "running",
		"task_id":      prepared.TaskTemplateID,
		"target_hosts": prepared.TargetHosts,
		"started_at":   startedAt.Format(time.RFC3339),
		"message":      "正在执行任务模板...",
	}
	return e.persistNodeStates(ctx, instance, "持久化执行节点运行状态")
}

func (e *FlowExecutor) logExecutionNodeStart(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, prepared *preparedExecutionNode) {
	processLogs := []string{
		"--- 任务模板配置 ---",
		fmt.Sprintf("任务模板 ID: %s", prepared.TaskTemplateID),
	}
	if prepared.Task != nil {
		processLogs = append(processLogs, fmt.Sprintf("任务模板名称: %s", prepared.Task.Name))
		if prepared.Task.Description != "" {
			processLogs = append(processLogs, fmt.Sprintf("任务描述: %s", prepared.Task.Description))
		}
	}
	processLogs = append(processLogs, "--- 目标主机 ---")
	processLogs = append(processLogs, fmt.Sprintf("主机来源: 上下文变量 %s", prepared.HostsKey))
	processLogs = append(processLogs, fmt.Sprintf("主机列表: %s", prepared.TargetHosts))
	processLogs = append(processLogs, executionVariableLogs(prepared.MergedExtraVars)...)
	processLogs = append(processLogs, "--- 开始执行 ---", "调用执行服务...")

	e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelInfo, "开始执行", map[string]interface{}{
		"input": map[string]interface{}{
			"context":            instance.Context,
			"task_template_info": prepared.TaskTemplateInfo,
			"target_hosts":       prepared.TargetHosts,
			"merged_vars":        prepared.MergedExtraVars,
			"hosts_source":       prepared.HostsKey,
		},
		"process": processLogs,
	})
}

func executionVariableLogs(mergedExtraVars map[string]any) []string {
	processLogs := []string{"--- 变量配置 ---"}
	if len(mergedExtraVars) == 0 {
		return append(processLogs, "无额外变量")
	}
	processLogs = append(processLogs, fmt.Sprintf("共 %d 个变量:", len(mergedExtraVars)))
	for key, value := range mergedExtraVars {
		processLogs = append(processLogs, fmt.Sprintf("  %s = %v", key, value))
	}
	return processLogs
}

func (e *FlowExecutor) runPreparedExecution(ctx context.Context, instance *model.FlowInstance, prepared *preparedExecutionNode) executionOutcome {
	run, execErr := e.executionSvc.ExecuteTask(ctx, prepared.TaskUUID, &execution.ExecuteOptions{
		TriggeredBy:      "healing",
		TargetHosts:      prepared.TargetHosts,
		ExtraVars:        prepared.MergedExtraVars,
		SkipNotification: true,
	})
	if execErr != nil {
		return executionOutcome{Status: "failed", Message: fmt.Sprintf("执行失败: %v", execErr)}
	}
	if run == nil {
		return executionOutcome{Status: "completed", Message: "执行成功"}
	}

	completedRun, waitErr := e.waitForRunCompletion(ctx, instance.ID, run.ID, 30*time.Minute)
	if waitErr != nil {
		return executionOutcome{Status: "failed", Message: fmt.Sprintf("等待执行完成失败: %v", waitErr)}
	}
	if completedRun == nil {
		return executionOutcome{Status: "completed", Message: "执行成功"}
	}
	return summarizeExecutionOutcome(completedRun)
}

func summarizeExecutionOutcome(run *model.ExecutionRun) executionOutcome {
	outcome := executionOutcome{
		Status:  "completed",
		Message: "执行成功",
		Run: map[string]interface{}{
			"run_id":    run.ID.String(),
			"status":    run.Status,
			"exit_code": run.ExitCode,
			"stats":     run.Stats,
		},
	}

	switch run.Status {
	case "failed":
		outcome.Status = "failed"
		if run.ExitCode != nil {
			outcome.Message = fmt.Sprintf("任务执行失败 (退出码: %d)", *run.ExitCode)
		} else {
			outcome.Message = "任务执行失败"
		}
	case "partial":
		outcome.Status = "partial"
		outcome.Message = "任务部分成功（部分主机执行失败或不可达）"
	}
	return outcome
}

func (e *FlowExecutor) recordExecutionOutcome(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode, prepared *preparedExecutionNode, outcome executionOutcome, startedAt time.Time) error {
	executionResult := map[string]interface{}{
		"status":       outcome.Status,
		"message":      outcome.Message,
		"started_at":   startedAt.Format(time.RFC3339),
		"finished_at":  time.Now().Format(time.RFC3339),
		"duration_ms":  time.Since(startedAt).Milliseconds(),
		"task_id":      prepared.TaskTemplateID,
		"target_hosts": prepared.TargetHosts,
	}
	if outcome.Run != nil {
		executionResult["run"] = outcome.Run
	}

	instance.NodeStates[node.ID] = executionResult
	if err := e.persistNodeStates(ctx, instance, "持久化执行节点结果"); err != nil {
		return err
	}
	if instance.Context == nil {
		instance.Context = make(model.JSON)
	}
	instance.Context["execution_result"] = executionResult
	if err := e.persistInstance(ctx, instance, "持久化执行上下文"); err != nil {
		return err
	}
	e.logNode(ctx, instance.ID, node.ID, node.Type, executionLogLevel(outcome.Status), outcome.Message, executionResult)
	return nil
}

func executionLogLevel(status string) string {
	switch status {
	case "failed":
		return model.LogLevelError
	case "partial":
		return model.LogLevelWarn
	default:
		return model.LogLevelInfo
	}
}
