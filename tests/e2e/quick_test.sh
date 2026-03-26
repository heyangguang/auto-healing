#!/bin/bash
set -euo pipefail
URL="http://localhost:8080/api/v1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/e2e_helpers.sh"

# 登录（保持原格式）
TOKEN=$(curl -s -X POST "$URL/auth/login" -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token // empty')
echo "✅ 登录成功"

PLAYBOOK_ID=$(select_first_ready_playbook "$URL" "$TOKEN")

# 创建任务（新格式：data 包装）
TASK=$(curl -s -X POST "$URL/execution-tasks" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"E2E测试任务","playbook_id":"'"$PLAYBOOK_ID"'","target_hosts":"localhost","extra_vars":{"target_host":"localhost"},"executor_type":"local"}')
TASK_ID=$(echo $TASK | jq -r '.data.id')
TASK_NAME=$(echo $TASK | jq -r '.data.name')
echo "✅ 创建任务: $TASK_ID ($TASK_NAME)"

# 执行任务（新格式：data 包装）
RUN=$(curl -s -X POST "$URL/execution-tasks/$TASK_ID/execute" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"triggered_by":"e2e-test"}')
RUN_ID=$(echo $RUN | jq -r '.data.id')
RUN_STATUS=$(echo $RUN | jq -r '.data.status')
RUN_EXIT=$(echo $RUN | jq -r '.data.exit_code')
echo "✅ 执行任务: $RUN_ID | 状态=$RUN_STATUS | 退出码=$RUN_EXIT"

# 获取执行历史
RUNS=$(curl -s "$URL/execution-tasks/$TASK_ID/runs" -H "Authorization: Bearer $TOKEN")
RUNS_TOTAL=$(echo $RUNS | jq -r '.total')
echo "✅ 执行历史: $RUNS_TOTAL 条记录"

# 获取执行日志（新格式：data 包装）
LOGS=$(curl -s "$URL/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN")
LOGS_COUNT=$(echo $LOGS | jq '.data | length')
echo "✅ 执行日志: $LOGS_COUNT 条"

# 显示日志内容
echo "   日志详情:"
echo $LOGS | jq -r '.data[] | "   [\(.stage)] \(.message)"'

# 删除任务
DEL=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$URL/execution-tasks/$TASK_ID" -H "Authorization: Bearer $TOKEN")
echo "✅ 删除任务: HTTP $DEL"

echo ""
echo "🎉 所有测试通过！"
