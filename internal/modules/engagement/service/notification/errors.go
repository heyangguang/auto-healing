package notification

import "errors"

var (
	ErrNotificationChannelNotFound      = errors.New("通知渠道不存在")
	ErrNotificationChannelExists        = errors.New("通知渠道名称已存在")
	ErrNotificationChannelInactive      = errors.New("通知渠道已停用")
	ErrNotificationTemplateNotFound     = errors.New("通知模板不存在")
	ErrNotificationLogNotFound          = errors.New("通知记录不存在")
	ErrNotificationUnsupportedType      = errors.New("不支持的通知渠道类型")
	ErrNotificationUnsupportedEventType = errors.New("不支持的通知事件类型")
	ErrNotificationResourceInUse        = errors.New("通知资源仍被引用")
	ErrNotificationLogPersistenceFailed = errors.New("通知日志持久化失败")
	ErrNotificationSendAllFailed        = errors.New("所有通知发送均失败")
)
