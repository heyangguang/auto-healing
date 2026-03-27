package playbook

import (
	"regexp"
	"strings"
)

func inferTypeSmartly(name string, value any) string {
	if value != nil {
		switch typed := value.(type) {
		case bool:
			return "boolean"
		case int, int64, float64:
			return "number"
		case []interface{}:
			return "list"
		case map[string]interface{}:
			return "object"
		case string:
			if inferredType := parseJinja2Default(typed); inferredType != "" {
				return inferredType
			}
		}
	}

	if inferredType := inferTypeByName(name); inferredType != "" {
		return inferredType
	}
	return "string"
}

func parseJinja2Default(expr string) string {
	re := regexp.MustCompile(`default\s*\(\s*([^)]+)\s*\)`)
	matches := re.FindStringSubmatch(expr)
	if len(matches) < 2 {
		return ""
	}

	defaultVal := strings.TrimSpace(matches[1])
	switch {
	case defaultVal == "true" || defaultVal == "false" || defaultVal == "True" || defaultVal == "False":
		return "boolean"
	case matchesNumber(defaultVal):
		return "number"
	case strings.HasPrefix(defaultVal, "["):
		return "list"
	case strings.HasPrefix(defaultVal, "{"):
		return "object"
	default:
		return ""
	}
}

func matchesNumber(value string) bool {
	matched, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, value)
	return matched
}

func inferTypeByName(name string) string {
	nameLower := strings.ToLower(name)
	if matchesBooleanName(nameLower) {
		return "boolean"
	}
	if matchesNumberName(nameLower) {
		return "number"
	}
	if matchesListName(nameLower) {
		return "list"
	}
	return ""
}

func matchesBooleanName(name string) bool {
	for _, pattern := range []string{"enabled", "disabled", "force", "verbose", "debug", "compress", "allow", "require", "skip", "dry_run"} {
		if name == pattern {
			return true
		}
	}
	for _, prefix := range []string{"is_", "has_", "can_", "should_", "enable_", "disable_", "use_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, suffix := range []string{"_enabled", "_disabled", "_flag", "_mode"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func matchesNumberName(name string) bool {
	for _, suffix := range []string{"_threshold", "_count", "_timeout", "_port", "_size", "_limit", "_max", "_min", "_interval", "_retries", "_delay", "_seconds", "_minutes", "_hours", "_days", "_percent", "_percentage", "_rate", "_number"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func matchesListName(name string) bool {
	for _, suffix := range []string{"_hosts", "_dirs", "_files", "_paths", "_list", "_items", "_servers", "_nodes", "_addresses"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
