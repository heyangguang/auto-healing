#!/bin/bash
# 通知模块端到端测试脚本
# 完整流程：创建渠道 → 创建模板 → 创建任务(带通知配置) → 执行任务 → 验证通知发送

set -euo pipefail

BASE_URL="http://localhost:8080/api/v1"
MOCK_URL="http://localhost:9999"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/e2e_helpers.sh"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 Mock 服务
echo_info "检查 Mock 通知服务..."
if ! curl -s "$MOCK_URL/health" > /dev/null 2>&1; then
  echo_error "Mock 通知服务未运行！请先执行: cd tools && python3 mock_notification.py"
  exit 1
fi

# 清理 Mock 服务
curl -s -X POST "$MOCK_URL/notifications/clear" > /dev/null

# 获取 Token
echo_info "=== 1. 登录获取 Token ==="
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

echo ""
echo_info "=== 2. 创建 Webhook 渠道 ==="
CHANNEL_RESP=$(curl -s -X POST "$BASE_URL/channels" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Test Channel '$SUFFIX'",
    "type": "webhook",
    "description": "E2E 测试渠道",
    "config": {
      "url": "'$MOCK_URL'/webhook",
      "method": "POST"
    },
    "recipients": ["ops-team@example.com"]
  }')

CHANNEL_ID=$(echo "$CHANNEL_RESP" | jq -r '.data.id')
if [ -z "$CHANNEL_ID" ] || [ "$CHANNEL_ID" == "null" ]; then
  echo_error "创建渠道失败: $CHANNEL_RESP"
  exit 1
fi
echo_info "渠道创建成功: $CHANNEL_ID"

# ==================== 创建通知模板 ====================

echo ""
echo_info "=== 3. 创建通知模板 ==="
TEMPLATE_RESP=$(curl -s -X POST "$BASE_URL/templates" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Test Template '$SUFFIX'",
    "description": "E2E 测试模板",
    "event_type": "execution_result",
    "subject_template": "[Auto-Healing] {{execution.status_emoji}} 任务执行{{execution.status}}",
    "body_template": "任务: {{task.name}}\n状态: {{execution.status}} {{execution.status_emoji}}\n主机: {{task.target_hosts}}\n时长: {{execution.duration}}\n时间: {{timestamp}}\n\n统计:\n- OK: {{stats.ok}}\n- Changed: {{stats.changed}}\n- Failed: {{stats.failed}}",
    "format": "text"
  }')

TEMPLATE_ID=$(echo "$TEMPLATE_RESP" | jq -r '.data.id')
if [ -z "$TEMPLATE_ID" ] || [ "$TEMPLATE_ID" == "null" ]; then
  echo_error "创建模板失败: $TEMPLATE_RESP"
  exit 1
fi
echo_info "模板创建成功: $TEMPLATE_ID"

# ==================== 获取 Git 仓库 ====================

echo ""
echo_info "=== 4. 获取已有 Git 仓库 ==="
REPO_RESP=$(curl -s "$BASE_URL/git-repos" -H "$AUTH_HEADER")
# 选择一个使用 test_ping.yml 的激活仓库（执行快）
REPO_ID=$(echo "$REPO_RESP" | jq -r '.data[] | select(.is_active == true and .main_playbook == "test_ping.yml") | .id' | head -1)

if [ -z "$REPO_ID" ] || [ "$REPO_ID" == "null" ]; then
  echo_error "没有找到合适的 Git 仓库，测试无法继续"
  echo_info "请先创建带 test_ping.yml 的 Git 仓库"
  
  # 清理
  curl -s -X DELETE "$BASE_URL/channels/$CHANNEL_ID" -H "$AUTH_HEADER" > /dev/null 2>&1 || true
  curl -s -X DELETE "$BASE_URL/templates/$TEMPLATE_ID" -H "$AUTH_HEADER" > /dev/null 2>&1 || true
  exit 1
fi
echo_info "使用仓库: $REPO_ID"
PLAYBOOK_ID=$(select_playbook_id "$BASE_URL" "$TOKEN" "$REPO_ID")

