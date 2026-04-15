package notification

import (
	"fmt"
	"strings"
)

const (
	NotificationEventTypeExecutionStarted   = "execution_started"
	NotificationEventTypeExecutionResult    = "execution_result"
	NotificationEventTypeFlowResult         = "flow_result"
	NotificationEventTypeApprovalRequired   = "approval_required"
	NotificationEventTypeManualNotification = "manual_notification"
)

var notificationEventTypes = map[string]struct{}{
	NotificationEventTypeExecutionStarted:   {},
	NotificationEventTypeExecutionResult:    {},
	NotificationEventTypeFlowResult:         {},
	NotificationEventTypeApprovalRequired:   {},
	NotificationEventTypeManualNotification: {},
}

func normalizeNotificationEventType(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func validateNotificationEventType(value string) (string, error) {
	eventType := normalizeNotificationEventType(value)
	if eventType == "" {
		return NotificationEventTypeManualNotification, nil
	}
	if _, ok := notificationEventTypes[eventType]; ok {
		return eventType, nil
	}
	return "", fmt.Errorf("%w: %s", ErrNotificationUnsupportedEventType, value)
}
