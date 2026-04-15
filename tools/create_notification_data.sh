#!/bin/bash

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

post_json() {
  local path="$1"
  local payload="$2"
  local response

  response=$(curl -sS -X POST "${API_BASE}${path}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$payload")

  if ! echo "$response" | jq -e '.code == 0' >/dev/null 2>&1; then
    echo "❌ 请求失败: POST ${path}" >&2
    echo "$response" | jq . >&2 || echo "$response" >&2
    return 1
  fi

  printf '%s\n' "$response"
}

print_result() {
  echo "$1" | jq -r '.data.id'
}

login_response=$(curl -sS -X POST "${API_BASE}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${USERNAME}\",\"password\":\"${PASSWORD}\"}")
TOKEN=$(echo "$login_response" | jq -r '.access_token // empty')

if [ -z "$TOKEN" ]; then
  echo "❌ 登录失败，未获取到 access token" >&2
  echo "$login_response" | jq . >&2 || echo "$login_response" >&2
  exit 1
fi

echo "=== 创建通知渠道 (15个) ==="

for i in {1..5}; do
  payload=$(cat <<EOF
{
  "name": "Webhook-运维告警-${i}",
  "type": "webhook",
  "description": "运维告警 Webhook 渠道 ${i}",
  "config": {
    "url": "http://localhost:5000/webhook/alert-${i}",
    "method": "POST"
  },
  "recipients": [],
  "is_default": $([ "$i" -eq 1 ] && echo "true" || echo "false")
}
EOF
)
  print_result "$(post_json "/channels" "$payload")"
done

for i in {1..5}; do
  payload=$(cat <<EOF
{
  "name": "邮件-团队${i}",
  "type": "email",
  "description": "团队${i}邮件通知渠道",
  "config": {
    "smtp_host": "smtp.example.com",
    "smtp_port": 587,
    "username": "noreply@company.com",
    "password": "password",
    "from_address": "noreply@company.com",
    "use_tls": true
  },
  "recipients": ["team${i}@company.com"],
  "is_default": false
}
EOF
)
  print_result "$(post_json "/channels" "$payload")"
done

for i in {1..5}; do
  payload=$(cat <<EOF
{
  "name": "钉钉-群组${i}",
  "type": "dingtalk",
  "description": "钉钉机器人群组${i}",
  "config": {
    "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=token${i}",
    "secret": "SEC_secret_${i}"
  },
  "recipients": [],
  "is_default": false
}
EOF
)
  print_result "$(post_json "/channels" "$payload")"
done

echo ""
echo "=== 创建通知模板 (15个) ==="

for i in {1..3}; do
  payload=$(cat <<EOF
{
  "name": "Webhook-执行结果通知-${i}",
  "description": "任务执行完成后的 Webhook 通知模板",
  "event_type": "execution_result",
  "supported_channels": ["webhook"],
  "subject_template": "【{{execution_status}}】任务执行通知",
  "body_template": "# {{execution_status_emoji}} 任务执行{{execution_status}}\n\n时间: {{timestamp}}\n耗时: {{execution_duration_ms}} ms",
  "format": "markdown"
}
EOF
)
  print_result "$(post_json "/templates" "$payload")"
done

for i in {1..3}; do
  payload=$(cat <<EOF
{
  "name": "邮件-执行结果通知-${i}",
  "description": "任务执行完成后的邮件通知模板",
  "event_type": "execution_result",
  "supported_channels": ["email"],
  "subject_template": "【{{execution_status}}】任务执行通知",
  "body_template": "<h1>任务执行{{execution_status}}</h1><p>时间: {{timestamp}}</p><p>耗时: {{execution_duration_ms}} ms</p>",
  "format": "html"
}
EOF
)
  print_result "$(post_json "/templates" "$payload")"
done

for i in {1..3}; do
  payload=$(cat <<EOF
{
  "name": "多渠道-执行结果通知-${i}",
  "description": "支持多渠道的任务执行通知模板",
  "event_type": "execution_result",
  "supported_channels": ["webhook", "email"],
  "subject_template": "任务执行报告-${i}",
  "body_template": "执行状态: {{execution_status}}\n主机: {{target_hosts}}\n时间: {{timestamp}}",
  "format": "text"
}
EOF
)
  print_result "$(post_json "/templates" "$payload")"
done

for i in {1..3}; do
  payload=$(cat <<EOF
{
  "name": "钉钉-告警通知-${i}",
  "description": "钉钉机器人告警通知模板",
  "event_type": "flow_result",
  "supported_channels": ["dingtalk"],
  "subject_template": "告警通知",
  "body_template": "### 告警通知\n- 状态: {{execution_status}}\n- 时间: {{timestamp}}",
  "format": "markdown"
}
EOF
)
  print_result "$(post_json "/templates" "$payload")"
done

for i in {1..3}; do
  payload=$(cat <<EOF
{
  "name": "全渠道-通用通知-${i}",
  "description": "支持所有渠道类型的通用通知模板",
  "event_type": "flow_result",
  "supported_channels": ["webhook", "email", "dingtalk"],
  "subject_template": "通用通知-${i}",
  "body_template": "{{execution_status}} | {{timestamp}} | {{target_hosts}}",
  "format": "text"
}
EOF
)
  print_result "$(post_json "/templates" "$payload")"
done

echo ""
echo "=== 完成！==="
echo "渠道: 15个 (5 webhook + 5 email + 5 dingtalk)"
echo "模板: 15个 (各种 supported_channels 组合)"
