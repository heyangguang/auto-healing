#!/bin/bash
# 通知模块完整功能演示
# 展示所有变量参数 + Ansible 执行日志

set -e

BASE_URL="http://localhost:8080/api/v1"
MOCK_URL="http://localhost:9999"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_error() { echo -e "${RED}[ERROR]${NC} $1"; }
echo_title() { echo -e "\n${CYAN}========== $1 ==========${NC}\n"; }

# 检查 Mock 服务
echo_info "检查 Mock 通知服务..."
if ! curl -s "$MOCK_URL/health" > /dev/null 2>&1; then
  echo_error "Mock 通知服务未运行！请先执行: cd tools && python3 mock_notification.py"
  exit 1
fi

# 清理 Mock 服务
curl -s -X POST "$MOCK_URL/notifications/clear" > /dev/null

# 获取 Token
echo_title "1. 登录认证"
TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123456"}' | jq -r '.access_token')

if [ -z "$TOKEN" ] || [ "$TOKEN" == "null" ]; then
  echo_error "登录失败"
  exit 1
fi
echo_info "Token 获取成功"
AUTH_HEADER="Authorization: Bearer $TOKEN"
SUFFIX=$(date +%s)

# ==================== 创建通知渠道 ====================
echo_title "2. 创建 Webhook 渠道（含重试配置）"

CHANNEL_RESP=$(curl -s -X POST "$BASE_URL/channels" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Full Demo Channel '$SUFFIX'",
    "type": "webhook",
    "description": "完整功能演示渠道",
    "config": {
      "url": "'$MOCK_URL'/webhook",
      "method": "POST",
      "headers": {"X-Source": "auto-healing", "X-Priority": "high"},
      "timeout_seconds": 30
    },
    "default_recipients": ["ops-team@example.com", "sre@example.com"],
    "retry_config": {
      "max_retries": 3,
      "retry_intervals": [1, 5, 15]
    }
  }')

CHANNEL_ID=$(echo "$CHANNEL_RESP" | jq -r '.id')
echo "渠道配置:"
echo "$CHANNEL_RESP" | jq '{id, name, type, is_active, default_recipients, retry_config}'

# ==================== 创建综合模板（包含所有变量）====================
echo_title "3. 创建综合通知模板（35+ 变量）"

# 使用所有可用变量的模板（包含 Ansible 日志）
TEMPLATE_RESP=$(curl -s -X POST "$BASE_URL/templates" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Full Demo Template '$SUFFIX'",
    "description": "展示所有变量的综合模板（含Ansible日志）",
    "event_type": "execution_result",
    "subject_template": "[Auto-Healing] {{execution.status_emoji}} {{task.name}} - {{execution.status}}",
    "body_template": "# 执行结果通知\n\n## 基本信息\n- **时间**: {{timestamp}}\n- **触发者**: {{execution.triggered_by}}\n- **触发类型**: {{execution.trigger_type}}\n\n## 执行信息\n- **Run ID**: {{execution.run_id}}\n- **状态**: {{execution.status}} {{execution.status_emoji}}\n- **退出码**: {{execution.exit_code}}\n- **执行时长**: {{execution.duration}}\n\n## 任务信息\n- **任务名称**: {{task.name}}\n- **目标主机**: {{task.target_hosts}}\n- **执行器**: {{task.executor_type}}\n\n## 仓库信息\n- **仓库名称**: {{repository.name}}\n- **主 Playbook**: {{repository.main_playbook}}\n- **分支**: {{repository.branch}}\n\n## 执行统计\n| OK | Changed | Failed | Unreachable | Skipped |\n|-----|---------|--------|-------------|---------|\n| {{stats.ok}} | {{stats.changed}} | {{stats.failed}} | {{stats.unreachable}} | {{stats.skipped}} |\n\n**成功率**: {{stats.success_rate}}\n\n## Ansible 执行日志\n```\n{{execution.stdout}}\n```\n\n## 错误信息\n{{error.message}}\n{{execution.stderr}}",
    "format": "markdown",
    "supported_channels": ["webhook", "dingtalk", "email"]
  }')

TEMPLATE_ID=$(echo "$TEMPLATE_RESP" | jq -r '.id')
echo "模板基本信息:"
echo "$TEMPLATE_RESP" | jq '{id, name, event_type, format}'

echo ""
echo_info "--- 模板主题 (subject_template) ---"
echo "$TEMPLATE_RESP" | jq -r '.subject_template'

echo ""
echo_info "--- 模板正文 (body_template) ---"
echo "$TEMPLATE_RESP" | jq -r '.body_template'

echo ""
echo_info "--- 模板中使用的变量 ---"
echo "$TEMPLATE_RESP" | jq '.available_variables'

# ==================== 查看可用变量 ====================
echo_title "4. 查看系统支持的所有变量"
curl -s "$BASE_URL/template-variables" -H "$AUTH_HEADER" | jq '.variables | group_by(.category) | .[] | {category: .[0].category, variables: [.[].name]}'

# ==================== 获取 Git 仓库 ====================
echo_title "5. 获取 Git 仓库"
REPO_RESP=$(curl -s "$BASE_URL/git-repos" -H "$AUTH_HEADER")
REPO_ID=$(echo "$REPO_RESP" | jq -r '(.items // .data)[] | select(.is_active == true and .main_playbook == "test_ping.yml") | .id' | head -1)

