#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/review/start_tmux_parallel_review.sh <session_name> [--detach]

Notes:
  - Run setup_parallel_review.sh first
  - This script creates one tmux session with one control window and one module window per row
  - If a matching worktree exists, the module window starts there
  - By default it attaches immediately; use --detach to only create the session
EOF
}

repo_root() {
  git rev-parse --show-toplevel
}

session_dir() {
  local root
  root="$(repo_root)"
  printf '%s/.parallel-review/%s\n' "$root" "$1"
}

tmux_session_name() {
  printf 'review-%s\n' "$1"
}

branch_name() {
  local session_name="$1"
  local branch_suffix="$2"
  printf 'fix/%s/%s\n' "$session_name" "$branch_suffix"
}

attach_or_switch() {
  local tmux_name="$1"
  if [[ -n "${TMUX:-}" ]]; then
    tmux switch-client -t "$tmux_name"
    return
  fi
  exec tmux attach -t "$tmux_name"
}

worktree_dir() {
  local module_suffix="$1"
  local root repo_name parent session_name="$2"
  root="$(repo_root)"
  repo_name="$(basename "$root")"
  parent="$(dirname "$root")"
  printf '%s/%s-%s-%s\n' "$parent" "$repo_name" "$session_name" "$module_suffix"
}

window_dir() {
  local suffix="$1"
  local session_name="$2"
  local wt
  wt="$(worktree_dir "$suffix" "$session_name")"
  if [[ -d "$wt" ]]; then
    printf '%s\n' "$wt"
    return
  fi
  repo_root
}

window_cmd() {
  local dir="$1"
  local prompt_file="$2"
  local status_csv="$3"
  local findings_file="$4"
  local branch_name="$5"
  local expected_dir="$6"
  local shared_touchpoints="$7"
  printf 'cd %q && clear && printf %q && exec "${SHELL:-bash}"' \
    "$dir" \
    "Branch: ${branch_name}\nWorktree: ${expected_dir}\nShared touchpoints: ${shared_touchpoints}\nPrompt: ${prompt_file}\nStatus: ${status_csv}\nFindings: ${findings_file}\n\n"
}

main() {
  [[ $# -ge 1 && $# -le 2 ]] || {
    usage
    exit 1
  }
  command -v tmux >/dev/null 2>&1 || {
    printf 'tmux is required\n' >&2
    exit 1
  }

  local review_name detach dir modules status_csv repair_plan tmux_name control_cmd
  review_name="$1"
  detach="0"
  if [[ $# -eq 2 ]]; then
    [[ "$2" == "--detach" ]] || {
      usage
      exit 1
    }
    detach="1"
  fi
  dir="$(session_dir "$review_name")"
  [[ -d "$dir" ]] || {
    printf 'review session not found: %s\n' "$dir" >&2
    exit 1
  }

  modules="$dir/modules.csv"
  status_csv="$dir/review_status.csv"
  repair_plan="$dir/repair_plan.csv"
  tmux_name="$(tmux_session_name "$review_name")"

  if tmux has-session -t "$tmux_name" >/dev/null 2>&1; then
    if [[ "$detach" == "1" ]]; then
      printf 'tmux session already exists: %s\n' "$tmux_name"
      printf 'attach with: tmux attach -t %s\n' "$tmux_name"
      exit 0
    fi
    attach_or_switch "$tmux_name"
  fi

  printf -v control_cmd 'cd %q && clear && printf %q && exec "${SHELL:-bash}"' \
    "$(repo_root)" \
    "Control window\nSession: ${review_name}\nStatus CSV: ${status_csv}\nRepair plan: ${repair_plan}\n\n"

  tmux new-session -d -s "$tmux_name" -n control
  tmux send-keys -t "$tmux_name:control" "$control_cmd" C-m

  tail -n +2 "$modules" | while IFS=, read -r module_id label worktree_suffix branch_suffix paths shared_touchpoints focus; do
    local_dir="$(window_dir "$worktree_suffix" "$review_name")"
    prompt_file="$dir/prompts/${module_id}.md"
    findings_file="$dir/findings/${module_id}.md"
    expected_dir="$(worktree_dir "$worktree_suffix" "$review_name")"
    module_branch="$(branch_name "$review_name" "$branch_suffix")"
    tmux new-window -t "$tmux_name" -n "$module_id"
    tmux send-keys -t "$tmux_name:$module_id" "$(window_cmd "$local_dir" "$prompt_file" "$status_csv" "$findings_file" "$module_branch" "$expected_dir" "$shared_touchpoints")" C-m
  done

  tmux select-window -t "$tmux_name:control"
  if [[ "$detach" == "1" ]]; then
    printf 'tmux session created: %s\n' "$tmux_name"
    printf 'attach with: tmux attach -t %s\n' "$tmux_name"
    exit 0
  fi

  attach_or_switch "$tmux_name"
}

main "$@"
