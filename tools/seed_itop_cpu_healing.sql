BEGIN;

INSERT INTO healing_flows (
  id,
  tenant_id,
  name,
  description,
  nodes,
  edges,
  is_active,
  created_by,
  created_at,
  updated_at,
  auto_close_source_incident
) VALUES (
  '7e44389b-3db8-4db2-a0e2-3e6c8d6d2053',
  'd9dffb0d-1c74-46f6-90a5-3bc05fc0af43',
  'iTop CPU 高负载自动恢复',
  'iTop CPU 高负载工单命中 real-host-77 时，自动执行 CPU 恢复任务，并发送流程结果通知。',
  $$[
    {"id":"start_cpu_auto","name":"开始","type":"start","config":{},"position":{"x":80,"y":220}},
    {"id":"extract_cpu_hosts","name":"提取工单主机","type":"host_extractor","config":{"split_by":",","output_key":"hosts","extract_mode":"split","source_field":"incident.affected_ci"},"position":{"x":280,"y":220}},
    {"id":"validate_cpu_hosts","name":"CMDB验证","type":"cmdb_validator","config":{"input_key":"hosts","output_key":"validated_hosts","fail_on_not_found":true},"position":{"x":500,"y":220}},
    {"id":"execute_cpu_recovery","name":"执行CPU恢复","type":"execution","config":{"hosts_key":"validated_hosts","extra_vars":{"healing_mode":"auto","healing_source":"itop","healing_scenario":"cpu_high"},"task_template_id":"4e03b26c-0d93-4421-88fb-333c2e1d9cac"},"position":{"x":760,"y":220}},
    {"id":"notify_cpu_success","name":"通知-恢复完成","type":"notification","config":{"channel_ids":["f0cdd80a-f844-4b88-bd86-94231eeee4fc"],"template_id":"d392abe5-1890-4f88-802b-2c982c343c32","include_incident_info":true,"include_execution_result":true},"position":{"x":1030,"y":160}},
    {"id":"notify_cpu_failed","name":"通知-恢复失败","type":"notification","config":{"channel_ids":["f0cdd80a-f844-4b88-bd86-94231eeee4fc"],"template_id":"5587ddd2-9fbd-43c2-bb21-00de568b6696","include_incident_info":true,"include_execution_result":true},"position":{"x":1030,"y":280}},
    {"id":"end_cpu_auto","name":"结束","type":"end","config":{},"position":{"x":1280,"y":220}}
  ]$$::jsonb,
  $$[
    {"source":"start_cpu_auto","target":"extract_cpu_hosts","sourceHandle":"default"},
    {"source":"extract_cpu_hosts","target":"validate_cpu_hosts","sourceHandle":"default"},
    {"source":"validate_cpu_hosts","target":"execute_cpu_recovery","sourceHandle":"default"},
    {"source":"execute_cpu_recovery","target":"notify_cpu_success","sourceHandle":"success"},
    {"source":"execute_cpu_recovery","target":"notify_cpu_failed","sourceHandle":"partial"},
    {"source":"execute_cpu_recovery","target":"notify_cpu_failed","sourceHandle":"failed"},
    {"source":"notify_cpu_success","target":"end_cpu_auto","sourceHandle":"default"},
    {"source":"notify_cpu_failed","target":"end_cpu_auto","sourceHandle":"default"}
  ]$$::jsonb,
  true,
  'b56c9caa-8218-4d7f-be39-f15009052e66',
  now(),
  now(),
  true
)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  nodes = EXCLUDED.nodes,
  edges = EXCLUDED.edges,
  is_active = EXCLUDED.is_active,
  updated_at = now(),
  auto_close_source_incident = EXCLUDED.auto_close_source_incident;

INSERT INTO healing_rules (
  id,
  tenant_id,
  name,
  description,
  priority,
  trigger_mode,
  conditions,
  match_mode,
  flow_id,
  is_active,
  created_by,
  created_at,
  updated_at
) VALUES (
  '322f2f31-f647-4b2f-9d9e-08ebcc44d29d',
  'd9dffb0d-1c74-46f6-90a5-3bc05fc0af43',
  'iTop CPU 高负载自动恢复规则',
  '匹配 iTop Adapter ITSM 的真实主机 CPU 高负载工单，自动进入 CPU 恢复流程。',
  30,
  'auto',
  $$[
    {"field":"source_plugin_name","value":"iTop Adapter ITSM","operator":"equals"},
    {"field":"category","value":"incident","operator":"equals"},
    {"field":"affected_ci","value":"real-host-77","operator":"contains"},
    {"field":"title","value":"cpu_usage_high","operator":"contains"}
  ]$$::jsonb,
  'all',
  '7e44389b-3db8-4db2-a0e2-3e6c8d6d2053',
  true,
  'b56c9caa-8218-4d7f-be39-f15009052e66',
  now(),
  now()
)
ON CONFLICT (id) DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  priority = EXCLUDED.priority,
  trigger_mode = EXCLUDED.trigger_mode,
  conditions = EXCLUDED.conditions,
  match_mode = EXCLUDED.match_mode,
  flow_id = EXCLUDED.flow_id,
  is_active = EXCLUDED.is_active,
  updated_at = now();

COMMIT;
