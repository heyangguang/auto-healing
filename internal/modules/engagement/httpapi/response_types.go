package httpapi

import (
	"encoding/json"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	"github.com/google/uuid"
)

type preferenceResponse struct {
	UserID      uuid.UUID       `json:"user_id"`
	Preferences json.RawMessage `json:"preferences"`
}

type notificationSendResponse struct {
	NotificationIDs []string                 `json:"notification_ids"`
	Logs            []*model.NotificationLog `json:"logs"`
}

type notificationSendFailureDetails struct {
	NotificationIDs []string                 `json:"notification_ids"`
	Logs            []*model.NotificationLog `json:"logs"`
}
