package service

import "testing"

func TestAllDictionarySeedsHaveUniqueTypeAndKey(t *testing.T) {
	seen := make(map[string]bool, len(AllDictionarySeeds))
	for _, item := range AllDictionarySeeds {
		key := item.DictType + "::" + item.DictKey
		if seen[key] {
			t.Fatalf("duplicate dictionary seed: %s", key)
		}
		seen[key] = true
	}
}

func TestCriticalDictionaryTypesExist(t *testing.T) {
	required := []string{
		"user_status",
		"tenant_status",
		"role_scope",
		"invitation_status",
		"impersonation_status",
		"approval_status",
		"instance_status",
		"executor_type",
		"run_status",
		"schedule_type",
		"schedule_status",
		"plugin_type",
		"plugin_status",
		"incident_severity",
		"incident_status",
		"incident_category",
		"healing_status",
		"cmdb_type",
		"cmdb_status",
		"cmdb_environment",
		"notification_channel_type",
		"notification_event_type",
		"notification_format",
		"notification_log_status",
		"site_message_category",
		"secrets_source_type",
		"secrets_auth_type",
		"secrets_source_status",
		"git_auth_type",
		"git_repo_status",
		"playbook_status",
		"playbook_config_mode",
		"playbook_scan_trigger_type",
		"execution_triggered_by",
	}
	index := make(map[string]int)
	for _, item := range AllDictionarySeeds {
		if item.IsActive {
			index[item.DictType]++
		}
	}
	for _, dictType := range required {
		if index[dictType] == 0 {
			t.Fatalf("missing active dictionary seeds for %s", dictType)
		}
	}
}

func TestLegacyDictionaryKeysStayInactive(t *testing.T) {
	assertSeedInactive(t, "user_status", "disabled")
	assertSeedInactive(t, "git_repo_status", "synced")
}

func assertSeedInactive(t *testing.T, dictType, dictKey string) {
	t.Helper()
	for _, item := range AllDictionarySeeds {
		if item.DictType == dictType && item.DictKey == dictKey {
			if item.IsActive {
				t.Fatalf("%s.%s should be inactive legacy key", dictType, dictKey)
			}
			return
		}
	}
	t.Fatalf("dictionary key not found: %s.%s", dictType, dictKey)
}
