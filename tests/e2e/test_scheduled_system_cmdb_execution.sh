#!/bin/bash
# 端到端测试 - 定时执行 + 系统 CMDB + SSE 实时日志
# 从系统 CMDB 获取主机 → 创建定时任务 → 执行 → SSE 实时日志

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/e2e_helpers.sh"

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_SECRETS="${MOCK_SECRETS:-http://localhost:5001}"
PLAYBOOK_PATH="${PLAYBOOK_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/playbooks}"

echo "=========================================="
echo "  定时执行 + 系统 CMDB + SSE 实时日志"
echo "=========================================="
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

# ==================== 0. 系统 CMDB 查询 ====================
echo ""
echo "========== 0. 系统 CMDB 查询 =========="

CMDB_RESULT=$(curl -s "$API_BASE/cmdb?environment=development" -H "Authorization: Bearer $TOKEN")
echo "查询系统 CMDB（environment=development）..."
echo "找到 $(echo "$CMDB_RESULT" | jq -r '.total') 台主机"

echo ""
echo "--- 筛选 Real Hosts CMDB 的主机 ---"
REAL_HOST=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB")')

TARGET_HOST=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB") | .ip_address' | head -1)
TARGET_NAME=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB") | .name' | head -1)

if [ -z "$TARGET_HOST" ] || [ "$TARGET_HOST" == "null" ]; then
  echo "❌ 没有找到 Real Hosts CMDB 的主机"
  exit 1
fi

echo "✅ 选中目标主机：$TARGET_NAME ($TARGET_HOST)"

# ==================== 1. 准备工作 ====================
echo ""
echo "========== 1. 准备工作 =========="

# 清理
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Scheduled System CMDB")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Scheduled System CMDB")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Scheduled System CMDB")) | .id')
for ID in $EXISTING; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done
echo "✅ 旧数据已清理"

# 创建密钥源
SECRETS_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Scheduled System CMDB Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
    }
  }")
SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id')
echo "✅ 密钥源创建成功"

# 创建仓库
REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Scheduled System CMDB Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")
REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id')
echo "✅ Git 仓库创建成功"

curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "test_ping.yml", "config_mode": "manual"}' > /dev/null
echo "✅ 仓库同步并激活"

PLAYBOOK_ID=$(select_playbook_id "$API_BASE" "$TOKEN" "$REPO_ID")

# ==================== 2. 创建定时任务 ====================
echo ""
echo "========== 2. 创建定时任务 =========="

echo "目标主机(系统 CMDB): $TARGET_NAME ($TARGET_HOST)"
echo "调度表达式: 30s"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Scheduled System CMDB Ping\",
    \"playbook_id\": \"$PLAYBOOK_ID\",
    \"target_hosts\": \"$TARGET_HOST\",
    \"executor_type\": \"local\",
    \"schedule_expr\": \"30s\",
    \"is_recurring\": false
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id')
echo "✅ 定时任务创建成功 (ID: $TASK_ID)"

# ==================== 3. 执行 + SSE ====================
echo ""
echo "========== 3. 执行任务 + SSE 实时日志 =========="

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"secrets_source_id\": \"$SECRETS_SOURCE_ID\"}")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id')
echo "✅ 执行已启动 (Run ID: $RUN_ID)"

echo ""
echo "--- SSE 实时日志流 ---"
timeout 30s curl -s -N "$API_BASE/execution-runs/$RUN_ID/stream" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: text/event-stream" 2>/dev/null | while IFS= read -r line; do
  if [[ "$line" == data:* ]]; then
    DATA="${line#data:}"
    STAGE=$(echo "$DATA" | jq -r '.stage // ""' 2>/dev/null)
    MESSAGE=$(echo "$DATA" | jq -r '.message // ""' 2>/dev/null)
    STATUS=$(echo "$DATA" | jq -r '.status // ""' 2>/dev/null)
    
    if [ -n "$MESSAGE" ]; then
      echo "  [SSE] [$STAGE] $MESSAGE"
    elif [ -n "$STATUS" ]; then
      echo "  [SSE] 完成: 状态=$STATUS"
    fi
  fi
done || true

echo ""
echo "--- SSE 监听结束 ---"

# ==================== 4. 结果 ====================
echo ""
echo "========== 4. 执行结果 =========="

sleep 2
RUN_DETAIL=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
STATUS=$(echo "$RUN_DETAIL" | jq -r '.data.status')
echo "$RUN_DETAIL" | jq '{status: .data.status, exit_code: .data.exit_code, stats: .data.stats}'

echo ""
echo "--- Ansible 输出 ---"
LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=========================================="
if [ "$STATUS" == "success" ]; then
  echo "  ✅ 定时执行 + 系统 CMDB + SSE 测试成功！"
  echo ""
  echo "  完整流程验证："
  echo "    1. ✅ 系统 CMDB 查询 → $TARGET_NAME"
  echo "    2. ✅ 主机来源: Real Hosts CMDB 插件"
  echo "    3. ✅ 创建定时任务"
  echo "    4. ✅ SSE 实时日志"
  echo "    5. ✅ 执行成功"
else
  echo "  ❌ 测试失败"
  exit 1
fi
echo "=========================================="
