-- 重命名 notification_channels 表的 default_recipients 字段为 recipients
ALTER TABLE notification_channels RENAME COLUMN default_recipients TO recipients;
