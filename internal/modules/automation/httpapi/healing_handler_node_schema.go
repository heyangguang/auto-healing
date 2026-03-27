package httpapi

func healingNodeSchema() healingNodeSchemaResponse {
	return healingNodeSchemaResponse{
		InitialContext: healingInitialContextSchema(),
		Nodes:          healingNodeDefinitions(),
	}
}

func healingInitialContextSchema() map[string]healingSchemaObject {
	return map[string]healingSchemaObject{
		"incident": {
			Type:        "object",
			Description: "触发流程的工单数据",
			Properties: map[string]healingSchemaProperty{
				"id":               {Type: "string", Description: "工单ID"},
				"title":            {Type: "string", Description: "工单标题"},
				"description":      {Type: "string", Description: "工单描述"},
				"severity":         {Type: "string", Description: "严重级别"},
				"priority":         {Type: "string", Description: "优先级"},
				"status":           {Type: "string", Description: "状态"},
				"category":         {Type: "string", Description: "分类"},
				"affected_ci":      {Type: "string", Description: "影响的CI（多个用逗号分隔）"},
				"affected_service": {Type: "string", Description: "影响的服务"},
				"assignee":         {Type: "string", Description: "处理人"},
				"reporter":         {Type: "string", Description: "报告人"},
				"raw_data":         {Type: "object", Description: "原始数据（来自第三方系统）"},
			},
		},
	}
}

func healingNodeDefinitions() map[string]healingNodeDefinition {
	return map[string]healingNodeDefinition{
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

func startNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "开始",
		Description: "流程起始节点",
		Config:      map[string]healingConfigField{},
		Ports:       defaultNodePorts(0, 1),
		Inputs:      []healingNodeIO{},
		Outputs: []healingNodeIO{
			{Key: "incident", Type: "object", Description: "工单对象"},
		},
	}
}

func endNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "结束",
		Description: "流程结束节点",
		Config:      map[string]healingConfigField{},
		Ports:       healingNodePorts{In: 1, Out: 0},
		Inputs:      []healingNodeIO{},
		Outputs:     []healingNodeIO{},
	}
}

func hostExtractorNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "主机提取器",
		Description: "从工单数据中提取主机列表",
		Config: map[string]healingConfigField{
			"source_field": {Type: "string", Required: true, Description: "数据来源字段，如 incident.affected_ci 或 incident.raw_data.cmdb_ci"},
			"extract_mode": {Type: "string", Default: "split", Description: "提取模式：split(分割) 或 regex(正则)"},
			"split_by":     {Type: "string", Default: ",", Description: "分割符（extract_mode=split时使用）"},
			"output_key":   {Type: "string", Default: "hosts", Description: "输出变量名"},
		},
		Ports: defaultNodePorts(1, 1),
		Inputs: []healingNodeIO{
			{Key: "incident", Type: "object", Description: "工单对象"},
		},
		Outputs: []healingNodeIO{
			{Key: "hosts", Type: "array[string]", Description: "提取的主机列表"},
		},
	}
}

func cmdbValidatorNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "CMDB验证器",
		Description: "验证主机是否在CMDB中存在，并获取主机详细信息",
		Config: map[string]healingConfigField{
			"input_key":  {Type: "string", Default: "hosts", Description: "输入变量名"},
			"output_key": {Type: "string", Default: "validated_hosts", Description: "输出变量名"},
		},
		Ports: defaultNodePorts(1, 1),
		Inputs: []healingNodeIO{
			{Key: "hosts", Type: "array[string]", Description: "主机列表"},
		},
		Outputs: []healingNodeIO{
			{Key: "validated_hosts", Type: "array[object]", Description: "验证后的主机详情"},
			{Key: "validation_summary", Type: "object", Description: "验证统计 {total, valid, invalid}"},
		},
	}
}

func approvalNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "审批节点",
		Description: "等待人工审批，有两个输出分支",
		Config: map[string]healingConfigField{
			"title":          {Type: "string", Required: true, Description: "审批标题"},
			"description":    {Type: "string", Description: "审批说明"},
			"approvers":      {Type: "array[string]", Description: "审批人用户名列表"},
			"approver_roles": {Type: "array[string]", Description: "审批人角色列表"},
			"timeout_hours":  {Type: "number", Default: "24", Description: "超时时间(小时)"},
		},
		Ports: healingNodePorts{
			In:  1,
			Out: 2,
			OutPorts: []healingNodePortOption{
				{ID: "approved", Name: "通过", Condition: "审批通过时"},
				{ID: "rejected", Name: "拒绝", Condition: "审批拒绝或超时时"},
			},
		},
		Inputs:  []healingNodeIO{},
		Outputs: []healingNodeIO{},
	}
}

func executionNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "执行节点",
		Description: "执行任务模板，根据执行结果走不同分支",
		Config: map[string]healingConfigField{
			"task_template_id": {Type: "string", Required: true, Description: "任务模板ID"},
			"hosts_key":        {Type: "string", Default: "validated_hosts", Description: "主机列表变量名"},
			"extra_vars":       {Type: "object", Description: "额外变量"},
		},
		Ports: healingNodePorts{
			In:  1,
			Out: 3,
			OutPorts: []healingNodePortOption{
				{ID: "success", Name: "成功", Condition: "所有主机执行成功"},
				{ID: "partial", Name: "部分成功", Condition: "部分主机成功，部分失败"},
				{ID: "failed", Name: "失败", Condition: "全部失败或取消/超时/错误"},
			},
		},
		Inputs: []healingNodeIO{
			{Key: "validated_hosts", Type: "array[object]", Description: "目标主机"},
		},
		Outputs: []healingNodeIO{
			{Key: "execution_result", Type: "object", Description: "执行结果，包含 status(success/partial/failed), stats 等"},
		},
	}
}

func notificationNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "通知节点",
		Description: "发送通知",
		Config: map[string]healingConfigField{
			"template_id": {Type: "string", Required: true, Description: "通知模板ID"},
			"channel_ids": {Type: "array[string]", Required: true, Description: "通知渠道ID列表"},
		},
		Ports:   defaultNodePorts(1, 1),
		Inputs:  []healingNodeIO{},
		Outputs: []healingNodeIO{},
	}
}

func conditionNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "条件分支",
		Description: "根据条件选择执行路径，有两个输出分支",
		Config: map[string]healingConfigField{
			"condition":    {Type: "string", Required: true, Description: "条件表达式，如 execution_result.status == 'success'"},
			"true_target":  {Type: "string", Description: "条件为真时跳转的节点ID（前端自动填充）"},
			"false_target": {Type: "string", Description: "条件为假时跳转的节点ID（前端自动填充）"},
		},
		Ports: healingNodePorts{
			In:  1,
			Out: 2,
			OutPorts: []healingNodePortOption{
				{ID: "true", Name: "是", Condition: "条件为真"},
				{ID: "false", Name: "否", Condition: "条件为假"},
			},
		},
		Inputs:  []healingNodeIO{},
		Outputs: []healingNodeIO{},
	}
}

func setVariableNodeSchema() healingNodeDefinition {
	return healingNodeDefinition{
		Name:        "设置变量",
		Description: "设置或修改上下文变量",
		Config: map[string]healingConfigField{
			"key":   {Type: "string", Required: true, Description: "变量名"},
			"value": {Type: "any", Required: true, Description: "变量值"},
		},
		Ports:   defaultNodePorts(1, 1),
		Inputs:  []healingNodeIO{},
		Outputs: []healingNodeIO{},
	}
}

func defaultNodePorts(in, out int) healingNodePorts {
	return healingNodePorts{
		In:  in,
		Out: out,
		OutPorts: []healingNodePortOption{
			{ID: "default", Name: "默认"},
		},
	}
}
