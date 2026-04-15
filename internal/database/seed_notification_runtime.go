package database

import "github.com/company/auto-healing/internal/pkg/logger"

var notificationAlignmentSQL = []string{
	`
	UPDATE notification_channels
	SET type = 'wecom',
	    updated_at = NOW()
	WHERE type = 'dingtalk'
	  AND COALESCE(config->>'webhook_url', '') LIKE 'https://qyapi.weixin.qq.com/%'
	`,
	`
	UPDATE notification_channels
	SET config = config - 'mentioned_list' - 'mentioned_mobile_list',
	    updated_at = NOW()
	WHERE type = 'wecom'
	  AND (config ? 'mentioned_list' OR config ? 'mentioned_mobile_list')
	`,
	`
	UPDATE notification_templates
	SET supported_channels = REPLACE(supported_channels::text, '"dingtalk"', '"wecom"')::jsonb,
	    updated_at = NOW()
	WHERE (name ILIKE '%企业微信%' OR description ILIKE '%企业微信%')
	  AND supported_channels::text LIKE '%"dingtalk"%'
	`,
	`
	UPDATE execution_runs
	SET triggered_by = CASE
	    WHEN COALESCE(triggered_by, '') = '' THEN 'manual'
	    WHEN LOWER(triggered_by) LIKE 'manual%' THEN 'manual'
	    WHEN LOWER(triggered_by) LIKE 'scheduler:cron%' THEN 'scheduler:cron'
	    WHEN LOWER(triggered_by) LIKE 'scheduler:once%' THEN 'scheduler:once'
	    WHEN LOWER(triggered_by) LIKE 'healing%' OR LOWER(triggered_by) LIKE 'workflow%' THEN 'healing'
	    ELSE LOWER(triggered_by)
	END
	WHERE triggered_by IS DISTINCT FROM CASE
	    WHEN COALESCE(triggered_by, '') = '' THEN 'manual'
	    WHEN LOWER(triggered_by) LIKE 'manual%' THEN 'manual'
	    WHEN LOWER(triggered_by) LIKE 'scheduler:cron%' THEN 'scheduler:cron'
	    WHEN LOWER(triggered_by) LIKE 'scheduler:once%' THEN 'scheduler:once'
	    WHEN LOWER(triggered_by) LIKE 'healing%' OR LOWER(triggered_by) LIKE 'workflow%' THEN 'healing'
	    ELSE LOWER(triggered_by)
	END
	`,
	`
	UPDATE notification_templates
	SET event_type = CASE
	    WHEN name ILIKE '%执行任务-开始%' OR subject_template ILIKE '[开始]%' THEN 'execution_started'
	    WHEN COALESCE(event_type, '') IN ('execution.start', 'execution_started') THEN 'execution_started'
	    WHEN COALESCE(event_type, '') IN ('execution.success', 'execution.failed', 'execution_result') THEN 'execution_result'
	    WHEN COALESCE(event_type, '') IN ('flow_execution', 'flow_result') THEN 'flow_result'
	    WHEN COALESCE(event_type, '') = 'approval_required' THEN 'approval_required'
	    WHEN COALESCE(event_type, '') = 'manual_notification' THEN 'manual_notification'
	    WHEN COALESCE(event_type, '') IN ('incident_created', 'incident_resolved', 'alert', 'acceptance.event', '') THEN 'manual_notification'
	    WHEN COALESCE(event_type, '') = 'custom' AND (name ILIKE '%自愈流程%' OR body_template ILIKE '%flow_instance_id%') THEN 'flow_result'
	    WHEN COALESCE(event_type, '') = 'custom' THEN 'manual_notification'
	    ELSE 'manual_notification'
	END,
	updated_at = NOW()
	WHERE event_type IS DISTINCT FROM CASE
	    WHEN name ILIKE '%执行任务-开始%' OR subject_template ILIKE '[开始]%' THEN 'execution_started'
	    WHEN COALESCE(event_type, '') IN ('execution.start', 'execution_started') THEN 'execution_started'
	    WHEN COALESCE(event_type, '') IN ('execution.success', 'execution.failed', 'execution_result') THEN 'execution_result'
	    WHEN COALESCE(event_type, '') IN ('flow_execution', 'flow_result') THEN 'flow_result'
	    WHEN COALESCE(event_type, '') = 'approval_required' THEN 'approval_required'
	    WHEN COALESCE(event_type, '') = 'manual_notification' THEN 'manual_notification'
	    WHEN COALESCE(event_type, '') IN ('incident_created', 'incident_resolved', 'alert', 'acceptance.event', '') THEN 'manual_notification'
	    WHEN COALESCE(event_type, '') = 'custom' AND (name ILIKE '%自愈流程%' OR body_template ILIKE '%flow_instance_id%') THEN 'flow_result'
	    WHEN COALESCE(event_type, '') = 'custom' THEN 'manual_notification'
	    ELSE 'manual_notification'
	END
	`,
	`
	WITH matched_logs AS (
	    SELECT
	        n.id AS log_id,
	        i.id AS matched_incident_id,
	        fi.id AS matched_workflow_instance_id
	    FROM notification_logs n
	    JOIN notification_templates t ON t.id = n.template_id
	    JOIN incidents i ON i.title = regexp_replace(COALESCE(n.subject, ''), '^\[[^]]+\]\s*', '')
	    LEFT JOIN LATERAL (
	        SELECT f.id
	        FROM flow_instances f
	        WHERE f.incident_id = i.id
	        ORDER BY ABS(EXTRACT(EPOCH FROM (n.created_at - f.created_at))) ASC
	        LIMIT 1
	    ) fi ON TRUE
	    WHERE t.event_type IN ('flow_result', 'approval_required')
	      AND (n.incident_id IS NULL OR n.workflow_instance_id IS NULL)
	)
	UPDATE notification_logs n
	SET incident_id = COALESCE(n.incident_id, matched_logs.matched_incident_id),
	    workflow_instance_id = COALESCE(n.workflow_instance_id, matched_logs.matched_workflow_instance_id)
	FROM matched_logs
	WHERE n.id = matched_logs.log_id
	  AND (
	      COALESCE(n.incident_id, matched_logs.matched_incident_id) IS DISTINCT FROM n.incident_id
	      OR COALESCE(n.workflow_instance_id, matched_logs.matched_workflow_instance_id) IS DISTINCT FROM n.workflow_instance_id
	  )
	`,
}

// AlignNotificationData 对齐通知模块历史数据（幂等）
func AlignNotificationData() error {
	affected := int64(0)
	for _, sql := range notificationAlignmentSQL {
		result := DB.Exec(sql)
		if result.Error != nil {
			return result.Error
		}
		affected += result.RowsAffected
	}
	logger.Info("通知模块历史数据对齐完成，修正 %d 条记录", affected)
	return nil
}
