package modeltypes

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type NotificationTriggerConfig struct {
	Enabled    bool        `json:"enabled"`
	ChannelIDs []uuid.UUID `json:"channel_ids,omitempty"`
	TemplateID *uuid.UUID  `json:"template_id,omitempty"`
}

type TaskNotificationConfig struct {
	Enabled   bool                       `json:"enabled"`
	OnStart   *NotificationTriggerConfig `json:"on_start,omitempty"`
	OnSuccess *NotificationTriggerConfig `json:"on_success,omitempty"`
	OnFailure *NotificationTriggerConfig `json:"on_failure,omitempty"`
}

func (c *TaskNotificationConfig) GetTriggerConfig(status string) *NotificationTriggerConfig {
	if c == nil || !c.Enabled {
		return nil
	}
	switch status {
	case "start":
		return c.OnStart
	case "success":
		return c.OnSuccess
	case "failed", "timeout":
		return c.OnFailure
	default:
		return nil
	}
}

func (c *TaskNotificationConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal TaskNotificationConfig: %v", value)
	}
	return json.Unmarshal(bytes, c)
}

func (c TaskNotificationConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}
