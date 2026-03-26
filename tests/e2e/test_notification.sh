#!/bin/bash
# 通知模块 E2E 测试脚本
# 测试渠道、模板、发送功能

set -e

BASE_URL="http://localhost:8080/api/v1"
MOCK_URL="http://localhost:9999"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
echo_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
echo_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 获取 Token
echo_info "=== 登录获取 Token ==="
TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123456"}' | jq -r '.access_token')

if [ -z "$TOKEN" ] || [ "$TOKEN" == "null" ]; then
  echo_error "登录失败"
  exit 1
fi
echo_info "Token 获取成功"

# 请求头
AUTH_HEADER="Authorization: Bearer $TOKEN"

# 生成唯一名称后缀
SUFFIX=$(date +%s)

# ==================== 清理旧测试数据 ====================

echo ""
echo_info "=== 0. 清理旧测试数据 ==="

# 删除旧测试渠道 (忽略错误)
OLD_CHANNELS=$(curl -s "$BASE_URL/channels" -H "$AUTH_HEADER" 2>/dev/null | jq -r '.data[]? | select(.name? | startswith("Test ")) | .id // empty' 2>/dev/null || echo "")
for id in $OLD_CHANNELS; do
  if [ -n "$id" ] && [ "$id" != "null" ]; then
    echo "删除旧渠道: $id"
    curl -s -X DELETE "$BASE_URL/channels/$id" -H "$AUTH_HEADER" > /dev/null 2>&1 || true
  fi
done

# 删除旧测试模板 (忽略错误)
OLD_TEMPLATES=$(curl -s "$BASE_URL/templates" -H "$AUTH_HEADER" 2>/dev/null | jq -r '.data[]? | select(.name? | startswith("Test ")) | .id // empty' 2>/dev/null || echo "")
for id in $OLD_TEMPLATES; do
  if [ -n "$id" ] && [ "$id" != "null" ]; then
    echo "删除旧模板: $id"
    curl -s -X DELETE "$BASE_URL/templates/$id" -H "$AUTH_HEADER" > /dev/null 2>&1 || true
  fi
done

# 清理 Mock 服务
curl -s -X POST "$MOCK_URL/notifications/clear" > /dev/null 2>&1 || true

echo_info "清理完成"

# ==================== 测试渠道管理 ====================

echo ""
echo_info "=== 1. 创建 Webhook 渠道 ==="
CHANNEL_RESP=$(curl -s -X POST "$BASE_URL/channels" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Webhook Channel '$SUFFIX'",
    "type": "webhook",
    "description": "测试用 Webhook 渠道",
    "config": {
      "url": "'$MOCK_URL'/webhook",
      "method": "POST",
      "timeout_seconds": 10
    },
    "recipients": ["test@example.com"],
    "retry_config": {
      "max_retries": 3,
      "retry_intervals": [1, 5, 15]
    }
  }')

CHANNEL_ID=$(echo "$CHANNEL_RESP" | jq -r '.data.id')
echo "Channel ID: $CHANNEL_ID"
echo "$CHANNEL_RESP" | jq .

if [ -z "$CHANNEL_ID" ] || [ "$CHANNEL_ID" == "null" ]; then
  echo_error "创建渠道失败"
  echo "$CHANNEL_RESP"
  exit 1
fi
echo_info "渠道创建成功"

echo ""
echo_info "=== 2. 测试渠道连接 ==="
TEST_RESP=$(curl -s -X POST "$BASE_URL/channels/$CHANNEL_ID/test" \
  -H "$AUTH_HEADER")
echo "$TEST_RESP" | jq .

echo ""
echo_info "=== 3. 获取渠道列表 ==="
curl -s "$BASE_URL/channels" -H "$AUTH_HEADER" | jq .

# ==================== 测试模板管理 ====================

echo ""
echo_info "=== 4. 创建通知模板 ==="
TEMPLATE_RESP=$(curl -s -X POST "$BASE_URL/templates" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test 执行结果通知 '$SUFFIX'",
    "description": "任务执行完成后的通知模板",
    "event_type": "execution_result",
    "subject_template": "[Auto-Healing] {{execution.status_emoji}} 任务 {{task.name}} 执行{{execution.status}}",
    "body_template": "## 任务执行结果\n\n**任务名称**: {{task.name}}\n**状态**: {{execution.status}} {{execution.status_emoji}}\n**目标主机**: {{task.target_hosts}}\n**执行时长**: {{execution.duration}}\n**触发者**: {{execution.triggered_by}}\n**时间**: {{timestamp}}\n\n### 执行统计\n- 成功: {{stats.ok}}\n- 变更: {{stats.changed}}\n- 失败: {{stats.failed}}\n- 跳过: {{stats.skipped}}",
    "format": "markdown",
    "supported_channels": ["webhook", "dingtalk"]
  }')

