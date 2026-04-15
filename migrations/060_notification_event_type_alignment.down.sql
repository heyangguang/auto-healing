UPDATE notification_channels
SET type = 'dingtalk',
    updated_at = NOW()
WHERE type = 'wecom'
  AND COALESCE(config->>'webhook_url', '') LIKE 'https://qyapi.weixin.qq.com/%';

UPDATE notification_templates
SET supported_channels = REPLACE(supported_channels::text, '"wecom"', '"dingtalk"')::jsonb,
    updated_at = NOW()
WHERE (name ILIKE '%企业微信%' OR description ILIKE '%企业微信%')
  AND supported_channels::text LIKE '%"wecom"%';

UPDATE notification_templates
SET event_type = CASE
    WHEN event_type = 'execution_started' THEN 'execution_result'
    WHEN event_type = 'flow_result' THEN 'custom'
    ELSE event_type
END,
updated_at = NOW()
WHERE event_type IN ('execution_started', 'flow_result');
