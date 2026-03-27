package httpapi

import (
	"errors"
	"net/http"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func writeNotificationLookupError(c *gin.Context, err error, notFoundMsg, internalMsg string) {
	switch {
	case errors.Is(err, notification.ErrNotificationChannelNotFound),
		errors.Is(err, notification.ErrNotificationTemplateNotFound),
		errors.Is(err, notification.ErrNotificationLogNotFound):
		response.NotFound(c, notFoundMsg)
	default:
		respondInternalError(c, "NOTIFY", internalMsg, err)
	}
}

func writeNotificationMutationError(c *gin.Context, err error, notFoundMsg, internalMsg string) {
	switch {
	case errors.Is(err, notification.ErrNotificationChannelNotFound),
		errors.Is(err, notification.ErrNotificationTemplateNotFound):
		response.NotFound(c, notFoundMsg)
	case errors.Is(err, notification.ErrNotificationUnsupportedType),
		errors.Is(err, notification.ErrNotificationChannelInactive):
		response.BadRequest(c, err.Error())
	case errors.Is(err, notification.ErrNotificationChannelExists):
		response.Conflict(c, err.Error())
	case errors.Is(err, notification.ErrNotificationResourceInUse):
		response.Conflict(c, err.Error())
	default:
		respondInternalError(c, "NOTIFY", internalMsg, err)
	}
}

func writeNotificationSendError(c *gin.Context, err error, logs []*model.NotificationLog) {
	switch {
	case errors.Is(err, notification.ErrNotificationChannelNotFound),
		errors.Is(err, notification.ErrNotificationTemplateNotFound),
		errors.Is(err, notification.ErrNotificationChannelInactive):
		response.BadRequest(c, err.Error())
	case errors.Is(err, notification.ErrNotificationSendAllFailed):
		response.ErrorWithMetadata(
			c,
			http.StatusInternalServerError,
			response.CodeInternal,
			"通知发送失败",
			"",
			notificationSendFailureDetails{
				NotificationIDs: notificationLogIDs(logs),
				Logs:            logs,
			},
		)
	default:
		respondInternalError(c, "NOTIFY", "发送通知失败", err)
	}
}
