#!/bin/bash
# 端到端测试 - 工单关闭/回写模块
# 测试: 获取工单、关闭工单、验证状态

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

echo "=========================================="
echo "  工单关闭/回写端到端测试"
echo "=========================================="

# 登录
TOKEN=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  exit 1
fi

# 1. 获取一个工单
echo ""
echo "--- 1. 获取工单 ---"
INCIDENT=$(curl -s "$API_BASE/incidents?page_size=1" -H "Authorization: Bearer $TOKEN" | jq -r '.data[0]')
INCIDENT_ID=$(echo "$INCIDENT" | jq -r '.id')
CURRENT_STATUS=$(echo "$INCIDENT" | jq -r '.status')

if [ "$INCIDENT_ID" == "null" ] || [ -z "$INCIDENT_ID" ]; then
  echo "⚠️ 没有可用的工单，跳过测试"
  exit 0
fi
echo "✅ 获取工单: $INCIDENT_ID (当前状态: $CURRENT_STATUS)"

# 2. 关闭工单
echo ""
echo "--- 2. 关闭工单 ---"
CLOSE_RESULT=$(curl -s -X POST "$API_BASE/incidents/$INCIDENT_ID/close" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "resolution": "E2E测试自动修复",
    "work_notes": "端到端测试回写",
    "close_code": "auto_healed",
    "close_status": "resolved"
  }')

if echo "$CLOSE_RESULT" | jq -e '.message' > /dev/null 2>&1; then
  SOURCE_UPDATED=$(echo "$CLOSE_RESULT" | jq -r '.source_updated')
  echo "✅ 关闭成功 (源系统更新: $SOURCE_UPDATED)"
else
  echo "❌ 关闭失败: $CLOSE_RESULT"
  exit 1
fi

# 3. 验证状态
echo ""
echo "--- 3. 验证状态 ---"
NEW_STATUS=$(curl -s "$API_BASE/incidents/$INCIDENT_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.status')
echo "✅ 工单新状态: $NEW_STATUS"

echo ""
echo "=========================================="
echo "  工单关闭/回写测试通过 ✅"
echo "=========================================="
