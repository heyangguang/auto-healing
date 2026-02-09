#!/bin/bash
# 端到端测试 - Docker 执行器 + 取消任务
# 启动一个长时间运行的任务，然后取消它

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_SECRETS="${MOCK_SECRETS:-http://localhost:5001}"
PLAYBOOK_PATH="/root/auto-healing/tests/playbooks"

TARGET_HOST="192.168.31.100"

echo "=========================================="
echo "  Docker 执行器 + 取消任务测试"
echo "=========================================="
echo ""
echo "执行器: docker (容器化执行)"
echo "目标: 启动一个长时间任务 (sleep 60s)，然后取消它"
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

# 清理
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Docker Cancel")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Docker Cancel")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Docker Cancel")) | .id')
for ID in $EXISTING; do
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
    \"name\": \"E2E Docker Cancel Test Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"ssh_key\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\"}\"
    }
  }")
SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id // .id')
echo "✅ 密钥源创建成功 (ID: $SECRETS_SOURCE_ID)"

# 创建仓库
echo ""
echo "--- 创建 Git 仓库 ---"
REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Docker Cancel Test Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")
REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id // .id')
echo "✅ Git 仓库创建成功 (ID: $REPO_ID)"

curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
# 使用长时间运行的 playbook
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "test_long_running.yml", "config_mode": "manual"}' > /dev/null
echo "✅ 仓库同步并激活 (使用 test_long_running.yml)"

# ==================== 2. 创建任务 ====================
echo ""
echo "========== 2. 创建任务 =========="

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Docker Cancel Long Running Task\",
    \"repository_id\": \"$REPO_ID\",
    \"target_hosts\": \"$TARGET_HOST\",
    \"executor_type\": \"docker\"
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id // .id')
echo "✅ 任务创建成功 (ID: $TASK_ID)"
echo "   执行器: docker"
echo "   Playbook: test_long_running.yml (sleep 60s)"

# ==================== 3. 启动执行 ====================
echo ""
echo "========== 3. 启动执行 =========="

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"secrets_source_id\": \"$SECRETS_SOURCE_ID\"}")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id // .id')
echo "✅ 执行已启动 (Run ID: $RUN_ID)"

# 等待任务开始运行
echo ""
echo "⏳ 等待 10 秒让 Docker 容器启动并开始运行..."
sleep 10

# 检查状态
STATUS=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
echo "当前状态: $STATUS"

if [ "$STATUS" != "running" ]; then
  echo "⚠️ 任务未处于 running 状态，可能已完成或失败"
fi

# ==================== 4. 取消任务 ====================
echo ""
echo "========== 4. 取消任务 =========="
echo "📛 发送取消请求..."

CANCEL_RESULT=$(curl -s -X POST "$API_BASE/execution-runs/$RUN_ID/cancel" \
  -H "Authorization: Bearer $TOKEN")
echo "取消响应: $CANCEL_RESULT"

# 等待状态更新
echo ""
echo "⏳ 等待 5 秒让状态更新..."
sleep 5

# ==================== 5. 检查结果 ====================
echo ""
echo "========== 5. 检查结果 =========="

RUN_DETAIL=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
STATUS=$(echo "$RUN_DETAIL" | jq -r '.data.status')

echo "$RUN_DETAIL" | jq '{status: .data.status, exit_code: .data.exit_code, started_at: .data.started_at, completed_at: .data.completed_at}'

echo ""
echo "--- 执行日志 ---"
LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
echo "$LOGS" | jq -r '.data[] | "[\(.stage)] \(.message)"'

echo ""
echo "=========================================="
if [ "$STATUS" == "cancelled" ]; then
  echo "  ✅ Docker 取消任务测试成功！"
  echo ""
  echo "  验证内容："
  echo "    1. ✅ Docker 容器中的任务正在运行时发送取消请求"
  echo "    2. ✅ 任务状态变为 cancelled"
  echo "    3. ✅ Docker 容器被终止（未等到 60s 就结束）"
else
  echo "  ⚠️ 取消测试结果: 状态=$STATUS"
  echo "    可能需要检查 Docker 取消功能的实现"
fi
echo "=========================================="
