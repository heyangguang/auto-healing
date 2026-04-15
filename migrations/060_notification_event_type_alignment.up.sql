UPDATE notification_channels
SET type = 'wecom',
    updated_at = NOW()
WHERE type = 'dingtalk'
  AND COALESCE(config->>'webhook_url', '') LIKE 'https://qyapi.weixin.qq.com/%';

UPDATE notification_templates
SET supported_channels = REPLACE(supported_channels::text, '"dingtalk"', '"wecom"')::jsonb,
    updated_at = NOW()
WHERE (name ILIKE '%企业微信%' OR description ILIKE '%企业微信%')
  AND supported_channels::text LIKE '%"dingtalk"%';

UPDATE notification_templates
SET event_type = CASE
    WHEN COALESCE(event_type, '') IN ('execution.start', 'execution_started') THEN 'execution_started'
    WHEN COALESCE(event_type, '') IN ('execution.success', 'execution.failed', 'execution_result') THEN 'execution_result'
    WHEN COALESCE(event_type, '') IN ('flow_execution', 'flow_result') THEN 'flow_result'
    WHEN COALESCE(event_type, '') = 'approval_required' THEN 'approval_required'
    WHEN COALESCE(event_type, '') IN ('incident_created', 'incident_resolved', 'alert', 'acceptance.event', '') THEN 'custom'
    WHEN COALESCE(event_type, '') = 'custom' AND (name ILIKE '%自愈流程%' OR body_template ILIKE '%flow_instance_id%') THEN 'flow_result'
    ELSE 'custom'
END,
updated_at = NOW()
WHERE event_type IS DISTINCT FROM CASE
    WHEN COALESCE(event_type, '') IN ('execution.start', 'execution_started') THEN 'execution_started'
    WHEN COALESCE(event_type, '') IN ('execution.success', 'execution.failed', 'execution_result') THEN 'execution_result'
    WHEN COALESCE(event_type, '') IN ('flow_execution', 'flow_result') THEN 'flow_result'
    WHEN COALESCE(event_type, '') = 'approval_required' THEN 'approval_required'
    WHEN COALESCE(event_type, '') IN ('incident_created', 'incident_resolved', 'alert', 'acceptance.event', '') THEN 'custom'
    WHEN COALESCE(event_type, '') = 'custom' AND (name ILIKE '%自愈流程%' OR body_template ILIKE '%flow_instance_id%') THEN 'flow_result'
    ELSE 'custom'
END;
