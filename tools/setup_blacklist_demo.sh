#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/blacklist_demo_lib.sh"

readonly ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
readonly LAB_DIR="${ROOT_DIR}/labs/blacklist-demo-playbooks"
readonly PLAYBOOK_PATH="playbooks/destructive-demo.yml"
readonly AHS_BASE_URL="${AHS_BASE_URL:-http://127.0.0.1:8080/api/v1}"
readonly GITEA_BASE_URL="${GITEA_BASE_URL:-http://127.0.0.1:13000}"
readonly GITEA_USERNAME="${GITEA_USERNAME:-gitadmin}"
readonly GITEA_PASSWORD="${GITEA_PASSWORD:-GitAdmin123!}"
readonly GITEA_OWNER="${GITEA_OWNER:-gitadmin}"
readonly DEMO_REPO_NAME="${DEMO_REPO_NAME:-blacklist-demo-playbooks}"
readonly DEMO_SUFFIX="${DEMO_SUFFIX:-$(date +%Y%m%d%H%M%S)}"
readonly TENANT_PASSWORD="${TENANT_PASSWORD:-Tenant123456!}"
readonly TENANT_USERNAME="${TENANT_USERNAME:-}"
readonly TENANT_LOGIN_PASSWORD="${TENANT_LOGIN_PASSWORD:-}"
readonly TARGET_TENANT_ID="${TARGET_TENANT_ID:-}"
readonly TARGET_TENANT_CODE="${TARGET_TENANT_CODE:-}"
readonly TARGET_HOSTS_OVERRIDE="${TARGET_HOSTS_OVERRIDE:-}"
readonly SECRETS_SOURCE_IDS_JSON="${SECRETS_SOURCE_IDS_JSON:-}"
readonly REAL_CONTEXT_TASK_NAME="${REAL_CONTEXT_TASK_NAME:-故障实验-CPU恢复}"
readonly AHS_ADMIN_USERNAME="${AHS_ADMIN_USERNAME:-admin}"
readonly AHS_ADMIN_PASSWORD="${AHS_ADMIN_PASSWORD:?请设置 AHS_ADMIN_PASSWORD}"
readonly DEMO_TENANT_CODE="blacklist_demo_${DEMO_SUFFIX}"
readonly DEMO_TENANT_NAME="Blacklist Demo ${DEMO_SUFFIX}"
readonly DEMO_USERNAME="blacklistdemo${DEMO_SUFFIX}"
readonly DEMO_EMAIL="${DEMO_USERNAME}@example.com"
readonly DEMO_REPO_TITLE="Blacklist Demo Repo ${DEMO_SUFFIX}"
readonly DEMO_PLAYBOOK_NAME="Blacklist Destructive Demo ${DEMO_SUFFIX}"
readonly DEMO_TASK_NAME="Blacklist Intercept Demo ${DEMO_SUFFIX}"
readonly POLL_SECONDS=2
readonly RUN_TIMEOUT_SECONDS=60

login_ahs() {
  login_user "${AHS_ADMIN_USERNAME}" "${AHS_ADMIN_PASSWORD}"
}

login_user() {
  local username="$1"
  local password="$2"
  curl_json \
    -X POST "${AHS_BASE_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${username}\",\"password\":\"${password}\"}"
}

get_platform_admin_role_id() {
  local token="$1"
  curl_json \
    "${AHS_BASE_URL}/platform/tenant-roles" \
    -H "Authorization: Bearer ${token}" |
    jq -r '.data[] | select(.name == "admin") | .id'
}

create_demo_tenant() {
  local token="$1"
  curl_json \
    -X POST "${AHS_BASE_URL}/platform/tenants" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${DEMO_TENANT_NAME}\",\"code\":\"${DEMO_TENANT_CODE}\",\"description\":\"AHS blacklist interception demo\",\"icon\":\"shield\"}" |
    jq -r '.data.id'
}

