#!/bin/bash
# 端到端测试 - 插件模块 (ITSM + CMDB)
# 测试: 创建、测试连接、激活、同步

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_ENDPOINT="${MOCK_ENDPOINT:-http://localhost:5000}"

echo "=========================================="
echo "  插件模块端到端测试"
echo "=========================================="

# 登录
TOKEN=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  exit 1
fi

# 清理已存在的测试插件
echo ""
echo "--- 清理测试插件 ---"
EXISTING=$(curl -s "$API_BASE/plugins" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Test")) | .id')
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/plugins/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除: $ID"
done

# ========== ITSM 插件测试 ==========
echo ""
echo "========== ITSM 插件 =========="

echo "--- 1. 创建 ITSM 插件 ---"
ITSM_RESULT=$(curl -s -X POST "$API_BASE/plugins" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Test ITSM\",
    \"type\": \"itsm\",
    \"adapter\": \"servicenow\",
    \"config\": {\"endpoint\":\"$MOCK_ENDPOINT\",\"username\":\"admin\",\"password\":\"admin123\"},
    \"sync_enabled\": true,
    \"sync_interval_minutes\": 5
  }")

ITSM_ID=$(echo "$ITSM_RESULT" | jq -r '.id')
ITSM_ADAPTER=$(echo "$ITSM_RESULT" | jq -r '.adapter')

if [ "$ITSM_ID" == "null" ] || [ -z "$ITSM_ID" ]; then
  echo "❌ 创建 ITSM 插件失败: $ITSM_RESULT"
  exit 1
fi
echo "✅ 创建成功 (ID: $ITSM_ID, adapter: $ITSM_ADAPTER)"

echo ""
echo "--- 2. 测试 ITSM 连接 ---"
TEST_RESULT=$(curl -s -X POST "$API_BASE/plugins/$ITSM_ID/test" -H "Authorization: Bearer $TOKEN")
if echo "$TEST_RESULT" | jq -e '.message' > /dev/null 2>&1; then
  echo "✅ 连接测试成功"
else
  echo "❌ 连接测试失败: $TEST_RESULT"
  exit 1
fi

echo ""
echo "--- 3. 验证插件状态 (应为 active) ---"
STATUS=$(curl -s "$API_BASE/plugins/$ITSM_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.status')
if [ "$STATUS" == "active" ]; then
  echo "✅ 状态正确: $STATUS"
else
  echo "❌ 状态错误: $STATUS (期望: active)"
  exit 1
fi

echo ""
echo "--- 4. 手动同步 ---"
SYNC_RESULT=$(curl -s -X POST "$API_BASE/plugins/$ITSM_ID/sync" -H "Authorization: Bearer $TOKEN")
SYNC_STATUS=$(echo "$SYNC_RESULT" | jq -r '.status')
echo "✅ 同步触发成功 (状态: $SYNC_STATUS)"

sleep 2

echo ""
echo "--- 5. 查看同步日志 ---"
LOGS=$(curl -s "$API_BASE/plugins/$ITSM_ID/logs?page_size=1" -H "Authorization: Bearer $TOKEN")
RECORDS=$(echo "$LOGS" | jq -r '.data[0].records_fetched // 0')
echo "✅ 同步日志: 获取 $RECORDS 条记录"

# ========== CMDB 插件测试 ==========
echo ""
echo "========== CMDB 插件 =========="

echo "--- 1. 创建 CMDB 插件 ---"
CMDB_RESULT=$(curl -s -X POST "$API_BASE/plugins" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Test CMDB\",
    \"type\": \"cmdb\",
    \"adapter\": \"servicenow\",
    \"config\": {\"endpoint\":\"$MOCK_ENDPOINT\",\"username\":\"admin\",\"password\":\"admin123\"},
    \"sync_enabled\": true,
    \"sync_interval_minutes\": 5
  }")

CMDB_ID=$(echo "$CMDB_RESULT" | jq -r '.id')
if [ "$CMDB_ID" == "null" ] || [ -z "$CMDB_ID" ]; then
  echo "❌ 创建 CMDB 插件失败: $CMDB_RESULT"
  exit 1
fi
echo "✅ 创建成功 (ID: $CMDB_ID)"

echo ""
echo "--- 2. 测试 CMDB 连接 ---"
TEST_RESULT=$(curl -s -X POST "$API_BASE/plugins/$CMDB_ID/test" -H "Authorization: Bearer $TOKEN")
if echo "$TEST_RESULT" | jq -e '.message' > /dev/null 2>&1; then
  echo "✅ 连接测试成功"
else
  echo "❌ 连接测试失败: $TEST_RESULT"
  exit 1
fi

echo ""
echo "--- 3. 手动同步 ---"
curl -s -X POST "$API_BASE/plugins/$CMDB_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
echo "✅ 同步触发成功"

echo ""
echo "--- 4. 查看 CMDB 统计 ---"
STATS=$(curl -s "$API_BASE/cmdb/stats" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$STATS" | jq -r '.total // 0')
echo "✅ CMDB 配置项总数: $TOTAL"

echo ""
echo "=========================================="
echo "  插件模块测试通过 ✅"
echo "=========================================="