if [ -z "$REPO_ID" ]; then
  echo_error "没有合适的仓库"
  exit 1
fi

REPO_INFO=$(echo "$REPO_RESP" | jq --arg id "$REPO_ID" '(.items // .data)[] | select(.id == $id) | {id, name, url, main_playbook, is_active}')
echo "$REPO_INFO"

# ==================== 创建任务 ====================
echo_title "6. 创建执行任务（带通知配置）"

TASK_RESP=$(curl -s -X POST "$BASE_URL/execution-tasks" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Full Demo Task '$SUFFIX'",
    "repository_id": "'$REPO_ID'",
    "target_hosts": "localhost",
    "executor_type": "local",
    "notification_config": {
      "enabled": true,
      "on_success": true,
      "on_failure": true,
      "on_timeout": true,
      "template_id": "'$TEMPLATE_ID'",
      "channel_ids": ["'$CHANNEL_ID'"],
      "extra_recipients": ["admin@example.com", "oncall@example.com"]
    }
  }')

TASK_ID=$(echo "$TASK_RESP" | jq -r '.id // .data.id')
echo "任务配置:"
# API 返回 {code, data} 或直接对象
if echo "$TASK_RESP" | jq -e '.data' > /dev/null 2>&1; then
  echo "$TASK_RESP" | jq '.data | {id, name, target_hosts, executor_type, notification_config}'
else
  echo "$TASK_RESP" | jq '{id, name, target_hosts, executor_type, notification_config}'
fi

# ==================== 执行任务 ====================
echo_title "7. 执行任务"

EXEC_RESP=$(curl -s -X POST "$BASE_URL/execution-tasks/$TASK_ID/execute" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"triggered_by": "demo_user"}')

RUN_ID=$(echo "$EXEC_RESP" | jq -r '.id // .data.id')
echo_info "执行已启动: $RUN_ID"

# ==================== 等待完成并显示进度 ====================
echo_title "8. 等待执行完成"
MAX_WAIT=60
WAITED=0

while [ $WAITED -lt $MAX_WAIT ]; do
  RUN_INFO=$(curl -s "$BASE_URL/execution-runs/$RUN_ID" -H "$AUTH_HEADER")
  STATUS=$(echo "$RUN_INFO" | jq -r '.status // .data.status // "pending"')
  
  printf "  ⏳ 状态: %-10s (等待 %ds)\r" "$STATUS" "$WAITED"
  
  if [ "$STATUS" == "success" ] || [ "$STATUS" == "failed" ] || [ "$STATUS" == "timeout" ]; then
    echo ""
    break
  fi
  
  sleep 2
  WAITED=$((WAITED + 2))
done

echo_info "任务执行完成: $STATUS"

# ==================== 显示执行日志 ====================
echo_title "9. Ansible 执行日志"

LOGS=$(curl -s "$BASE_URL/execution-runs/$RUN_ID/logs" -H "$AUTH_HEADER")
echo "$LOGS" | jq -r '(.data // .)[] | "\(.log_level | ascii_upcase)] [\(.stage)] \(.message)"' 2>/dev/null || echo "$LOGS" | jq '.'

# 显示 Ansible 输出
echo ""
echo_info "--- Ansible 标准输出 ---"
echo "$LOGS" | jq -r '(.data // .)[] | select(.stage == "output") | .details.stdout // empty' 2>/dev/null

# 显示统计信息
echo ""
echo_info "--- 执行统计 ---"
echo "$LOGS" | jq '(.data // .)[] | select(.message | contains("执行成功") or contains("执行失败")) | .details' 2>/dev/null || true

# ==================== 验证通知 ====================
echo_title "10. 验证 Mock 服务收到的通知"
sleep 2

MOCK_RESP=$(curl -s "$MOCK_URL/notifications")
NOTIFICATION_COUNT=$(echo "$MOCK_RESP" | jq '.total')

echo_info "收到 $NOTIFICATION_COUNT 条通知"
echo ""

# 显示通知内容
NOTIFICATION=$(echo "$MOCK_RESP" | jq '.notifications[-1]')

echo_info "--- 请求头 ---"
echo "$NOTIFICATION" | jq '.headers | {content_type: ."Content-Type", x_source: ."X-Source", x_priority: ."X-Priority"}' 2>/dev/null || true

echo ""
echo_info "--- 接收人 ---"
echo "$NOTIFICATION" | jq '.body.recipients'

echo ""
echo_info "--- 邮件主题 ---"
echo "$NOTIFICATION" | jq -r '.body.subject'

echo ""
echo_info "--- 通知正文（所有变量值）---"
echo "$NOTIFICATION" | jq -r '.body.body'

# ==================== 查看通知记录 ====================
echo_title "11. 系统通知记录"
curl -s "$BASE_URL/notifications?page_size=1" -H "$AUTH_HEADER" | jq '(.items // .data)[0] | {id, status, sent_at, recipients, subject, channel: .channel.name, template: .template.name}'

# ==================== 清理 ====================
echo_title "12. 清理"
curl -s -X DELETE "$BASE_URL/execution-tasks/$TASK_ID" -H "$AUTH_HEADER" > /dev/null 2>&1 || true
curl -s -X POST "$MOCK_URL/notifications/clear" > /dev/null

echo ""
echo -e "${GREEN}======================================${NC}"
echo -e "${GREEN}   完整功能演示完成！${NC}"
echo -e "${GREEN}======================================${NC}"
