package healing

import "strings"

func conversionFunctions() map[string]interface{} {
	return map[string]interface{}{
		"toInt":    func(v interface{}) int { return toIntValue(v) },
		"toFloat":  func(v interface{}) float64 { return toFloatValue(v) },
		"toString": func(v interface{}) string { return toStringValue(v) },
	}
}

func stringFunctions() map[string]interface{} {
	return map[string]interface{}{
		"upper":       func(s string) string { return strings.ToUpper(s) },
		"lower":       func(s string) string { return strings.ToLower(s) },
		"replace":     func(s, old, new string) string { return strings.ReplaceAll(s, old, new) },
		"trim":        func(s string) string { return strings.TrimSpace(s) },
		"split":       func(s, sep string) []string { return strings.Split(s, sep) },
		"strContains": func(s, substr string) bool { return strings.Contains(s, substr) },
		"hasPrefix":   func(s, prefix string) bool { return strings.HasPrefix(s, prefix) },
		"hasSuffix":   func(s, suffix string) bool { return strings.HasSuffix(s, suffix) },
	}
}

func mathFunctions() map[string]interface{} {
	return map[string]interface{}{
		"abs": func(n interface{}) float64 {
			f := toFloatValue(n)
			if f < 0 {
				return -f
			}
			return f
		},
		"max": func(a, b interface{}) float64 {
			fa, fb := toFloatValue(a), toFloatValue(b)
			if fa > fb {
				return fa
			}
			return fb
		},
		"min": func(a, b interface{}) float64 {
			fa, fb := toFloatValue(a), toFloatValue(b)
			if fa < fb {
				return fa
			}
			return fb
		},
	}
}
