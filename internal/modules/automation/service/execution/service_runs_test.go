package execution

import "testing"

func TestNormalizeTriggeredByNormalizesKnownPrefixes(t *testing.T) {
	cases := map[string]string{
		"":                                 "manual",
		"manual":                           "manual",
		"manual:test-notification-success": "manual",
		"scheduler:cron:nightly":           "scheduler:cron",
		"scheduler:once:adhoc":             "scheduler:once",
		"healing:flow-node":                "healing",
		"workflow":                         "healing",
		"custom-source":                    "custom-source",
	}

	for input, want := range cases {
		if got := NormalizeTriggeredBy(input); got != want {
			t.Fatalf("NormalizeTriggeredBy(%q) = %q, want %q", input, got, want)
		}
	}
}