register_demo_user() {
  local token="$1"
  local tenant_id="$2"
  local role_id="$3"
  local invitation_token

  invitation_token="$(
    curl_json \
      -X POST "${AHS_BASE_URL}/platform/tenants/${tenant_id}/invitations" \
      -H "Authorization: Bearer ${token}" \
      -H "Content-Type: application/json" \
      -d "{\"email\":\"${DEMO_EMAIL}\",\"role_id\":\"${role_id}\",\"send_email\":false}" |
      jq -r '.data.invitation_url | split("token=")[1]'
  )"

  curl_json \
    -X POST "${AHS_BASE_URL}/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"token\":\"${invitation_token}\",\"username\":\"${DEMO_USERNAME}\",\"password\":\"${TENANT_PASSWORD}\",\"display_name\":\"Blacklist Demo Admin\"}" >/dev/null
}

enable_blacklist_rules() {
  local token="$1"
  local tenant_id="$2"
  local rule_ids payload

  rule_ids="$(
    curl_json \
      "${AHS_BASE_URL}/tenant/command-blacklist?page=1&page_size=100" \
      -H "Authorization: Bearer ${token}" \
      -H "X-Tenant-ID: ${tenant_id}" |
      jq -r '.data[] | select(.is_system and (.name == "删除根目录" or .name == "清空防火墙规则" or .name == "重启命令")) | .id'
  )"
  [ -n "${rule_ids}" ] || fail "未找到黑名单系统规则"
  payload="$(printf '%s\n' "${rule_ids}" | jq -R . | jq -sc '{ids: ., is_active: true}')"
  curl_json \
    -X POST "${AHS_BASE_URL}/tenant/command-blacklist/batch-toggle" \
    -H "Authorization: Bearer ${token}" \
    -H "X-Tenant-ID: ${tenant_id}" \
    -H "Content-Type: application/json" \
    -d "${payload}" >/dev/null
}

create_ahs_repo() {
  local token="$1"
  local tenant_id="$2"
  curl_json \
    -X POST "${AHS_BASE_URL}/tenant/git-repos" \
    -H "Authorization: Bearer ${token}" \
    -H "X-Tenant-ID: ${tenant_id}" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${DEMO_REPO_TITLE}\",\"url\":\"${GITEA_BASE_URL}/${GITEA_OWNER}/${DEMO_REPO_NAME}.git\",\"default_branch\":\"main\",\"auth_type\":\"none\",\"auth_config\":{},\"sync_enabled\":false}" |
    jq -r '.data.id'
}

wait_repo_ready() {
  local token="$1"
  local tenant_id="$2"
  local repo_id="$3"
  local attempt status

  curl_json \
    -X POST "${AHS_BASE_URL}/tenant/git-repos/${repo_id}/sync" \
    -H "Authorization: Bearer ${token}" \
    -H "X-Tenant-ID: ${tenant_id}" >/dev/null

  for attempt in $(seq 1 20); do
    status="$(
      curl_json \
        "${AHS_BASE_URL}/tenant/git-repos/${repo_id}" \
        -H "Authorization: Bearer ${token}" \
        -H "X-Tenant-ID: ${tenant_id}" |
        jq -r '.data.status'
    )"
    if [ "${status}" = "ready" ]; then
      return
    fi
    sleep "${POLL_SECONDS}"
  done
  fail "等待 AHS 仓库同步 ready 超时"
}

resolve_target_tenant_id() {
  local login_json="$1"
  if [ -n "${TARGET_TENANT_ID}" ]; then
    printf '%s' "${TARGET_TENANT_ID}"
    return
  fi
  if [ -n "${TARGET_TENANT_CODE}" ]; then
    printf '%s' "${login_json}" | jq -r --arg code "${TARGET_TENANT_CODE}" '.data.tenants[]? | select(.code == $code) | .id' | sed -n '1p'
    return
  fi
  printf '%s' "${login_json}" | jq -r '.data.current_tenant_id // .current_tenant_id'
}

