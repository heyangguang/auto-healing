#!/bin/bash

fail() {
  echo "❌ $*" >&2
  exit 1
}

require_commands() {
  local cmd
  for cmd in curl jq git mktemp; do
    command -v "$cmd" >/dev/null 2>&1 || fail "缺少命令: $cmd"
  done
}

curl_json() {
  local body_file http_code
  body_file="$(mktemp)"
  http_code="$(curl -sS -o "$body_file" -w "%{http_code}" "$@")"
  if [ "$http_code" -lt 200 ] || [ "$http_code" -ge 300 ]; then
    cat "$body_file" >&2
    rm -f "$body_file"
    fail "HTTP ${http_code} 请求失败"
  fi
  cat "$body_file"
  rm -f "$body_file"
}

ensure_gitea_repo() {
  local body_file http_code
  body_file="$(mktemp)"
  http_code="$(
    curl -sS -u "${GITEA_USERNAME}:${GITEA_PASSWORD}" \
      -o "$body_file" -w "%{http_code}" \
      "${GITEA_BASE_URL}/api/v1/repos/${GITEA_OWNER}/${DEMO_REPO_NAME}"
  )"
  if [ "$http_code" = "200" ]; then
    rm -f "$body_file"
    return
  fi
  if [ "$http_code" != "404" ]; then
    cat "$body_file" >&2
    rm -f "$body_file"
    fail "查询 Gitea 仓库失败"
  fi
  rm -f "$body_file"
  curl_json \
    -u "${GITEA_USERNAME}:${GITEA_PASSWORD}" \
    -X POST "${GITEA_BASE_URL}/api/v1/user/repos" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${DEMO_REPO_NAME}\",\"description\":\"AHS blacklist interception demo repo\",\"private\":false,\"auto_init\":false,\"default_branch\":\"main\"}" >/dev/null
}

push_lab_repo() {
  local encoded_user encoded_pass remote_url work_dir
  encoded_user="$(printf '%s' "${GITEA_USERNAME}" | jq -sRr @uri)"
  encoded_pass="$(printf '%s' "${GITEA_PASSWORD}" | jq -sRr @uri)"
  remote_url="http://${encoded_user}:${encoded_pass}@${GITEA_BASE_URL#http://}/${GITEA_OWNER}/${DEMO_REPO_NAME}.git"
  work_dir="$(mktemp -d)"

  git clone "${remote_url}" "${work_dir}/repo" >/dev/null 2>&1 || fail "克隆 Gitea 演示仓失败"
  find "${work_dir}/repo" -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +
  cp -R "${LAB_DIR}/." "${work_dir}/repo/"
  git -C "${work_dir}/repo" checkout -B main >/dev/null 2>&1
  git -C "${work_dir}/repo" add .

  if [ -z "$(git -C "${work_dir}/repo" status --short)" ]; then
    rm -rf "${work_dir}"
    return
  fi

  git -C "${work_dir}/repo" -c user.name="AHS Demo" -c user.email="ahs-demo@example.com" \
    commit -m "Update blacklist demo playbooks" >/dev/null
  git -C "${work_dir}/repo" push -u origin main >/dev/null
  rm -rf "${work_dir}"
}

wait_run_terminal() {
  local token="$1"
  local tenant_id="$2"
  local run_id="$3"
  local deadline current_status

  deadline=$((SECONDS + RUN_TIMEOUT_SECONDS))
  while [ "${SECONDS}" -lt "${deadline}" ]; do
    current_status="$(
      curl_json \
        "${AHS_BASE_URL}/tenant/execution-runs/${run_id}" \
        -H "Authorization: Bearer ${token}" \
        -H "X-Tenant-ID: ${tenant_id}" |
        jq -r '.data.status'
    )"
    if [ "${current_status}" = "success" ] || [ "${current_status}" = "failed" ] || [ "${current_status}" = "cancelled" ]; then
      printf '%s' "${current_status}"
      return
    fi
    sleep "${POLL_SECONDS}"
  done
  fail "等待执行记录结束超时"
}
