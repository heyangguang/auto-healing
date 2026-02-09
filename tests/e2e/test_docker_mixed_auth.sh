#!/bin/bash
# 端到端测试 - Docker 执行器 + 混合认证 + 错误主机
# 与 local 执行器相同的场景，但使用 Docker 执行器

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_SECRETS="${MOCK_SECRETS:-http://localhost:5001}"
PLAYBOOK_PATH="/root/auto-healing/tests/playbooks"

# 混合认证 + 错误主机
TARGET_HOSTS="192.168.31.100,192.168.31.101,192.168.31.102,192.168.31.103,192.168.31.98,192.168.31.99"

echo "=========================================="
echo "  Docker 执行器 + 混合认证 + 错误主机测试"
echo "=========================================="
echo ""
echo "执行器: docker (容器化执行)"
echo "目标主机: $TARGET_HOSTS"
echo "  - 192.168.31.100-102: SSH 密钥 (正确)"
echo "  - 192.168.31.103: 密码 (正确)"
echo "  - 192.168.31.98-99: 不可达 (错误)"
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
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Docker")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Docker")) | .id')
for ID in $EXISTING; do curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null; done

EXISTING=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Docker")) | .id')
for ID in $EXISTING; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done
echo "✅ 旧数据已清理"

# 创建两个密钥源
echo ""
echo "--- 创建密钥源 1 (SSH Key) ---"
SECRETS_RESULT_SSH=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Docker SSH Key Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"ssh_key\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\"}\"
    }
  }")
SSH_SOURCE_ID=$(echo "$SECRETS_RESULT_SSH" | jq -r '.data.id // .id')
echo "✅ SSH 密钥源创建成功 (ID: $SSH_SOURCE_ID)"

echo ""
echo "--- 创建密钥源 2 (Password) ---"
SECRETS_RESULT_PWD=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Docker Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\"}\"
    }
  }")
PWD_SOURCE_ID=$(echo "$SECRETS_RESULT_PWD" | jq -r '.data.id // .id')
echo "✅ 密码密钥源创建成功 (ID: $PWD_SOURCE_ID)"

# 创建仓库
echo ""
echo "--- 创建 Git 仓库 ---"
REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Docker Test Repo\",
    \"url\": \"file://$PLAYBOOK_PATH\",
    \"default_branch\": \"master\"
  }")
REPO_ID=$(echo "$REPO_RESULT" | jq -r '.data.id // .id')
echo "✅ Git 仓库创建成功 (ID: $REPO_ID)"

curl -s -X POST "$API_BASE/git-repos/$REPO_ID/sync" -H "Authorization: Bearer $TOKEN" > /dev/null
sleep 2
curl -s -X POST "$API_BASE/git-repos/$REPO_ID/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "test_ping.yml", "config_mode": "manual"}' > /dev/null
echo "✅ 仓库同步并激活"

# ==================== 2. 创建任务 ====================
echo ""
echo "========== 2. 创建任务 =========="

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Docker Mixed Auth Test\",
    \"repository_id\": \"$REPO_ID\",
    \"target_hosts\": \"$TARGET_HOSTS\",
    \"executor_type\": \"docker\"
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id // .id')
echo "✅ 任务创建成功 (ID: $TASK_ID)"
echo "   执行器: docker"
echo "   目标主机: $TARGET_HOSTS"
echo "   预期: 4台成功, 2台失败(unreachable)"

# ==================== 3. 执行任务 ====================
echo ""
echo "========== 3. 执行任务 =========="

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"secrets_source_ids\": [\"$SSH_SOURCE_ID\", \"$PWD_SOURCE_ID\"]}")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id // .id')
echo "✅ 执行已启动 (Run ID: $RUN_ID)"

# SSE 实时日志
echo ""
echo "--- SSE 实时日志流 ---"
timeout 120s curl -s -N "$API_BASE/execution-runs/$RUN_ID/stream" \
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
UNREACHABLE=$(echo "$RUN_DETAIL" | jq -r '.data.stats.unreachable')
OK=$(echo "$RUN_DETAIL" | jq -r '.data.stats.ok')

echo "$RUN_DETAIL" | jq '{status: .data.status, exit_code: .data.exit_code, stats: .data.stats}'

echo ""
echo "--- Ansible 输出 ---"
LOGS=$(curl -s "$API_BASE/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
echo "$LOGS" | jq -r '.data[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=========================================="
echo "  Docker 执行器测试结果分析"
echo "=========================================="
echo ""
echo "  预期结果："
echo "    - 状态: failed (因为有 unreachable 主机)"
echo "    - ok: 12 (4台成功 × 3任务)"
echo "    - unreachable: 2 (98, 99)"
echo ""
echo "  实际结果："
echo "    - 状态: $STATUS"
echo "    - ok: $OK"
echo "    - unreachable: $UNREACHABLE"
echo ""

if [ "$UNREACHABLE" == "2" ] && [ "$OK" == "12" ]; then
  echo "  ✅ Docker 执行器测试成功！"
  echo "    - 使用 Docker 容器执行 Ansible"
  echo "    - 混合认证正常工作"
  echo "    - 错误主机正确报告 unreachable"
else
  echo "  ⚠️ 结果与预期不完全匹配，请检查"
fi
echo "=========================================="
