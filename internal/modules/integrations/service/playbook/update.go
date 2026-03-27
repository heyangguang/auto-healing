package playbook

import (
	"fmt"
	"strings"
)

type UpdateInput struct {
	Name        *string
	Description *string
}

func applyPlaybookUpdate(targetName, targetDescription *string, input *UpdateInput) {
	if input == nil {
		return
	}
	if input.Name != nil {
		*targetName = *input.Name
	}
	if input.Description != nil {
		*targetDescription = *input.Description
	}
}

func validatePlaybookUpdateInput(input *UpdateInput) error {
	if input == nil {
		return nil
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return fmt.Errorf("Playbook 名称不能为空")
	}
	return nil
}
