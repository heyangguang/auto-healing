#!/bin/bash
# 端到端测试 - CMDB 联动执行
# 完整流程：CMDB同步 → 获取主机 → 创建密钥源 → 创建Git仓库 → 创建任务模板 → 执行任务

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/e2e_helpers.sh"

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_ENDPOINT="${MOCK_ENDPOINT:-http://localhost:5001}"
PLAYBOOK_PATH="${PLAYBOOK_PATH:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/playbooks}"

echo "=========================================="
echo "  CMDB 联动执行端到端测试"
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

# ==================== 0. CMDB 同步（从 Mock CMDB 获取主机）====================
echo ""
echo "========== 0. CMDB 同步 =========="

# 先检查 Mock CMDB 服务是否可用
if curl -s "$MOCK_ENDPOINT/api/cmdb/hosts" > /dev/null 2>&1; then
  echo "✅ Mock CMDB 服务可用"
else
  echo "❌ Mock CMDB 服务不可用，请先启动: python3 tools/mock_secrets.py"
  exit 1
fi

# 触发 CMDB 同步
echo ""
echo "--- 触发 CMDB 同步 ---"
SYNC_RESULT=$(curl -s -X POST "$MOCK_ENDPOINT/api/cmdb/sync")
SYNCED=$(echo "$SYNC_RESULT" | jq -r '.synced_hosts // 0')
echo "✅ 同步完成: $SYNCED 台主机"

# 获取主机列表
echo ""
echo "--- 获取 CMDB 主机列表 ---"
CMDB_HOSTS=$(curl -s "$MOCK_ENDPOINT/api/cmdb/hosts")
TOTAL_HOSTS=$(echo "$CMDB_HOSTS" | jq -r '.total')
echo "CMDB 主机数量: $TOTAL_HOSTS"

echo ""
echo "主机列表:"
echo "$CMDB_HOSTS" | jq -r '.hosts[] | "  [\(.status)] \(.hostname) (\(.ip)) - \(.business)/\(.environment)"'

# 选择目标主机（从 CMDB 获取在线的 dev 环境主机）
echo ""
echo "--- 筛选目标主机（环境=dev, 状态=online）---"
FILTERED_HOSTS=$(curl -s "$MOCK_ENDPOINT/api/cmdb/hosts?environment=dev&status=online")
FILTERED_COUNT=$(echo "$FILTERED_HOSTS" | jq -r '.total')
echo "符合条件的主机: $FILTERED_COUNT 台"

if [ "$FILTERED_COUNT" -eq 0 ]; then
  echo "❌ 没有符合条件的主机"
  exit 1
fi

# 获取第一个符合条件的主机
TARGET_HOST=$(echo "$FILTERED_HOSTS" | jq -r '.hosts[0].ip')
TARGET_HOSTNAME=$(echo "$FILTERED_HOSTS" | jq -r '.hosts[0].hostname')
echo ""
echo "选中目标主机: $TARGET_HOSTNAME ($TARGET_HOST)"

# 获取主机详情
echo ""
echo "--- 获取主机详情 ---"
HOST_DETAIL=$(curl -s "$MOCK_ENDPOINT/api/cmdb/hosts/$TARGET_HOST")
echo "$HOST_DETAIL" | jq '{hostname, ip, os, business, environment, tags}'

# ==================== 1. 创建密钥源 ====================
echo ""
echo "========== 1. 创建密钥源 =========="

# 清理旧的测试密钥源
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E CMDB")) | .id')
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除旧密钥源: $ID"
done

echo ""
echo "创建密钥源..."
echo "  类型: webhook"
echo "  认证方式: password"

SECRETS_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E CMDB Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_ENDPOINT/api/secrets/query\",
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
  echo "   用户名: $USERNAME_VAL"
  echo "   密码: ${PASSWORD_VAL:0:3}***"
else
  echo "❌ 密钥源验证失败: $QUERY_RESULT"
  exit 1
fi

# ==================== 2. 创建 Git 仓库 ====================
echo ""
echo "========== 2. 创建 Git 仓库 =========="

# 清理旧的测试仓库
EXISTING_REPOS=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E CMDB")) | .id')
for ID in $EXISTING_REPOS; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除旧仓库: $ID"
done

echo ""
echo "创建 Git 仓库..."

REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E CMDB Test Repo\",
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
EXISTING_TASKS=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E CMDB")) | .id')
for ID in $EXISTING_TASKS; do
  curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除旧任务: $ID"
done

echo ""
echo "创建执行任务模板..."
echo "  目标主机（来自 CMDB）: $TARGET_HOSTNAME ($TARGET_HOST)"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E CMDB Ping Test\",
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
echo "目标主机: $TARGET_HOSTNAME ($TARGET_HOST)"
echo "密钥源: $SECRETS_SOURCE_ID"
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
  echo "  ✅ CMDB 联动执行测试成功！"
  echo ""
  echo "  完整流程验证："
  echo "    1. ✅ CMDB 同步 → 获取 $TOTAL_HOSTS 台主机"
  echo "    2. ✅ 筛选 dev 环境主机 → $TARGET_HOSTNAME"
  echo "    3. ✅ 创建密钥源 → 验证凭据获取"
  echo "    4. ✅ 创建 Git 仓库 → 同步激活"
  echo "    5. ✅ 创建任务模板 → 执行成功"
else
  echo "  ❌ CMDB 联动执行测试失败"
  exit 1
fi
echo "=========================================="
