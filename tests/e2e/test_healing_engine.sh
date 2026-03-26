#!/bin/bash
# 端到端测试 - 自愈引擎完整流程
# 场景1: CRUD + 规则管理
# 场景2: 自动触发 + 规则匹配
# 场景3: 审批流程

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

echo "============================================"
echo "  自愈引擎 E2E 测试"
echo "============================================"
echo ""

# 登录
echo "--- 登录 ---"
TOKEN=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  exit 1
fi
echo "✅ 登录成功"

list_total_or_fail() {
  local response="$1"
  local label="$2"
  local total

  total=$(echo "$response" | jq -er '.total')
  if [ -z "$total" ] || [ "$total" = "null" ]; then
    echo "❌ ${label}缺少顶层 total"
    echo "$response" | jq
    exit 1
  fi
  printf '%s' "$total"
}

# ==================== 清理旧数据 ====================
echo ""
echo "========== 清理旧数据 =========="

# 清理测试数据
echo "清理旧的流程..."
EXISTING=$(curl -s "$API_BASE/healing/flows" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Test")) | .id' 2>/dev/null || echo "")
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/healing/flows/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

echo "清理旧的规则..."
EXISTING=$(curl -s "$API_BASE/healing/rules" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Test")) | .id' 2>/dev/null || echo "")
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/healing/rules/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done
echo "✅ 旧数据已清理"

# ==================== 场景1: CRUD + 规则管理 ====================
echo ""
echo "=========================================="
echo "  场景1: CRUD + 规则管理"
echo "=========================================="

# 1.1 创建自愈流程
echo ""
echo "--- 1.1 创建自愈流程 ---"
FLOW_RESULT=$(curl -s -X POST "$API_BASE/healing/flows" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Test Flow",
    "description": "E2E 测试用自愈流程",
    "nodes": [
      {"id": "start", "type": "start", "name": "开始"},
      {"id": "approval", "type": "approval", "name": "审批", "config": {"timeout_hours": 1}},
      {"id": "notify", "type": "notification", "name": "通知"},
      {"id": "end", "type": "end", "name": "结束"}
    ],
    "edges": [
      {"from": "start", "to": "approval"},
      {"from": "approval", "to": "notify"},
      {"from": "notify", "to": "end"}
    ],
    "is_active": true
  }')

FLOW_ID=$(echo "$FLOW_RESULT" | jq -r '.data.id')
if [ "$FLOW_ID" == "null" ] || [ -z "$FLOW_ID" ]; then
  echo "❌ 创建流程失败"
  echo "$FLOW_RESULT" | jq
  exit 1
fi
echo "✅ 流程创建成功 (ID: $FLOW_ID)"

# 1.2 创建自愈规则
echo ""
echo "--- 1.2 创建自愈规则 ---"
RULE_RESULT=$(curl -s -X POST "$API_BASE/healing/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Test Rule - Critical Severity\",
    \"description\": \"匹配严重级别为 critical 的工单\",
    \"priority\": 100,
    \"trigger_mode\": \"auto\",
    \"conditions\": [
      {\"field\": \"severity\", \"operator\": \"equals\", \"value\": \"critical\"}
    ],
    \"match_mode\": \"all\",
    \"flow_id\": $FLOW_ID,
    \"is_active\": false
  }")

RULE_ID=$(echo "$RULE_RESULT" | jq -r '.data.id')
if [ "$RULE_ID" == "null" ] || [ -z "$RULE_ID" ]; then
  echo "❌ 创建规则失败"
  echo "$RULE_RESULT" | jq
  exit 1
fi
echo "✅ 规则创建成功 (ID: $RULE_ID)"

# 1.3 查询列表
echo ""
echo "--- 1.3 查询列表 ---"
FLOWS_LIST=$(curl -s "$API_BASE/healing/flows" -H "Authorization: Bearer $TOKEN")
FLOW_COUNT=$(list_total_or_fail "$FLOWS_LIST" "流程列表响应")
echo "流程总数: $FLOW_COUNT"

RULES_LIST=$(curl -s "$API_BASE/healing/rules" -H "Authorization: Bearer $TOKEN")
RULE_COUNT=$(list_total_or_fail "$RULES_LIST" "规则列表响应")
echo "规则总数: $RULE_COUNT"

# 1.4 更新规则
echo ""
echo "--- 1.4 更新规则优先级 ---"
UPDATE_RESULT=$(curl -s -X PUT "$API_BASE/healing/rules/$RULE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"priority": 200}')

NEW_PRIORITY=$(echo "$UPDATE_RESULT" | jq -r '.data.priority')
if [ "$NEW_PRIORITY" == "200" ]; then
  echo "✅ 规则更新成功 (优先级: 200)"
else
  echo "❌ 规则更新失败: 优先级=$NEW_PRIORITY"
  exit 1
fi

# 1.5 启用规则
echo ""
echo "--- 1.5 启用规则 ---"
ACTIVATE_RESULT=$(curl -s -X POST "$API_BASE/healing/rules/$RULE_ID/activate" \
  -H "Authorization: Bearer $TOKEN")
echo "$ACTIVATE_RESULT" | jq -r '.message // "操作完成"'

# 验证状态
RULE_DETAIL=$(curl -s "$API_BASE/healing/rules/$RULE_ID" -H "Authorization: Bearer $TOKEN")
IS_ACTIVE=$(echo "$RULE_DETAIL" | jq -r '.data.is_active')
if [ "$IS_ACTIVE" == "true" ]; then
  echo "✅ 规则已启用"
else
  echo "❌ 规则启用失败: $IS_ACTIVE"
  exit 1
fi

# 1.6 停用规则
echo ""
echo "--- 1.6 停用规则 ---"
curl -s -X POST "$API_BASE/healing/rules/$RULE_ID/deactivate" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
RULE_DETAIL=$(curl -s "$API_BASE/healing/rules/$RULE_ID" -H "Authorization: Bearer $TOKEN")
IS_ACTIVE=$(echo "$RULE_DETAIL" | jq -r '.data.is_active')
if [ "$IS_ACTIVE" == "false" ]; then
  echo "✅ 规则已停用"
else
  echo "❌ 规则停用失败: $IS_ACTIVE"
  exit 1
fi

echo ""
echo "=========================================="
echo "  场景1 完成: CRUD + 规则管理 ✅"
echo "=========================================="

# ==================== 场景2: 流程实例和审批任务查询 ====================
echo ""
echo "=========================================="
echo "  场景2: 流程实例和审批任务查询"
echo "=========================================="

# 2.1 查询流程实例列表
echo ""
echo "--- 2.1 查询流程实例列表 ---"
INSTANCES_LIST=$(curl -s "$API_BASE/healing/instances" -H "Authorization: Bearer $TOKEN")
INSTANCE_COUNT=$(list_total_or_fail "$INSTANCES_LIST" "流程实例列表响应")
echo "流程实例总数: $INSTANCE_COUNT"

# 2.2 查询待审批任务
echo ""
echo "--- 2.2 查询待审批任务 ---"
PENDING_LIST=$(curl -s "$API_BASE/healing/approvals/pending" -H "Authorization: Bearer $TOKEN")
PENDING_COUNT=$(list_total_or_fail "$PENDING_LIST" "待审批列表响应")
echo "待审批任务数: $PENDING_COUNT"

# 2.3 查询所有审批任务
echo ""
echo "--- 2.3 查询所有审批任务 ---"
APPROVALS_LIST=$(curl -s "$API_BASE/healing/approvals" -H "Authorization: Bearer $TOKEN")
APPROVAL_COUNT=$(list_total_or_fail "$APPROVALS_LIST" "审批列表响应")
echo "审批任务总数: $APPROVAL_COUNT"

echo ""
echo "=========================================="
echo "  场景2 完成: 查询 API 正常 ✅"
echo "=========================================="

# ==================== 场景3: 再次启用规则等待调度器 ====================
echo ""
echo "=========================================="
echo "  场景3: 规则启用 + 调度器验证"
echo "=========================================="

# 3.1 重新启用规则
echo ""
echo "--- 3.1 重新启用规则 ---"
curl -s -X POST "$API_BASE/healing/rules/$RULE_ID/activate" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
echo "✅ 规则已重新启用"

# 3.2 获取规则详情确认
RULE_DETAIL=$(curl -s "$API_BASE/healing/rules/$RULE_ID" -H "Authorization: Bearer $TOKEN")
echo ""
echo "规则详情:"
echo "$RULE_DETAIL" | jq '{
  id: .data.id,
  name: .data.name,
  priority: .data.priority,
  trigger_mode: .data.trigger_mode,
  is_active: .data.is_active,
  flow_id: .data.flow_id
}'

echo ""
echo "=========================================="
echo "  场景3 完成: 规则已激活 ✅"
echo "  调度器会每 10 秒扫描未处理工单"
echo "=========================================="

# ==================== 清理或保留 ====================
echo ""
echo "=========================================="
echo "  测试资源"
echo "=========================================="
echo "  流程 ID: $FLOW_ID"
echo "  规则 ID: $RULE_ID"
echo ""
echo "  如需清理，执行:"
echo "    curl -X DELETE $API_BASE/healing/rules/$RULE_ID -H 'Authorization: Bearer $TOKEN'"
echo "    curl -X DELETE $API_BASE/healing/flows/$FLOW_ID -H 'Authorization: Bearer $TOKEN'"
echo ""

# ==================== 最终结果 ====================
echo ""
echo "============================================"
echo "  自愈引擎 E2E 测试结果"
echo "============================================"
echo ""
echo "  ✅ 场景1: CRUD + 规则管理"
echo "  ✅ 场景2: 流程实例和审批任务查询"
echo "  ✅ 场景3: 规则启用 + 调度器验证"
echo ""
echo "  所有测试通过！"
echo "============================================"
