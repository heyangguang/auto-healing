package httpapi

import "testing"

func TestValidateScheduleCreateRequestRejectsNegativeMaxFailures(t *testing.T) {
	value := -1
	if err := validateScheduleCreateRequest(&CreateScheduleRequest{MaxFailures: &value}); err == nil {
		t.Fatal("expected negative max_failures to be rejected")
	}
}

func TestValidateScheduleUpdateRequestRejectsNegativeMaxFailures(t *testing.T) {
	value := -1
	if err := validateScheduleUpdateRequest(&UpdateScheduleRequest{MaxFailures: &value}); err == nil {
		t.Fatal("expected negative max_failures to be rejected")
	}
}