TEMPLATE_ID=$(echo "$TEMPLATE_RESP" | jq -r '.data.id')
echo "Template ID: $TEMPLATE_ID"
echo "$TEMPLATE_RESP" | jq .

if [ -z "$TEMPLATE_ID" ] || [ "$TEMPLATE_ID" == "null" ]; then
  echo_error "创建模板失败"
  exit 1
fi
echo_info "模板创建成功"

echo ""
echo_info "=== 5. 获取可用变量列表 ==="
curl -s "$BASE_URL/template-variables" -H "$AUTH_HEADER" | jq .

echo ""
echo_info "=== 6. 预览模板 ==="
PREVIEW_RESP=$(curl -s -X POST "$BASE_URL/templates/$TEMPLATE_ID/preview" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "variables": {
      "execution": {
        "status": "success",
        "status_emoji": "✅",
        "duration": "2m 35s",
        "triggered_by": "admin"
      },
      "task": {
        "name": "重启 MySQL 服务",
        "target_hosts": "192.168.31.66"
      },
      "stats": {
        "ok": 5,
        "changed": 2,
        "failed": 0,
        "skipped": 1
      },
      "timestamp": "2026-01-05 12:00:00"
    }
  }')
echo "$PREVIEW_RESP" | jq .

# 检查变量是否被正确替换
BODY=$(echo "$PREVIEW_RESP" | jq -r '.data.body')
if echo "$BODY" | grep -q "{{"; then
  echo_error "模板变量未完全替换！"
  echo "$BODY"
  exit 1
fi
echo_info "模板变量替换成功"

# ==================== 测试发送通知 ====================

echo ""
echo_info "=== 7. 发送通知 ==="
SEND_RESP=$(curl -s -X POST "$BASE_URL/notifications/send" \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -d '{
    "template_id": "'$TEMPLATE_ID'",
    "channel_ids": ["'$CHANNEL_ID'"],
    "recipients": ["sre@example.com"],
    "variables": {
      "execution": {
        "status": "success",
        "status_emoji": "✅",
        "duration": "2m 35s",
        "triggered_by": "admin"
      },
      "task": {
        "name": "重启 MySQL 服务",
        "target_hosts": "192.168.31.66"
      },
      "stats": {
        "ok": 5,
        "changed": 2,
        "failed": 0,
        "skipped": 1
      },
      "timestamp": "2026-01-05 12:00:00"
    }
  }')
echo "$SEND_RESP" | jq .

NOTIFICATION_ID=$(echo "$SEND_RESP" | jq -r '.data.notification_ids[0]')
echo "Notification ID: $NOTIFICATION_ID"

echo ""
echo_info "=== 8. 查看通知记录 ==="
curl -s "$BASE_URL/notifications" -H "$AUTH_HEADER" | jq .

echo ""
echo_info "=== 9. 查看通知详情 ==="
curl -s "$BASE_URL/notifications/$NOTIFICATION_ID" -H "$AUTH_HEADER" | jq .

echo ""
echo_info "=== 10. 检查 Mock 服务接收到的通知 ==="
MOCK_NOTIFICATIONS=$(curl -s "$MOCK_URL/notifications")
echo "$MOCK_NOTIFICATIONS" | jq .

TOTAL=$(echo "$MOCK_NOTIFICATIONS" | jq '.total')
if [ "$TOTAL" -ge 1 ]; then
  echo_info "Mock 服务成功接收到 $TOTAL 条通知"
else
  echo_error "Mock 服务未接收到通知"
  exit 1
fi

# ==================== 清理 ====================

echo ""
echo_info "=== 清理测试数据 ==="
curl -s -X DELETE "$BASE_URL/channels/$CHANNEL_ID" -H "$AUTH_HEADER" | jq .
curl -s -X DELETE "$BASE_URL/templates/$TEMPLATE_ID" -H "$AUTH_HEADER" | jq .
curl -s -X POST "$MOCK_URL/notifications/clear" | jq .

echo ""
echo_info "=== 测试完成 ==="
echo_info "所有测试通过！"
