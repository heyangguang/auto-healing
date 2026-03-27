#!/usr/bin/env bash
set -euo pipefail

EXPECTED_MODULE_HEADER='module_id,label,module_kind,module_note,worktree_suffix,branch_suffix,paths,shared_touchpoints,focus'

repo_root=__REPO_ROOT__
session_name=__SESSION_NAME__
parent=__PARENT_DIR__
repo_name=__REPO_NAME__
session_base_ref=__SESSION_BASE_REF__
session_from_detached_head=__SESSION_FROM_DETACHED_HEAD__
modules_file="$(cd "$(dirname "$0")" && pwd)/modules.csv"
validator_file="$(cd "$(dirname "$0")" && pwd)/module_csv_validator.awk"

usage() {
	cat <<'USAGE'
Usage:
  ./create_worktrees.sh [module_id...]
  ./create_worktrees.sh --list

Notes:
  - Without module arguments, all modules are created
  - With module arguments, only the selected modules are created
  - REVIEW_BASE_BRANCH overrides the session's default base ref
USAGE
}

branch_name() {
	local branch_suffix="$1"
	printf 'fix/%s/%s\n' "$session_name" "$branch_suffix"
}

worktree_dir() {
	local worktree_suffix="$1"
	printf '%s/%s-%s-%s\n' "$parent" "$repo_name" "$session_name" "$worktree_suffix"
}

validate_modules_csv() {
	local modules="$1"
	[[ -r "$validator_file" ]] || {
		printf 'validator file not found or unreadable: %s\n' "$validator_file" >&2
		exit 1
	}
	awk -F, -v expected="$EXPECTED_MODULE_HEADER" -f "$validator_file" "$modules"
}

module_requested() {
	local module_id="$1"
	if [[ ${#selected_modules[@]} -eq 0 ]]; then
		return 0
	fi
	local wanted
	for wanted in "${selected_modules[@]}"; do
		if [[ "$wanted" == "$module_id" ]]; then
			return 0
		fi
	done
	return 1
}

validate_requested_modules() {
	if [[ ${#selected_modules[@]} -eq 0 ]]; then
		return 0
	fi

	while IFS=, read -r module_id label module_kind module_note worktree_suffix branch_suffix paths shared_touchpoints focus; do
		if module_requested "$module_id"; then
			found_modules["$module_id"]=1
		fi
	done < <(tail -n +2 "$modules_file")

	local missing=0
	local wanted
	for wanted in "${selected_modules[@]}"; do
		if [[ -z "${found_modules[$wanted]:-}" ]]; then
			printf 'unknown module_id: %s\n' "$wanted" >&2
			missing=1
		fi
	done
	if [[ "$missing" == '1' ]]; then
		exit 1
	fi
}

list_modules() {
	printf 'module_id,label,module_kind,module_note,branch_name,worktree_dir\n'
	while IFS=, read -r module_id label module_kind module_note worktree_suffix branch_suffix paths shared_touchpoints focus; do
		printf '%s,%s,%s,%s,%s,%s\n' \
			"$module_id" \
			"$label" \
			"$module_kind" \
			"$module_note" \
			"$(branch_name "$branch_suffix")" \
			"$(worktree_dir "$worktree_suffix")"
	done < <(tail -n +2 "$modules_file")
}

create_worktrees() {
	while IFS=, read -r module_id label module_kind module_note worktree_suffix branch_suffix paths shared_touchpoints focus; do
		if ! module_requested "$module_id"; then
			continue
		fi
		local branch dir
		branch="$(branch_name "$branch_suffix")"
		dir="$(worktree_dir "$worktree_suffix")"
		if [[ -d "$dir" ]]; then
			printf 'skip existing worktree: %s\n' "$dir"
			continue
		fi
		if git show-ref --verify --quiet "refs/heads/$branch"; then
			git worktree add "$dir" "$branch"
			continue
		fi
		git worktree add -b "$branch" "$dir" "$base_ref"
	done < <(tail -n +2 "$modules_file")
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
	usage
	exit 0
fi

validate_modules_csv "$modules_file"

if [[ "${1:-}" == "--list" ]]; then
	list_modules
	exit 0
fi

cd "$repo_root"

base_ref="${REVIEW_BASE_BRANCH:-$session_base_ref}"
[[ -n "$base_ref" ]] || {
	if [[ "$session_from_detached_head" == 'true' ]]; then
		printf 'REVIEW_BASE_BRANCH is required because this session was generated from detached HEAD\n' >&2
	else
		printf 'REVIEW_BASE_BRANCH is required\n' >&2
	fi
	exit 1
}
git rev-parse --verify "$base_ref" >/dev/null 2>&1 || {
	printf 'base ref not found: %s\n' "$base_ref" >&2
	exit 1
}

selected_modules=("$@")
declare -A found_modules=()
validate_requested_modules
create_worktrees