# ==================== 创建任务（带通知配置） ====================

echo ""
echo_info "=== 5. 创建执行任务（带通知配置）==="
TASK_RESP=$(curl -s -X POST "$BASE_URL/execution-tasks" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Notification Test Task '$SUFFIX'",
    "playbook_id": "'$PLAYBOOK_ID'",
    "target_hosts": "localhost",
    "executor_type": "local",
    "notification_config": {
      "enabled": true,
      "on_success": true,
      "on_failure": true,
      "on_timeout": true,
      "template_id": "'$TEMPLATE_ID'",
      "channel_ids": ["'$CHANNEL_ID'"],
      "extra_recipients": ["admin@example.com"]
    }
  }')

TASK_ID=$(echo "$TASK_RESP" | jq -r '.data.id // empty')
if [ -z "$TASK_ID" ] || [ "$TASK_ID" == "null" ]; then
  echo_error "创建任务失败: $TASK_RESP"
  exit 1
fi
echo_info "任务创建成功: $TASK_ID"
echo "$TASK_RESP" | jq '.data.notification_config'

# ==================== 执行任务 ====================

echo ""
echo_info "=== 6. 执行任务 ==="
EXEC_RESP=$(curl -s -X POST "$BASE_URL/execution-tasks/$TASK_ID/execute" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{"triggered_by": "e2e_test"}')

RUN_ID=$(echo "$EXEC_RESP" | jq -r '.data.id // empty')
if [ -z "$RUN_ID" ] || [ "$RUN_ID" == "null" ]; then
  echo_error "执行任务失败: $EXEC_RESP"
  exit 1
fi
echo_info "执行已启动: $RUN_ID"

# ==================== 等待执行完成 ====================

echo ""
echo_info "=== 7. 等待执行完成 ==="
MAX_WAIT=60
WAITED=0

while [ $WAITED -lt $MAX_WAIT ]; do
  STATUS=$(curl -s "$BASE_URL/execution-runs/$RUN_ID" -H "$AUTH_HEADER" | jq -er '.data.status')
  echo "  状态: $STATUS (等待 ${WAITED}s)"
  
  if [ "$STATUS" == "success" ] || [ "$STATUS" == "failed" ] || [ "$STATUS" == "timeout" ]; then
    break
  fi
  
  sleep 2
  WAITED=$((WAITED + 2))
done

if [ "$STATUS" != "success" ] && [ "$STATUS" != "failed" ]; then
  echo_error "任务执行超时或异常: $STATUS"
  exit 1
fi

echo_info "任务执行完成: $STATUS"

# ==================== 验证通知发送 ====================

echo ""
echo_info "=== 8. 验证 Mock 服务收到通知 ==="
sleep 2  # 等待异步通知发送

MOCK_RESP=$(curl -s "$MOCK_URL/notifications")
NOTIFICATION_COUNT=$(echo "$MOCK_RESP" | jq '.total')

echo "Mock 服务收到 $NOTIFICATION_COUNT 条通知"

if [ "$NOTIFICATION_COUNT" -ge 1 ]; then
  echo_info "✅ 端到端测试通过！执行任务后自动发送了通知"
  echo ""
  echo "通知内容:"
  echo "$MOCK_RESP" | jq '.notifications[-1].body'
else
  echo_error "❌ 端到端测试失败！Mock 服务未收到通知"
  echo "检查执行日志:"
  curl -s "$BASE_URL/execution-runs/$RUN_ID/logs" -H "$AUTH_HEADER" | jq '.data[-3:]'
  exit 1
fi

# ==================== 清理 ====================

echo ""
echo_info "=== 9. 清理测试数据 ==="
curl -s -X DELETE "$BASE_URL/execution-tasks/$TASK_ID" -H "$AUTH_HEADER" > /dev/null 2>&1 || true
# 不删除渠道和模板，因为有外键约束（通知日志引用它们）
curl -s -X POST "$MOCK_URL/notifications/clear" > /dev/null

echo ""
echo_info "=== 端到端测试完成 ==="
echo_info "所有测试通过！"
