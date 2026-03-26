package middleware

import "testing"

func TestRedactRawQueryFallsBackForMalformedQuery(t *testing.T) {
	raw := "token=secret&broken=%zz&refresh_token=refresh"

	got := redactRawQuery(raw)

	if got != "token=REDACTED&broken=%zz&refresh_token=REDACTED" {
		t.Fatalf("redactRawQuery() = %q", got)
	}
}
