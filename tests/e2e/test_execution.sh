#!/bin/bash
# 端到端测试 - 执行任务集成密钥服务
# 完整流程：创建密钥源 → 创建Git仓库 → 创建任务模板 → 执行任务 → 查看日志

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/e2e_helpers.sh"

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_SECRETS_ENDPOINT="${MOCK_SECRETS_ENDPOINT:-http://localhost:5001}"
TARGET_HOST="${TARGET_HOST:-192.168.31.66}"
PLAYBOOK_PATH="${PLAYBOOK_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/playbooks}"

echo "=========================================="
echo "  执行任务端到端测试（完整流程）"
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

# ==================== 1. 创建密钥源 ====================
echo ""
echo "========== 1. 创建密钥源 =========="

# 清理旧的测试密钥源
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Exec")) | .id')
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除旧密钥源: $ID"
done

# 创建 Webhook 密钥源（密码方式）
echo ""
echo "创建密钥源..."
echo "  类型: webhook"
echo "  认证方式: password"
echo "  Webhook URL: $MOCK_SECRETS_ENDPOINT/api/secrets/query"

SECRETS_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Exec Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS_ENDPOINT/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
    }
  }")

SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id')
if [ "$SECRETS_SOURCE_ID" != "null" ] && [ -n "$SECRETS_SOURCE_ID" ]; then
  echo "✅ 密钥源创建成功 (ID: $SECRETS_SOURCE_ID)"
else
  echo "❌ 创建密钥源失败: $SECRETS_RESULT"
  exit 1
fi

# 验证密钥源
echo ""
echo "--- 验证密钥源（查询 $TARGET_HOST 的凭据）---"
QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"hostname\":\"$TARGET_HOST\",\"source_id\":\"$SECRETS_SOURCE_ID\"}")

AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type')
PASSWORD_VAL=$(echo "$QUERY_RESULT" | jq -r '.data.password')
USERNAME_VAL=$(echo "$QUERY_RESULT" | jq -r '.data.username')
if [ "$AUTH_TYPE" == "password" ]; then
  echo "✅ 密钥源验证成功"
  echo "   主机: $TARGET_HOST"
  echo "   用户名: $USERNAME_VAL"
  echo "   密码: ${PASSWORD_VAL:0:3}***${PASSWORD_VAL: -1}"
else
  echo "❌ 密钥源验证失败: $QUERY_RESULT"
  exit 1
fi

# ==================== 2. 创建 Git 仓库 ====================
echo ""
echo "========== 2. 创建 Git 仓库 =========="

# 清理旧的测试仓库
EXISTING_REPOS=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Exec")) | .id')
for ID in $EXISTING_REPOS; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除旧仓库: $ID"
done

# 创建本地仓库
echo ""
echo "创建 Git 仓库..."
echo "  URL: file://$PLAYBOOK_PATH"
echo "  分支: master"

REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Exec Test Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")

REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id')
if [ "$REPO_ID" != "null" ] && [ -n "$REPO_ID" ]; then
  echo "✅ Git 仓库创建成功 (ID: $REPO_ID)"
else
  echo "❌ 创建 Git 仓库失败: $REPO_RESULT"
  exit 1
fi

# 同步仓库
echo ""
echo "--- 同步仓库 ---"
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2

REPO_STATUS=$(curl -s "$API_BASE/git-repos/$REPO_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
echo "✅ 仓库同步完成 (状态: $REPO_STATUS)"

# 激活仓库
echo ""
echo "--- 激活仓库 ---"
echo "  入口文件: test_ping.yml"
echo "  配置模式: manual"

ACTIVATE_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"main_playbook\": \"test_ping.yml\",
    \"config_mode\": \"manual\"
  }")

if echo "$ACTIVATE_RESULT" | jq -e '.code == 0 or ((.message // "") | test("激活成功|success|activated"; "i"))' > /dev/null 2>&1; then
  echo "✅ 仓库已激活"
else
  echo "❌ 仓库激活失败: $ACTIVATE_RESULT"
  exit 1
fi

PLAYBOOK_ID=$(select_playbook_id "$API_BASE" "$TOKEN" "$REPO_ID")

# ==================== 3. 创建执行任务模板 ====================
echo ""
echo "========== 3. 创建执行任务模板 =========="

# 清理旧的任务模板
EXISTING_TASKS=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E")) | .id')
for ID in $EXISTING_TASKS; do
  curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除旧任务: $ID"
done

# 创建任务模板
echo ""
echo "创建执行任务模板..."
echo "  目标主机: $TARGET_HOST"
echo "  执行器: local"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Ping Test\",
    \"playbook_id\": \"$PLAYBOOK_ID\",
    \"target_hosts\": \"$TARGET_HOST\",
    \"executor_type\": \"local\"
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id')
if [ "$TASK_ID" != "null" ] && [ -n "$TASK_ID" ]; then
  echo "✅ 任务模板创建成功 (ID: $TASK_ID)"
else
  echo "❌ 创建任务模板失败: $TASK_RESULT"
  exit 1
fi

# ==================== 4. 执行任务 ====================
echo ""
echo "========== 4. 执行任务 =========="
echo "密钥源 ID: $SECRETS_SOURCE_ID"
echo ""
echo "启动执行..."

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

# 等待执行完成
echo ""
echo "--- 等待执行完成 ---"
FINAL_STATUS=""
for i in {1..18}; do
  sleep 5
  RUN_STATUS=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
  echo "   [$i] 状态: $RUN_STATUS"
  
  if [ "$RUN_STATUS" == "success" ] || [ "$RUN_STATUS" == "failed" ]; then
    FINAL_STATUS=$RUN_STATUS
    break
  fi
done

# ==================== 5. 执行结果 ====================
echo ""
echo "========== 5. 执行结果 =========="

RUN_DETAIL=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
echo "$RUN_DETAIL" | jq '{
  status: .data.status,
  exit_code: .data.exit_code,
  stats: .data.stats
}'

# ==================== 6. 执行日志 ====================
echo ""
echo "========== 6. 执行日志 =========="

LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")

echo ""
echo "--- [准备阶段] ---"
echo "$LOGS" | jq -r '.data[] | select(.stage == "prepare") | "  \(.message)"'

echo ""
echo "--- [执行阶段] ---"
echo "$LOGS" | jq -r '.data[] | select(.stage == "execute") | "  \(.message)"'

echo ""
echo "--- [Ansible 输出] ---"
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=========================================="
if [ "$FINAL_STATUS" == "success" ]; then
  echo "  ✅ 执行任务测试成功！"
else
  echo "  ❌ 执行任务测试失败"
  exit 1
fi
echo "=========================================="
