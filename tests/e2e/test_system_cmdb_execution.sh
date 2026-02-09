#!/bin/bash
# 端到端测试 - 使用系统 CMDB 执行（非定时）
# 从系统 CMDB API 获取主机 → 创建密钥源 → Git 仓库 → 任务模板 → 执行

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
MOCK_SECRETS="${MOCK_SECRETS:-http://localhost:5001}"
PLAYBOOK_PATH="/root/auto-healing/tests/playbooks"

echo "=========================================="
echo "  系统 CMDB 联动执行测试"
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

# ==================== 0. 从系统 CMDB 获取主机 ====================
echo ""
echo "========== 0. 系统 CMDB 查询 =========="

# 查询系统 CMDB 中的主机（筛选 development 环境）
echo "查询系统 CMDB（environment=development）..."
CMDB_RESULT=$(curl -s "$API_BASE/cmdb?environment=development" -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$CMDB_RESULT" | jq -r '.total // 0')
echo "找到 $TOTAL 台主机"

# 显示主机列表
echo ""
echo "--- 系统 CMDB 主机列表 ---"
echo "$CMDB_RESULT" | jq -r '.data[] | "  [\(.status)] \(.name) (\(.ip_address)) - 来源: \(.source_plugin_name)"' | head -10

# 筛选 Real Hosts CMDB 的主机
echo ""
echo "--- 筛选 Real Hosts CMDB 的主机 ---"
REAL_HOST=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB") | {id, name, ip_address, status, environment}' | head -20)

if [ -z "$REAL_HOST" ]; then
  echo "❌ 没有找到 Real Hosts CMDB 的主机"
  exit 1
fi

echo "$REAL_HOST" | jq '.'

# 提取主机信息
TARGET_HOST=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB") | .ip_address' | head -1)
TARGET_NAME=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB") | .name' | head -1)
TARGET_ID=$(echo "$CMDB_RESULT" | jq -r '.data[] | select(.source_plugin_name == "Real Hosts CMDB") | .id' | head -1)

echo ""
echo "✅ 选中目标主机：$TARGET_NAME ($TARGET_HOST)"
echo "   CMDB ID: $TARGET_ID"

# ==================== 1. 创建密钥源 ====================
echo ""
echo "========== 1. 创建密钥源 =========="

# 清理旧数据
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E System CMDB")) | .id')
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

echo "创建密钥源..."
SECRETS_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E System CMDB Password Source\",
    \"type\": \"webhook\",
    \"auth_type\": \"password\",
    \"config\": {
      \"url\": \"$MOCK_SECRETS/api/secrets/query\",
      \"method\": \"POST\",
      \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
    }
  }")

SECRETS_SOURCE_ID=$(echo "$SECRETS_RESULT" | jq -r '.data.id // .id')
echo "✅ 密钥源创建成功 (ID: $SECRETS_SOURCE_ID)"

# 验证密钥源
echo ""
echo "--- 验证密钥源 ---"
QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"hostname\":\"$TARGET_HOST\",\"source_id\":\"$SECRETS_SOURCE_ID\"}")

AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type // .auth_type')
if [ "$AUTH_TYPE" == "password" ]; then
  echo "✅ 密钥源验证成功"
else
  echo "❌ 密钥源验证失败"
  exit 1
fi

# ==================== 2. 创建 Git 仓库 ====================
echo ""
echo "========== 2. 创建 Git 仓库 =========="

# 清理
EXISTING_REPOS=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E System CMDB")) | .id')
for ID in $EXISTING_REPOS; do
  curl -s -X POST "$API_BASE/git-repos/$ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

REPO_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E System CMDB Test Repo\",
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

# ==================== 3. 创建任务模板 ====================
echo ""
echo "========== 3. 创建任务模板 =========="

EXISTING_TASKS=$(curl -s "$API_BASE/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E System CMDB")) | .id')
for ID in $EXISTING_TASKS; do
  curl -s -X DELETE "$API_BASE/execution-tasks/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

echo "创建任务模板..."
echo "  目标主机(来自系统 CMDB): $TARGET_NAME ($TARGET_HOST)"

TASK_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E System CMDB Ping Test\",
    \"repository_id\": \"$REPO_ID\",
    \"target_hosts\": \"$TARGET_HOST\",
    \"executor_type\": \"local\"
  }")

TASK_ID=$(echo "$TASK_RESULT" | jq -r '.data.id // .id')
echo "✅ 任务模板创建成功 (ID: $TASK_ID)"

# ==================== 4. 执行任务 ====================
echo ""
echo "========== 4. 执行任务 =========="

echo "目标主机(系统 CMDB): $TARGET_NAME ($TARGET_HOST)"
echo "密钥源: $SECRETS_SOURCE_ID"
echo ""

EXEC_RESULT=$(curl -s -X POST "$API_BASE/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"secrets_source_id\": \"$SECRETS_SOURCE_ID\"}")

RUN_ID=$(echo "$EXEC_RESULT" | jq -r '.data.id // .id')
echo "✅ 执行已启动 (Run ID: $RUN_ID)"

# 等待完成
echo ""
echo "--- 等待执行完成 ---"
for i in {1..18}; do
  sleep 5
  STATUS=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
  echo "   [$i] 状态: $STATUS"
  if [ "$STATUS" == "success" ] || [ "$STATUS" == "failed" ]; then
    break
  fi
done

# ==================== 5. 执行结果 ====================
echo ""
echo "========== 5. 执行结果 =========="

RUN_DETAIL=$(curl -s "$API_BASE/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
echo "$RUN_DETAIL" | jq '{status: .data.status, exit_code: .data.exit_code, stats: .data.stats}'

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
FINAL_STATUS=$(echo "$RUN_DETAIL" | jq -r '.data.status')
if [ "$FINAL_STATUS" == "success" ]; then
  echo "  ✅ 系统 CMDB 联动执行测试成功！"
  echo ""
  echo "  完整流程验证："
  echo "    1. ✅ 系统 CMDB 查询 → 找到 $TARGET_NAME"
  echo "    2. ✅ 主机来源: Real Hosts CMDB 插件"
  echo "    3. ✅ 创建密钥源 → 验证凭据"
  echo "    4. ✅ 执行成功"
else
  echo "  ❌ 测试失败"
fi
echo "=========================================="
