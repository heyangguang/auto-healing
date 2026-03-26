package plugin

import "testing"

func TestToStringHandlesNumericValues(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{name: "float64 integer", input: float64(123), want: "123"},
		{name: "float64 decimal", input: float64(12.5), want: "12.5"},
		{name: "int", input: 7, want: "7"},
		{name: "bool", input: true, want: "true"},
	}

	for _, tt := range tests {
		if got := toString(tt.input); got != tt.want {
			t.Fatalf("%s: toString(%v) = %q, want %q", tt.name, tt.input, got, tt.want)
		}
	}
}

func TestApplyFilterWithReasonSupportsNumericEquals(t *testing.T) {
	t.Helper()

	filter := &FilterCondition{
		Field:    "severity_level",
		Operator: "equals",
		Value:    2,
	}
	data := map[string]interface{}{
		"severity_level": float64(2),
	}

	matched, reason := ApplyFilterWithReason(filter, data)
	if !matched {
		t.Fatalf("expected numeric filter to match, reason=%q", reason)
	}
}
