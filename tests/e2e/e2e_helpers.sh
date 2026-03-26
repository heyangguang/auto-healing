#!/bin/bash
set -euo pipefail

json_created_id() {
  local response="$1"
  echo "$response" | jq -r '.data.id // empty'
}

select_playbook_id() {
  local base_url="$1"
  local token="$2"
  local repo_id="$3"
  local response

  response=$(curl -sS "${base_url}/playbooks?page=1&page_size=100&repository_id=${repo_id}" \
    -H "Authorization: Bearer ${token}")
  if ! echo "$response" | jq -e '.code == 0' >/dev/null 2>&1; then
    printf '%s\n' "$response" >&2
    return 1
  fi

  local id
  id=$(echo "$response" | jq -r '.data[]? | select(.status == "ready" or .status == "outdated") | .id' | head -1)
  if [ -z "$id" ]; then
    echo "未找到 ready/outdated Playbook" >&2
    return 1
  fi
  printf '%s' "$id"
}

select_first_ready_playbook() {
  local base_url="$1"
  local token="$2"
  local response

  response=$(curl -sS "${base_url}/playbooks?page=1&page_size=100" \
    -H "Authorization: Bearer ${token}")
  if ! echo "$response" | jq -e '.code == 0' >/dev/null 2>&1; then
    printf '%s\n' "$response" >&2
    return 1
  fi

  local id
  id=$(echo "$response" | jq -r '.data[]? | select(.status == "ready" or .status == "outdated") | .id' | head -1)
  if [ -z "$id" ]; then
    echo "未找到 ready/outdated Playbook" >&2
    return 1
  fi
  printf '%s' "$id"
}
