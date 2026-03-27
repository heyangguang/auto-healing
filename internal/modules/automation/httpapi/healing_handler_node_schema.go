package httpapi

func healingNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"initial_context": healingInitialContextSchema(),
		"nodes":           healingNodeDefinitions(),
	}
}

func healingInitialContextSchema() map[string]interface{} {
	return map[string]interface{}{
		"incident": map[string]interface{}{
			"type":        "object",
			"description": "触发流程的工单数据",
			"properties": map[string]interface{}{
				"id":               map[string]string{"type": "string", "description": "工单ID"},
				"title":            map[string]string{"type": "string", "description": "工单标题"},
				"description":      map[string]string{"type": "string", "description": "工单描述"},
				"severity":         map[string]string{"type": "string", "description": "严重级别"},
				"priority":         map[string]string{"type": "string", "description": "优先级"},
				"status":           map[string]string{"type": "string", "description": "状态"},
				"category":         map[string]string{"type": "string", "description": "分类"},
				"affected_ci":      map[string]string{"type": "string", "description": "影响的CI（多个用逗号分隔）"},
				"affected_service": map[string]string{"type": "string", "description": "影响的服务"},
				"assignee":         map[string]string{"type": "string", "description": "处理人"},
				"reporter":         map[string]string{"type": "string", "description": "报告人"},
				"raw_data":         map[string]string{"type": "object", "description": "原始数据（来自第三方系统）"},
			},
		},
	}
}

func healingNodeDefinitions() map[string]interface{} {
	return map[string]interface{}{
		"start":          startNodeSchema(),
		"end":            endNodeSchema(),
		"host_extractor": hostExtractorNodeSchema(),
		"cmdb_validator": cmdbValidatorNodeSchema(),
		"approval":       approvalNodeSchema(),
		"execution":      executionNodeSchema(),
		"notification":   notificationNodeSchema(),
		"condition":      conditionNodeSchema(),
		"set_variable":   setVariableNodeSchema(),
	}
}

func startNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "开始",
		"description": "流程起始节点",
		"config":      map[string]interface{}{},
		"ports": map[string]interface{}{
			"in":  0,
			"out": 1,
			"out_ports": []map[string]string{
				{"id": "default", "name": "默认"},
			},
		},
		"inputs": []interface{}{},
		"outputs": []map[string]string{
			{"key": "incident", "type": "object", "description": "工单对象"},
		},
	}
}

func endNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "结束",
		"description": "流程结束节点",
		"config":      map[string]interface{}{},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 0,
		},
		"inputs":  []interface{}{},
		"outputs": []interface{}{},
	}
}

func hostExtractorNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "主机提取器",
		"description": "从工单数据中提取主机列表",
		"config": map[string]interface{}{
			"source_field": map[string]string{"type": "string", "required": "true", "description": "数据来源字段，如 incident.affected_ci 或 incident.raw_data.cmdb_ci"},
			"extract_mode": map[string]string{"type": "string", "default": "split", "description": "提取模式：split(分割) 或 regex(正则)"},
			"split_by":     map[string]string{"type": "string", "default": ",", "description": "分割符（extract_mode=split时使用）"},
			"output_key":   map[string]string{"type": "string", "default": "hosts", "description": "输出变量名"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 1,
			"out_ports": []map[string]string{
				{"id": "default", "name": "默认"},
			},
		},
		"inputs": []map[string]string{
			{"key": "incident", "type": "object", "description": "工单对象"},
		},
		"outputs": []map[string]string{
			{"key": "hosts", "type": "array[string]", "description": "提取的主机列表"},
		},
	}
}

func cmdbValidatorNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "CMDB验证器",
		"description": "验证主机是否在CMDB中存在，并获取主机详细信息",
		"config": map[string]interface{}{
			"input_key":  map[string]string{"type": "string", "default": "hosts", "description": "输入变量名"},
			"output_key": map[string]string{"type": "string", "default": "validated_hosts", "description": "输出变量名"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 1,
			"out_ports": []map[string]string{
				{"id": "default", "name": "默认"},
			},
		},
		"inputs": []map[string]string{
			{"key": "hosts", "type": "array[string]", "description": "主机列表"},
		},
		"outputs": []map[string]string{
			{"key": "validated_hosts", "type": "array[object]", "description": "验证后的主机详情"},
			{"key": "validation_summary", "type": "object", "description": "验证统计 {total, valid, invalid}"},
		},
	}
}

func approvalNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "审批节点",
		"description": "等待人工审批，有两个输出分支",
		"config": map[string]interface{}{
			"title":          map[string]string{"type": "string", "required": "true", "description": "审批标题"},
			"description":    map[string]string{"type": "string", "description": "审批说明"},
			"approvers":      map[string]string{"type": "array[string]", "description": "审批人用户名列表"},
			"approver_roles": map[string]string{"type": "array[string]", "description": "审批人角色列表"},
			"timeout_hours":  map[string]string{"type": "number", "default": "24", "description": "超时时间(小时)"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 2,
			"out_ports": []map[string]string{
				{"id": "approved", "name": "通过", "condition": "审批通过时"},
				{"id": "rejected", "name": "拒绝", "condition": "审批拒绝或超时时"},
			},
		},
		"inputs":  []interface{}{},
		"outputs": []interface{}{},
	}
}

func executionNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "执行节点",
		"description": "执行任务模板，根据执行结果走不同分支",
		"config": map[string]interface{}{
			"task_template_id": map[string]string{"type": "string", "required": "true", "description": "任务模板ID"},
			"hosts_key":        map[string]string{"type": "string", "default": "validated_hosts", "description": "主机列表变量名"},
			"extra_vars":       map[string]string{"type": "object", "description": "额外变量"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 3,
			"out_ports": []map[string]string{
				{"id": "success", "name": "成功", "condition": "所有主机执行成功"},
				{"id": "partial", "name": "部分成功", "condition": "部分主机成功，部分失败"},
				{"id": "failed", "name": "失败", "condition": "全部失败或取消/超时/错误"},
			},
		},
		"inputs": []map[string]string{
			{"key": "validated_hosts", "type": "array[object]", "description": "目标主机"},
		},
		"outputs": []map[string]string{
			{"key": "execution_result", "type": "object", "description": "执行结果，包含 status(success/partial/failed), stats 等"},
		},
	}
}

func notificationNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "通知节点",
		"description": "发送通知",
		"config": map[string]interface{}{
			"template_id": map[string]string{"type": "string", "required": "true", "description": "通知模板ID"},
			"channel_ids": map[string]string{"type": "array[string]", "required": "true", "description": "通知渠道ID列表"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 1,
			"out_ports": []map[string]string{
				{"id": "default", "name": "默认"},
			},
		},
		"inputs":  []interface{}{},
		"outputs": []interface{}{},
	}
}

func conditionNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "条件分支",
		"description": "根据条件选择执行路径，有两个输出分支",
		"config": map[string]interface{}{
			"condition":    map[string]string{"type": "string", "required": "true", "description": "条件表达式，如 execution_result.status == 'success'"},
			"true_target":  map[string]string{"type": "string", "description": "条件为真时跳转的节点ID（前端自动填充）"},
			"false_target": map[string]string{"type": "string", "description": "条件为假时跳转的节点ID（前端自动填充）"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 2,
			"out_ports": []map[string]string{
				{"id": "true", "name": "是", "condition": "条件为真"},
				{"id": "false", "name": "否", "condition": "条件为假"},
			},
		},
		"inputs":  []interface{}{},
		"outputs": []interface{}{},
	}
}

func setVariableNodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "设置变量",
		"description": "设置或修改上下文变量",
		"config": map[string]interface{}{
			"key":   map[string]string{"type": "string", "required": "true", "description": "变量名"},
			"value": map[string]string{"type": "any", "required": "true", "description": "变量值"},
		},
		"ports": map[string]interface{}{
			"in":  1,
			"out": 1,
			"out_ports": []map[string]string{
				{"id": "default", "name": "默认"},
			},
		},
		"inputs":  []interface{}{},
		"outputs": []interface{}{},
	}
}
