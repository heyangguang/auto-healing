package handler

import "testing"

func TestBuildSearchAllowlistUsesDedicatedRepositoryAndPlaybookPermissions(t *testing.T) {
	allow := buildSearchAllowlist([]string{"plugin:list"})
	if allow["git_repos"] {
		t.Fatal("git_repos should not be allowed by plugin:list alone")
	}
	if allow["playbooks"] {
		t.Fatal("playbooks should not be allowed by plugin:list alone")
	}
	if !allow["hosts"] || !allow["plugins"] {
		t.Fatal("plugin:list should still allow hosts and plugins")
	}

	allow = buildSearchAllowlist([]string{"plugin:list", "repository:list", "playbook:list"})
	if !allow["git_repos"] || !allow["playbooks"] {
		t.Fatal("repository:list and playbook:list should enable git_repos and playbooks")
	}
}
