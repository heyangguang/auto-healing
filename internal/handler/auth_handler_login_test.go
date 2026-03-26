package handler

import (
	"errors"
	"testing"

	authService "github.com/company/auto-healing/internal/service/auth"
)

func TestIsLoginUnauthorizedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "invalid credentials", err: authService.ErrInvalidCredentials, want: true},
		{name: "locked", err: authService.ErrUserLocked, want: true},
		{name: "inactive", err: authService.ErrUserInactive, want: true},
		{name: "generic", err: errors.New("db down"), want: false},
	}

	for _, tc := range cases {
		if got := isLoginUnauthorizedError(tc.err); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
