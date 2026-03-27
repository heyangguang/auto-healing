package repository

import (
	"testing"

	"gorm.io/gorm"
)

func createIntegrationsRepositorySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecRepositorySQL(t, db, `
		CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			default_branch TEXT,
			auth_type TEXT,
			auth_config TEXT,
			local_path TEXT,
			branches TEXT,
			last_sync_at DATETIME,
			last_commit_id TEXT,
			status TEXT,
			error_message TEXT,
			sync_enabled BOOLEAN,
			sync_interval TEXT,
			next_sync_at DATETIME,
			max_failures INTEGER,
			consecutive_failures INTEGER,
			pause_reason TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE playbooks (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			repository_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			file_path TEXT NOT NULL,
			variables TEXT,
			scanned_variables TEXT,
			last_scanned_at DATETIME,
			config_mode TEXT,
			status TEXT,
			tags TEXT,
			default_extra_vars TEXT,
			default_timeout_minutes INTEGER,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE git_sync_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			repository_id TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL,
			commit_id TEXT,
			branch TEXT,
			duration_ms INTEGER,
			error_message TEXT,
			created_at DATETIME
		);
		CREATE TABLE playbook_scan_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			playbook_id TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			files_scanned INTEGER,
			variables_found INTEGER,
			new_count INTEGER,
			removed_count INTEGER,
			details TEXT,
			created_at DATETIME
		);
		CREATE TABLE plugins (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT,
			version TEXT NOT NULL,
			config TEXT NOT NULL,
			field_mapping TEXT,
			sync_filter TEXT,
			sync_enabled BOOLEAN,
			sync_interval_minutes INTEGER,
			last_sync_at DATETIME,
			next_sync_at DATETIME,
			max_failures INTEGER,
			consecutive_failures INTEGER,
			pause_reason TEXT,
			status TEXT,
			error_message TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE plugin_sync_logs (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			plugin_id TEXT NOT NULL,
			sync_type TEXT NOT NULL,
			status TEXT NOT NULL,
			records_fetched INTEGER,
			records_filtered INTEGER,
			records_processed INTEGER,
			records_new INTEGER,
			records_updated INTEGER,
			records_failed INTEGER,
			started_at DATETIME,
			completed_at DATETIME,
			error_message TEXT,
			details TEXT
		);
	`)
}
