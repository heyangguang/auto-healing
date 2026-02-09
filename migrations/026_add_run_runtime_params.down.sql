-- 回滚执行记录运行时参数快照字段

ALTER TABLE execution_runs
DROP COLUMN IF EXISTS runtime_target_hosts,
DROP COLUMN IF EXISTS runtime_secrets_source_ids,
DROP COLUMN IF EXISTS runtime_extra_vars,
DROP COLUMN IF EXISTS runtime_skip_notification;