resolve_execution_context() {
  local token="$1"
  local tenant_id="$2"
  local task_context task_filter task_payload
  if [ -n "${TARGET_HOSTS_OVERRIDE}" ] && [ -n "${SECRETS_SOURCE_IDS_JSON}" ]; then
    jq -cn \
      --arg target_hosts "${TARGET_HOSTS_OVERRIDE}" \
      --argjson secrets_source_ids "${SECRETS_SOURCE_IDS_JSON}" \
      '{target_hosts: $target_hosts, secrets_source_ids: $secrets_source_ids}'
    return
  fi

  task_payload="$(curl_json "${AHS_BASE_URL}/tenant/execution-tasks?page=1&page_size=100" -H "Authorization: Bearer ${token}" -H "X-Tenant-ID: ${tenant_id}")"
  task_filter='.data[] | select(.target_hosts != "" and .target_hosts != "localhost" and (.secrets_source_ids | length) > 0)'
  task_context="$(printf '%s' "${task_payload}" | jq -c --arg name "${REAL_CONTEXT_TASK_NAME}" "${task_filter} | select(.name == \$name) | {target_hosts, secrets_source_ids}" | sed -n '1p')"
  if [ -z "${task_context}" ]; then
    task_context="$(printf '%s' "${task_payload}" | jq -c "${task_filter} | {target_hosts, secrets_source_ids}" | sed -n '1p')"
  fi
  [ -n "${task_context}" ] || fail "未找到可复用的真实任务上下文，请显式传入 TARGET_HOSTS_OVERRIDE / SECRETS_SOURCE_IDS_JSON"
  printf '%s' "${task_context}"
}

create_playbook_and_task() {
  local token="$1"
  local tenant_id="$2"
  local repo_id="$3"
  local target_hosts="$4"
  local secrets_source_ids_json="$5"
  local playbook_id task_id task_payload
  playbook_id="$(
    curl_json \
      -X POST "${AHS_BASE_URL}/tenant/playbooks" \
      -H "Authorization: Bearer ${token}" \
      -H "X-Tenant-ID: ${tenant_id}" \
      -H "Content-Type: application/json" \
      -d "{\"repository_id\":\"${repo_id}\",\"name\":\"${DEMO_PLAYBOOK_NAME}\",\"file_path\":\"${PLAYBOOK_PATH}\",\"description\":\"AHS blacklist destructive command demo\",\"config_mode\":\"auto\"}" |
      jq -r '.data.id'
  )"
  curl_json -X POST "${AHS_BASE_URL}/tenant/playbooks/${playbook_id}/scan" -H "Authorization: Bearer ${token}" -H "X-Tenant-ID: ${tenant_id}" >/dev/null
  curl_json -X POST "${AHS_BASE_URL}/tenant/playbooks/${playbook_id}/ready" -H "Authorization: Bearer ${token}" -H "X-Tenant-ID: ${tenant_id}" >/dev/null
  task_payload="$(jq -cn \
    --arg name "${DEMO_TASK_NAME}" \
    --arg playbook_id "${playbook_id}" \
    --arg target_hosts "${target_hosts}" \
    --arg executor_type "local" \
    --arg description "Expect security interception before execution with real target host and secrets context" \
    --argjson secrets_source_ids "${secrets_source_ids_json}" \
    '{name: $name, playbook_id: $playbook_id, target_hosts: $target_hosts, executor_type: $executor_type, description: $description, secrets_source_ids: $secrets_source_ids}')"
  task_id="$(
    curl_json \
      -X POST "${AHS_BASE_URL}/tenant/execution-tasks" \
      -H "Authorization: Bearer ${token}" \
      -H "X-Tenant-ID: ${tenant_id}" \
      -H "Content-Type: application/json" \
      -d "${task_payload}" |
      jq -r '.data.id'
  )"
  printf '%s %s\n' "${playbook_id}" "${task_id}"
}

