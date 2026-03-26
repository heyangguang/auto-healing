#!/bin/bash
# 端到端测试 - 定时执行 + SSE 实时日志
# 不联动 CMDB，直接指定主机

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/e2e_helpers.sh"

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_ENDPOINT="${MOCK_ENDPOINT:-http://localhost:5001}"
TARGET_HOST="${TARGET_HOST:-192.168.31.66}"
PLAYBOOK_PATH="${PLAYBOOK_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/playbooks}"

echo "=========================================="
echo "  定时执行 + SSE 实时日志测试"
echo "=========================================="
echo "目标主机: $TARGET_HOST"
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

# ==================== 1. 准备工作 ====================
echo ""
echo "========== 1. 准备工作 =========="

# 清理旧数据
echo "--- 清理旧的测试数据 ---"

# 清理旧密钥源
EXISTING_SECRETS=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Schedule")) | .id')
for ID in $EXISTING_SECRETS; do
  curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "  已删除密钥源: $ID"
done

# 清理旧任务
EXISTING_TASKS=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Schedule")) | .id')
for ID in $EXISTING_TASKS; do
  curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "  已删除任务: $ID"
done

# 清理旧仓库
EXISTING_REPOS=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Schedule")) | .id')
for ID in $EXISTING_REPOS; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "  已删除仓库: $ID"
done

# 创建密钥源
echo ""
echo "--- 创建密钥源 ---"
SECRETS_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Schedule Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_ENDPOINT/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
    }
  }")

SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id')
echo "✅ 密钥源创建成功 (ID: $SECRETS_SOURCE_ID)"

# 创建 Git 仓库
echo ""
echo "--- 创建 Git 仓库 ---"
REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Schedule Test Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")

REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id')
echo "✅ Git 仓库创建成功 (ID: $REPO_ID)"

# 同步仓库
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
echo "✅ 仓库同步完成"

# 激活仓库
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "test_ping.yml", "config_mode": "manual"}' > /dev/null
echo "✅ 仓库已激活"

PLAYBOOK_ID=$(select_playbook_id "$API_BASE" "$TOKEN" "$REPO_ID")

# ==================== 2. 创建定时执行任务 ====================
echo ""
echo "========== 2. 创建定时执行任务 =========="

echo "创建定时任务..."
echo "  调度表达式: 30s（30秒后执行）"
echo "  目标主机: $TARGET_HOST"
echo "  循环执行: 否（仅执行一次用于测试）"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Schedule Ping Test\",
    \"playbook_id\": \"$PLAYBOOK_ID\",
    \"target_hosts\": \"$TARGET_HOST\",
    \"executor_type\": \"local\",
    \"schedule_expr\": \"30s\",
    \"is_recurring\": false
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id')
NEXT_RUN=$(echo "$TASK_RESULT" | jq -r '.data.next_run_at // "N/A"')

if [ "$TASK_ID" != "null" ] && [ -n "$TASK_ID" ]; then
  echo "✅ 定时任务创建成功"
  echo "   任务 ID: $TASK_ID"
  echo "   下次执行: $NEXT_RUN"
else
  echo "❌ 创建定时任务失败: $TASK_RESULT"
  exit 1
fi

# ==================== 3. 手动触发执行并监听 SSE ====================
echo ""
echo "========== 3. 执行任务 + SSE 实时日志 =========="

echo "手动触发执行..."

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"secrets_source_id\": \"$SECRETS_SOURCE_ID\"
  }")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id')

if [ "$RUN_ID" != "null" ] && [ -n "$RUN_ID" ]; then
  echo "✅ 执行已启动 (Run ID: $RUN_ID)"
else
  echo "❌ 执行失败: $EXEC_RESULT"
  exit 1
fi

# 监听 SSE 实时日志
echo ""
echo "--- SSE 实时日志流 ---"
echo "(监听 30 秒...)"
echo ""

# 使用 curl 监听 SSE，超时 30 秒
timeout 30s curl -s -N "$API_BASE/execution-runs/$RUN_ID/stream" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream" 2>/dev/null | while IFS= read -r line; do
  # 解析 SSE 事件
  if [[ "$line" == data:* ]]; then
    DATA="${line#data:}"
    # 提取日志信息
    STAGE=$(echo "$DATA" | jq -r '.stage // ""' 2>/dev/null)
    MESSAGE=$(echo "$DATA" | jq -r '.message // ""' 2>/dev/null)
    STATUS=$(echo "$DATA" | jq -r '.status // ""' 2>/dev/null)
    
    if [ -n "$MESSAGE" ]; then
      echo "  [SSE] [$STAGE] $MESSAGE"
    elif [ -n "$STATUS" ]; then
      echo "  [SSE] 完成: 状态=$STATUS"
    fi
  fi
done || true  # 忽略 timeout 退出码

echo ""
echo "--- SSE 监听结束 ---"

# ==================== 4. 获取最终结果 ====================
echo ""
echo "========== 4. 执行结果 =========="

# 等待执行完成
sleep 2

RUN_DETAIL=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
STATUS=$(echo "$RUN_DETAIL" | jq -r '.data.status')
EXIT_CODE=$(echo "$RUN_DETAIL" | jq -r '.data.exit_code // "N/A"')

echo "状态: $STATUS"
echo "退出码: $EXIT_CODE"
echo ""
echo "统计信息:"
echo "$RUN_DETAIL" | jq '.data.stats'

# 获取 Ansible 输出
echo ""
echo "--- Ansible 输出 ---"
LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=========================================="
if [ "$STATUS" == "success" ]; then
  echo "  ✅ 定时执行 + SSE 实时日志测试成功！"
else
  echo "  ❌ 测试失败 (状态: $STATUS)"
  exit 1
fi
echo "=========================================="
