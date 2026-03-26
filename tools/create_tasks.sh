#!/bin/bash

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
TASK_COUNT="${TASK_COUNT:-100}"

EXECUTORS=("local" "docker")
CATEGORIES=("日志清理" "服务重启" "配置更新" "健康检查" "备份任务" "监控告警" "安全扫描" "性能优化" "数据同步" "系统维护")
ENVS=("生产" "测试" "开发" "预发布" "灾备")

login_response=$(curl -sS -X POST "${API_BASE}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${USERNAME}\",\"password\":\"${PASSWORD}\"}")
TOKEN=$(echo "$login_response" | jq -r '.access_token // empty')

if [ -z "$TOKEN" ]; then
  echo "❌ 登录失败，未获取到 access token" >&2
  echo "$login_response" | jq . >&2 || echo "$login_response" >&2
  exit 1
fi

playbooks_response=$(curl -sS -X GET "${API_BASE}/playbooks?page=1&page_size=100" \
  -H "Authorization: Bearer ${TOKEN}")
if ! echo "$playbooks_response" | jq -e '.code == 0' >/dev/null 2>&1; then
  echo "❌ 获取 Playbook 列表失败" >&2
  echo "$playbooks_response" | jq . >&2 || echo "$playbooks_response" >&2
  exit 1
fi

mapfile -t PLAYBOOKS < <(echo "$playbooks_response" | jq -r '.data[] | select(.status == "ready" or .status == "outdated") | .id')
if [ "${#PLAYBOOKS[@]}" -eq 0 ]; then
  echo "❌ 未找到可用的 ready/outdated Playbook" >&2
  exit 1
fi

echo "开始创建 ${TASK_COUNT} 个任务模板..."

for i in $(seq 1 "${TASK_COUNT}"); do
  playbook_id="${PLAYBOOKS[$((RANDOM % ${#PLAYBOOKS[@]}))]}"
  executor="${EXECUTORS[$((RANDOM % ${#EXECUTORS[@]}))]}"
  category="${CATEGORIES[$((RANDOM % ${#CATEGORIES[@]}))]}"
  env="${ENVS[$((RANDOM % ${#ENVS[@]}))]}"
  name="${env}环境-${category}-任务${i}"
  desc="这是第 ${i} 个测试任务模板，用于 ${env} 环境的 ${category} 操作"
  host="192.168.$((RANDOM % 256)).$((RANDOM % 256))"

  payload=$(cat <<EOF
{
  "name": "${name}",
  "playbook_id": "${playbook_id}",
  "target_hosts": "${host}",
  "executor_type": "${executor}",
  "description": "${desc}"
}
EOF
)

  response=$(curl -sS -X POST "${API_BASE}/execution-tasks" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$payload")
  if ! echo "$response" | jq -e '.code == 0 and .data.id != null' >/dev/null 2>&1; then
    echo "❌ 创建任务失败: ${name}" >&2
    echo "$response" | jq . >&2 || echo "$response" >&2
    exit 1
  fi

  if [ $((i % 10)) -eq 0 ]; then
    echo "已创建 ${i} 个..."
  fi
done

echo "创建完成！"
