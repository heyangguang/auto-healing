#!/usr/bin/env bash
set -euo pipefail

PARALLEL_REVIEW_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
	cat <<'EOF'
Usage:
  scripts/review/setup_parallel_review.sh [session_name]
  scripts/review/setup_parallel_review.sh --help

Optional env:
  REVIEW_ROOT        Override output root. Default: .parallel-review
  REVIEW_MODULES     Override module CSV. Default: scripts/review/backend_modules.csv

Session rules:
  - Default session_name: backend-review-YYYYMMDD-HHMMSS
  - Allowed characters: A-Z a-z 0-9 . _ -
  - session_name must not start with '-'

Module CSV rules:
  - Header must match the built-in 9-column schema exactly
  - Quoted CSV fields are not supported
  - Fields must not contain literal commas

Required commands:
  - bash
  - git
  - awk
  - python3 or python

What it does:
  1. Creates a review session directory
  2. Generates review_status.csv and repair_plan.csv
  3. Generates one prompt per module
  4. Generates branch-aware create_worktrees.sh with a fixed default base ref
EOF
}

load_parallel_review_libs() {
	local common_sh writer_sh
	common_sh="$PARALLEL_REVIEW_SCRIPT_DIR/parallel_review_common.sh"
	writer_sh="$PARALLEL_REVIEW_SCRIPT_DIR/parallel_review_writer.sh"
	[[ -r "$common_sh" ]] || {
		printf 'required library not found or unreadable: %s\n' "$common_sh" >&2
		exit 1
	}
	[[ -r "$writer_sh" ]] || {
		printf 'required library not found or unreadable: %s\n' "$writer_sh" >&2
		exit 1
	}
	# shellcheck source=/dev/null
	source "$common_sh"
	# shellcheck source=/dev/null
	source "$writer_sh"
}

main() {
	if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
		usage
		exit 0
	fi

	load_parallel_review_libs
	ensure_inputs

	local session modules output_root session_dir default_base_ref
	session="$(session_name "$@")"
	validate_session_name "$session"
	modules="$(module_file)"
	output_root="$(review_root)"
	session_dir="${output_root}/${session}"
	default_base_ref="$(current_branch)"

	mkdir -p "$output_root"
	mkdir "$session_dir" || {
		printf 'session directory already exists: %s\n' "$session_dir" >&2
		exit 1
	}
	mkdir "$session_dir/prompts" "$session_dir/findings"

	cp "$modules" "$session_dir/modules.csv"
	write_status_csv "$session_dir" "$session" "$modules"
	write_repair_plan_csv "$session_dir" "$session" "$modules"
	write_module_csv_validator "$session_dir"
	write_worktree_helper "$session_dir/create_worktrees.sh" "$session" "$default_base_ref"
	write_readme "$session_dir" "$session" "$default_base_ref"
	populate_module_outputs "$session_dir" "$session" "$modules"
	print_next_steps "$session_dir"
}

main "$@"
