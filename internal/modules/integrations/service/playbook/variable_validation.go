package playbook

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

var playbookVariableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var allowedPlaybookVariableTypes = map[string]struct{}{
	"string":  {},
	"number":  {},
	"integer": {},
	"boolean": {},
	"bool":    {},
	"list":    {},
	"array":   {},
	"object":  {},
	"dict":    {},
	"enum":    {},
	"select":  {},
}

func validatePlaybookVariables(variables model.JSONArray) error {
	seen := make(map[string]struct{}, len(variables))
	for i, raw := range variables {
		variable, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("variables[%d] 必须是对象", i)
		}
		name, _ := variable["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("variables[%d].name 不能为空", i)
		}
		if !playbookVariableNamePattern.MatchString(name) {
			return fmt.Errorf("variables[%d].name 不是合法 Ansible 变量名: %s", i, name)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("variables[%d].name 重复: %s", i, name)
		}
		seen[name] = struct{}{}
		variableType := normalizedVariableType(variable["type"])
		if _, ok := allowedPlaybookVariableTypes[variableType]; !ok {
			return fmt.Errorf("variables[%d].type 不支持: %s", i, variableType)
		}
		if err := validateVariableOptionalString(variable, "description", i); err != nil {
			return err
		}
		if err := validateVariableOptionalString(variable, "source_file", i); err != nil {
			return err
		}
		if err := validateVariableOptionalBool(variable, "required", i); err != nil {
			return err
		}
		if err := validateVariableOptionalBool(variable, "in_code", i); err != nil {
			return err
		}
		if err := validateVariableEnum(variable, variableType, i); err != nil {
			return err
		}
		if err := validateVariableDefault(variable, variableType, i); err != nil {
			return err
		}
		if err := validateVariableBounds(variable, variableType, i); err != nil {
			return err
		}
		if err := validateVariablePattern(variable, variableType, i); err != nil {
			return err
		}
	}
	return nil
}

func normalizedVariableType(raw interface{}) string {
	value, _ := raw.(string)
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "string"
	}
	return value
}

func validateVariableOptionalString(variable map[string]interface{}, field string, index int) error {
	if value, exists := variable[field]; exists && value != nil {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("variables[%d].%s 必须是字符串", index, field)
		}
	}
	return nil
}

func validateVariableOptionalBool(variable map[string]interface{}, field string, index int) error {
	if value, exists := variable[field]; exists && value != nil {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("variables[%d].%s 必须是布尔值", index, field)
		}
	}
	return nil
}

func validateVariableEnum(variable map[string]interface{}, variableType string, index int) error {
	raw, exists := variable["enum"]
	if !exists || raw == nil {
		if variableType == "enum" || variableType == "select" {
			return fmt.Errorf("variables[%d].enum 类型变量必须配置 enum", index)
		}
		return nil
	}
	values, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("variables[%d].enum 必须是字符串数组", index)
	}
	if len(values) == 0 {
		return fmt.Errorf("variables[%d].enum 不能为空", index)
	}
	seen := map[string]struct{}{}
	for j, rawValue := range values {
		value, ok := rawValue.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("variables[%d].enum[%d] 必须是非空字符串", index, j)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("variables[%d].enum[%d] 重复: %s", index, j, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateVariableDefault(variable map[string]interface{}, variableType string, index int) error {
	raw, exists := variable["default"]
	if !exists || raw == nil {
		return nil
	}
	switch variableType {
	case "string", "enum", "select":
		value, ok := raw.(string)
		if !ok {
			return fmt.Errorf("variables[%d].default 必须是字符串", index)
		}
		if enumValues, ok := variable["enum"].([]interface{}); ok && len(enumValues) > 0 {
			for _, enumValue := range enumValues {
				if enumString, ok := enumValue.(string); ok && enumString == value {
					return nil
				}
			}
			return fmt.Errorf("variables[%d].default 必须是 enum 中的值", index)
		}
	case "number":
		if !isJSONNumber(raw) {
			return fmt.Errorf("variables[%d].default 必须是数字", index)
		}
	case "integer":
		if !isJSONInteger(raw) {
			return fmt.Errorf("variables[%d].default 必须是整数", index)
		}
	case "boolean", "bool":
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("variables[%d].default 必须是布尔值", index)
		}
	case "list", "array":
		if _, ok := raw.([]interface{}); !ok {
			return fmt.Errorf("variables[%d].default 必须是数组", index)
		}
	case "object", "dict":
		if _, ok := raw.(map[string]interface{}); !ok {
			return fmt.Errorf("variables[%d].default 必须是对象", index)
		}
	}
	return nil
}

func validateVariableBounds(variable map[string]interface{}, variableType string, index int) error {
	for _, field := range []string{"min", "max"} {
		raw, exists := variable[field]
		if !exists || raw == nil {
			continue
		}
		if variableType != "number" && variableType != "integer" {
			return fmt.Errorf("variables[%d].%s 仅适用于 number/integer 类型", index, field)
		}
		if !isJSONNumber(raw) {
			return fmt.Errorf("variables[%d].%s 必须是数字", index, field)
		}
	}
	return nil
}

func validateVariablePattern(variable map[string]interface{}, variableType string, index int) error {
	raw, exists := variable["pattern"]
	if !exists || raw == nil {
		return nil
	}
	pattern, ok := raw.(string)
	if !ok {
		return fmt.Errorf("variables[%d].pattern 必须是字符串", index)
	}
	if variableType != "string" && variableType != "enum" && variableType != "select" {
		return fmt.Errorf("variables[%d].pattern 仅适用于 string/enum/select 类型", index)
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("variables[%d].pattern 不是合法正则: %w", index, err)
	}
	return nil
}

func isJSONNumber(value interface{}) bool {
	switch value.(type) {
	case int, int64, float64, float32:
		return true
	default:
		return false
	}
}

func isJSONInteger(value interface{}) bool {
	switch typed := value.(type) {
	case int:
		return true
	case int64:
		return true
	case float64:
		return typed == float64(int64(typed))
	case float32:
		return typed == float32(int64(typed))
	default:
		return false
	}
}
