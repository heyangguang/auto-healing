#!/bin/bash
# 端到端测试 - 多主机执行（成功和失败场景）
# 目标：1台正确主机 + 1台错误主机，验证执行结果

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_SECRETS="${MOCK_SECRETS:-http://localhost:5001}"
PLAYBOOK_PATH="/root/auto-healing/tests/playbooks"

# 正确的主机（能连上）
GOOD_HOST="192.168.31.66"
# 错误的主机（连不上）
BAD_HOST="192.168.31.99"

echo "=========================================="
echo "  多主机执行测试（成功+失败场景）"
echo "=========================================="
echo ""
echo "正确主机: $GOOD_HOST (应该成功)"
echo "错误主机: $BAD_HOST (应该失败)"
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
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Multi Host")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Multi Host")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Multi Host")) | .id')
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
    \"name\": \"E2E Multi Host Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
    }
  }")
SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id // .id')
echo "✅ 密钥源创建成功"

# 创建仓库
REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Multi Host Test Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")
REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id // .id')
echo "✅ Git 仓库创建成功"

curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "test_ping.yml", "config_mode": "manual"}' > /dev/null
echo "✅ 仓库同步并激活"

# ==================== 2. 创建任务（多主机）====================
echo ""
echo "========== 2. 创建任务（多主机）=========="

# 目标主机：正确的 + 错误的
TARGET_HOSTS="$GOOD_HOST,$BAD_HOST"
echo "目标主机: $TARGET_HOSTS"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Multi Host Test\",
    \"repository_id\": \"$REPO_ID\",
    \"target_hosts\": \"$TARGET_HOSTS\",
    \"executor_type\": \"local\"
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id // .id')
echo "✅ 任务创建成功 (ID: $TASK_ID)"

# ==================== 3. 执行任务 ====================
echo ""
echo "========== 3. 执行任务 =========="

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"secrets_source_id\": \"$SECRETS_SOURCE_ID\"}")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id // .id')
echo "✅ 执行已启动 (Run ID: $RUN_ID)"

# SSE 实时日志
echo ""
echo "--- SSE 实时日志流 ---"
timeout 60s curl -s -N "$API_BASE/execution-runs/$RUN_ID/stream" \
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
STATUS=$(echo "$RUN_DETAIL" | jq -r '.data.status')
EXIT_CODE=$(echo "$RUN_DETAIL" | jq -r '.data.exit_code')

echo "状态: $STATUS"
echo "退出码: $EXIT_CODE"
echo ""
echo "统计信息:"
echo "$RUN_DETAIL" | jq '.data.stats'

# 日志
echo ""
echo "--- 执行日志 ---"
LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
echo "$LOGS" | jq -r '.data[] | select(.stage == "prepare") | "  [准备] \(.message)"'
echo "$LOGS" | jq -r '.data[] | select(.stage == "execute") | "  [执行] \(.message)"'

echo ""
echo "--- Ansible 输出 ---"
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=========================================="
echo "  测试结果分析"
echo "=========================================="
echo ""
echo "正确主机 ($GOOD_HOST):"
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty' | grep -A1 "$GOOD_HOST" | head -5 || echo "  (无特定输出)"

echo ""
echo "错误主机 ($BAD_HOST):"
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty' | grep -A1 "$BAD_HOST" | head -5 || echo "  (应显示 unreachable)"

echo ""
if [ "$STATUS" == "success" ]; then
  echo "  状态: success（但检查 unreachable 是否 > 0）"
elif [ "$STATUS" == "failed" ]; then
  echo "  状态: failed（预期结果，因为有主机连不上）"
else
  echo "  状态: $STATUS"
fi
echo "=========================================="
