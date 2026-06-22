package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"github.com/google/uuid"
)

const maxIncidentCloseErrorDetailRunes = 600

// IncidentCloseWritebackError carries a user-safe close/writeback failure.
// Full request and response bodies remain in the writeback log for operators.
type IncidentCloseWritebackError struct {
	Cause              error
	WritebackLogID     *uuid.UUID
	ResponseStatusCode *int
	Detail             string
}

func (e *IncidentCloseWritebackError) Error() string {
	if e == nil || e.Cause == nil {
		return "关闭工单回写失败"
	}
	return e.Cause.Error()
}

func (e *IncidentCloseWritebackError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *IncidentCloseWritebackError) UserMessage() string {
	if e == nil {
		return "关闭工单失败"
	}

	message := "源系统回写失败"
	if e.ResponseStatusCode != nil {
		message = fmt.Sprintf("%s（HTTP %d）", message, *e.ResponseStatusCode)
	}
	if e.Detail != "" {
		message += "：" + e.Detail
	}
	if e.WritebackLogID != nil {
		message += fmt.Sprintf("。回写记录ID：%s", e.WritebackLogID.String())
	}
	return message
}

func NewIncidentCloseWritebackError(
	cause error,
	logEntry *platformmodel.IncidentWritebackLog,
	result *IncidentWritebackHTTPResult,
) *IncidentCloseWritebackError {
	err := &IncidentCloseWritebackError{
		Cause:          cause,
		WritebackLogID: writebackLogID(logEntry),
		Detail:         summarizeCloseWritebackFailure(cause, result),
	}
	if result != nil {
		err.ResponseStatusCode = intPointer(result.StatusCode)
	}
	return err
}

func summarizeCloseWritebackFailure(cause error, result *IncidentWritebackHTTPResult) string {
	if result != nil && strings.TrimSpace(result.ResponseBody) != "" {
		if message := extractCloseWritebackResponseMessage(result.ResponseBody); message != "" {
			return sanitizeCloseWritebackDetail(message)
		}
		return sanitizeCloseWritebackDetail(result.ResponseBody)
	}
	if cause != nil {
		return sanitizeCloseWritebackDetail(cause.Error())
	}
	return ""
}

func extractCloseWritebackResponseMessage(body string) string {
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	return findCloseWritebackMessage(payload)
}

func findCloseWritebackMessage(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"message", "error_message", "detail", "description", "error"} {
			raw, ok := typed[key]
			if !ok {
				continue
			}
			if message, ok := raw.(string); ok && strings.TrimSpace(message) != "" {
				return message
			}
			if nested := findCloseWritebackMessage(raw); nested != "" {
				return nested
			}
		}
		for _, raw := range typed {
			if nested := findCloseWritebackMessage(raw); nested != "" {
				return nested
			}
		}
	case []any:
		for _, item := range typed {
			if nested := findCloseWritebackMessage(item); nested != "" {
				return nested
			}
		}
	}
	return ""
}

func sanitizeCloseWritebackDetail(detail string) string {
	detail = strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		return r
	}, detail)
	detail = strings.Join(strings.Fields(strings.TrimSpace(detail)), " ")
	runes := []rune(detail)
	if len(runes) > maxIncidentCloseErrorDetailRunes {
		return string(runes[:maxIncidentCloseErrorDetailRunes-3]) + "..."
	}
	return detail
}