main() {
  local platform_token admin_role_id tenant_id tenant_login tenant_token current_tenant_id
  local repo_id playbook_id task_id run_id run_status run_logs context_json target_hosts secrets_source_ids_json
  require_commands
  ensure_gitea_repo
  push_lab_repo
  if [ -n "${TENANT_USERNAME}" ] && [ -n "${TENANT_LOGIN_PASSWORD}" ]; then
    tenant_login="$(login_user "${TENANT_USERNAME}" "${TENANT_LOGIN_PASSWORD}")"
    tenant_token="$(printf '%s' "${tenant_login}" | jq -r '.data.access_token // .access_token')"
    tenant_id="$(resolve_target_tenant_id "${tenant_login}")"
    [ -n "${tenant_id}" ] || fail "无法解析目标租户 ID"
  else
    platform_token="$(login_ahs)"
    admin_role_id="$(get_platform_admin_role_id "${platform_token}")"
    [ -n "${admin_role_id}" ] || fail "未获取到平台租户 admin 角色"
    tenant_id="$(create_demo_tenant "${platform_token}")"
    register_demo_user "${platform_token}" "${tenant_id}" "${admin_role_id}"
    tenant_login="$(login_user "${DEMO_USERNAME}" "${TENANT_PASSWORD}")"
    tenant_token="$(printf '%s' "${tenant_login}" | jq -r '.data.access_token // .access_token')"
    current_tenant_id="$(printf '%s' "${tenant_login}" | jq -r '.data.current_tenant_id // .current_tenant_id')"
    [ "${current_tenant_id}" = "${tenant_id}" ] || fail "当前租户与新建租户不一致"
  fi
  enable_blacklist_rules "${tenant_token}" "${tenant_id}"
  context_json="$(resolve_execution_context "${tenant_token}" "${tenant_id}")"
  target_hosts="$(printf '%s' "${context_json}" | jq -r '.target_hosts')"
  secrets_source_ids_json="$(printf '%s' "${context_json}" | jq -c '.secrets_source_ids')"
  repo_id="$(create_ahs_repo "${tenant_token}" "${tenant_id}")"
  wait_repo_ready "${tenant_token}" "${tenant_id}" "${repo_id}"
  read -r playbook_id task_id <<<"$(create_playbook_and_task "${tenant_token}" "${tenant_id}" "${repo_id}" "${target_hosts}" "${secrets_source_ids_json}")"
  run_id="$(
    curl_json \
      -X POST "${AHS_BASE_URL}/tenant/execution-tasks/${task_id}/execute" \
      -H "Authorization: Bearer ${tenant_token}" \
      -H "X-Tenant-ID: ${tenant_id}" \
      -H "Content-Type: application/json" \
      -d "{\"triggered_by\":\"blacklist-demo-script\"}" |
      jq -r '.data.id'
  )"
  run_status="$(wait_run_terminal "${tenant_token}" "${tenant_id}" "${run_id}")"
  run_logs="$(
    curl_json \
      "${AHS_BASE_URL}/tenant/execution-runs/${run_id}/logs" \
      -H "Authorization: Bearer ${tenant_token}" \
      -H "X-Tenant-ID: ${tenant_id}"
  )"
  printf '%s\n' "${run_logs}" | jq -e '.data[] | select(.stage == "security" and (.message | contains("执行已拦截")))' >/dev/null ||
    fail "执行记录中未找到执行已拦截日志"
  [ "${run_status}" = "failed" ] || fail "预期执行被拦截失败，实际状态: ${run_status}"
  jq -n \
    --arg tenant_id "${tenant_id}" \
    --arg tenant_username "${DEMO_USERNAME}" \
    --arg gitea_repo "${GITEA_BASE_URL}/${GITEA_OWNER}/${DEMO_REPO_NAME}.git" \
    --arg ahs_repo_id "${repo_id}" \
    --arg playbook_id "${playbook_id}" \
    --arg task_id "${task_id}" \
    --arg run_id "${run_id}" \
    --arg run_status "${run_status}" \
    --arg target_hosts "${target_hosts}" \
    --argjson secrets_source_ids "${secrets_source_ids_json}" \
    '{
      tenant_id: $tenant_id,
      tenant_username: $tenant_username,
      gitea_repo: $gitea_repo,
      ahs_repo_id: $ahs_repo_id,
      playbook_id: $playbook_id,
      task_id: $task_id,
      run_id: $run_id,
      run_status: $run_status,
      target_hosts: $target_hosts,
      secrets_source_ids: $secrets_source_ids,
      result: "security_intercepted"
    }'
}

main "$@"
