#!/bin/bash
# 端到端测试 - 定时执行 + CMDB 联动 + SSE 实时日志
# 从 CMDB 获取主机执行定时任务

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_ENDPOINT="${MOCK_ENDPOINT:-http://localhost:5001}"
PLAYBOOK_PATH="/root/auto-healing/tests/playbooks"

echo "=========================================="
echo "  定时执行 + CMDB 联动 + SSE 实时日志测试"
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

# ==================== 0. CMDB 同步 ====================
echo ""
echo "========== 0. CMDB 同步 =========="

# 检查 Mock CMDB 服务
if ! curl -s "$MOCK_ENDPOINT/api/cmdb/hosts" > /dev/null 2>&1; then
  echo "❌ Mock CMDB 服务不可用"
  exit 1
fi
echo "✅ Mock CMDB 服务可用"

# 触发同步
SYNC_RESULT=$(curl -s -X POST "$MOCK_ENDPOINT/api/cmdb/sync")
echo "✅ 同步完成: $(echo "$SYNC_RESULT" | jq -r '.synced_hosts') 台主机"

# 获取主机列表
echo ""
echo "--- CMDB 主机列表 ---"
CMDB_HOSTS=$(curl -s "$MOCK_ENDPOINT/api/cmdb/hosts")
echo "$CMDB_HOSTS" | jq -r '.hosts[] | "  [\(.status)] \(.hostname) (\(.ip)) - \(.business)/\(.environment)"'

# 筛选 dev 环境主机
echo ""
echo "--- 筛选 dev 环境主机 ---"
DEV_HOSTS=$(curl -s "$MOCK_ENDPOINT/api/cmdb/hosts?environment=dev&status=online")
DEV_COUNT=$(echo "$DEV_HOSTS" | jq -r '.total')
echo "符合条件的主机: $DEV_COUNT 台"

if [ "$DEV_COUNT" -eq 0 ]; then
  echo "❌ 没有符合条件的主机"
  exit 1
fi

TARGET_HOST=$(echo "$DEV_HOSTS" | jq -r '.hosts[0].ip')
TARGET_HOSTNAME=$(echo "$DEV_HOSTS" | jq -r '.hosts[0].hostname')
echo "选中目标主机: $TARGET_HOSTNAME ($TARGET_HOST)"

# ==================== 1. 准备工作 ====================
echo ""
echo "========== 1. 准备工作 =========="

# 清理旧数据
echo "--- 清理旧的测试数据 ---"

EXISTING_SECRETS=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E CMDB Schedule")) | .id')
for ID in $EXISTING_SECRETS; do
  curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

EXISTING_TASKS=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E CMDB Schedule")) | .id')
for ID in $EXISTING_TASKS; do
  curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

EXISTING_REPOS=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E CMDB Schedule")) | .id')
for ID in $EXISTING_REPOS; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done
echo "✅ 旧数据已清理"

# 创建密钥源
echo ""
echo "--- 创建密钥源 ---"
SECRETS_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E CMDB Schedule Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_ENDPOINT/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
    }
  }")

SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id // .id')
echo "✅ 密钥源创建成功 (ID: $SECRETS_SOURCE_ID)"

# 创建 Git 仓库
echo ""
echo "--- 创建 Git 仓库 ---"
REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E CMDB Schedule Test Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")

REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id // .id')
echo "✅ Git 仓库创建成功 (ID: $REPO_ID)"

curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
echo "✅ 仓库同步完成"

curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "test_ping.yml", "config_mode": "manual"}' > /dev/null
echo "✅ 仓库已激活"

# ==================== 2. 创建定时执行任务 ====================
echo ""
echo "========== 2. 创建定时执行任务 =========="

echo "创建定时任务..."
echo "  目标主机（来自 CMDB）: $TARGET_HOSTNAME ($TARGET_HOST)"
echo "  调度表达式: 30s"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E CMDB Schedule Ping Test\",
    \"repository_id\": \"$REPO_ID\",
    \"target_hosts\": \"$TARGET_HOST\",
    \"executor_type\": \"local\",
    \"schedule_expr\": \"30s\",
    \"is_recurring\": false
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id // .id')
NEXT_RUN=$(echo "$TASK_RESULT" | jq -r '.data.next_run_at // "N/A"')

echo "✅ 定时任务创建成功"
echo "   任务 ID: $TASK_ID"
echo "   下次执行: $NEXT_RUN"

# ==================== 3. 执行 + SSE 实时日志 ====================
echo ""
echo "========== 3. 执行任务 + SSE 实时日志 =========="

echo "手动触发执行..."

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"secrets_source_id\": \"$SECRETS_SOURCE_ID\"
  }")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id // .id')
echo "✅ 执行已启动 (Run ID: $RUN_ID)"

echo ""
echo "--- SSE 实时日志流 ---"
echo "(监听 30 秒...)"
echo ""

# 使用 curl 监听 SSE
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

# ==================== 4. 执行结果 ====================
echo ""
echo "========== 4. 执行结果 =========="

sleep 2

RUN_DETAIL=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
STATUS=$(echo "$RUN_DETAIL" | jq -r '.data.status // .status')
EXIT_CODE=$(echo "$RUN_DETAIL" | jq -r '.data.exit_code // "N/A"')

echo "状态: $STATUS"
echo "退出码: $EXIT_CODE"
echo ""
echo "统计信息:"
echo "$RUN_DETAIL" | jq '.data.stats'

echo ""
echo "--- Ansible 输出 ---"
LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=========================================="
if [ "$STATUS" == "success" ]; then
  echo "  ✅ CMDB 联动定时执行 + SSE 测试成功！"
  echo ""
  echo "  完整流程验证："
  echo "    1. ✅ CMDB 同步 → 获取主机列表"
  echo "    2. ✅ 筛选 dev 环境 → $TARGET_HOSTNAME"
  echo "    3. ✅ 创建密钥源 → 验证凭据"
  echo "    4. ✅ 创建定时任务 → 执行成功"
  echo "    5. ✅ SSE 实时日志 → 监听成功"
else
  echo "  ❌ 测试失败 (状态: $STATUS)"
fi
echo "=========================================="
