-- 回滚：将 recipients  重命名回 default_recipients
ALTER TABLE notification_channels RENAME COLUMN recipients TO default_recipients;
