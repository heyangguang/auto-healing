#!/bin/bash
# 混合主机通知测试 - 使用正确的 Mock Secrets 服务
# 4 台正确主机 (192.168.31.100-103) + 2 台错误主机 (192.168.31.98-99)
# 使用所有 35+ 变量的完整模板

set -e

BASE_URL="http://localhost:8080/api/v1"
MOCK_URL="http://localhost:9999"
MOCK_SECRETS="http://localhost:5001"  # mock_secrets.py

echo "=== 混合主机通知测试 (Docker + 4成功 + 2失败) ==="
echo "  MOCK Secrets: $MOCK_SECRETS"

# 登录
TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123456"}' | jq -r '.access_token')

# 清理 Mock 通知
curl -s -X POST "$MOCK_URL/notifications/clear" > /dev/null

SUFFIX=$(date +%s)

echo ""
echo "=== 1. 创建密钥源 (使用 mock_secrets.py) ==="
SSH_SOURCE_RESP=$(curl -s -X POST "$BASE_URL/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Notification Test SSH Key '$SUFFIX'",
    "type": "webhook",
    "auth_type": "ssh_key",
    "config": {
      "url": "'$MOCK_SECRETS'/api/secrets/query",
      "method": "POST",
      "body_template": "{\"hostname\": \"{hostname}\"}"
    }
  }')
SSH_SOURCE_ID=$(echo "$SSH_SOURCE_RESP" | jq -r '.data.id // .id')
echo "SSH 密钥源: $SSH_SOURCE_ID"

PWD_SOURCE_RESP=$(curl -s -X POST "$BASE_URL/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Notification Test Password '$SUFFIX'",
    "type": "webhook",
    "auth_type": "password",
    "config": {
      "url": "'$MOCK_SECRETS'/api/secrets/query",
      "method": "POST",
      "body_template": "{\"hostname\": \"{hostname}\"}"
    }
  }')
PWD_SOURCE_ID=$(echo "$PWD_SOURCE_RESP" | jq -r '.data.id // .id')
echo "密码密钥源: $PWD_SOURCE_ID"

echo ""
echo "=== 2. 创建渠道 ==="
CHANNEL_ID=$(curl -s -X POST "$BASE_URL/channels" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Mixed Notification Channel '$SUFFIX'", "type": "webhook", "config": {"url": "'$MOCK_URL'/webhook"}, "default_recipients": ["sre@example.com"]}' | jq -r '.id')
echo "渠道: $CHANNEL_ID"

echo ""
echo "=== 3. 创建模板（使用所有 40 个变量）==="
TEMPLATE_RESP=$(curl -s -X POST "$BASE_URL/templates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Mixed Full Template $SUFFIX\",
    \"description\": \"完整变量模板 - Docker混合认证测试 - 包含35+变量\",
    \"event_type\": \"execution_result\",
    \"subject_template\": \"[Auto-Healing] {{execution.status_emoji}} {{task.name}} - {{execution.status}}\",
    \"body_template\": \"# 执行结果通知\\n\\n## 基本信息\\n- **时间戳**: {{timestamp}}\\n- **日期**: {{date}}\\n- **时间**: {{time}}\\n- **触发者**: {{execution.triggered_by}}\\n- **触发类型**: {{execution.trigger_type}}\\n\\n## 执行信息\\n- **Run ID**: {{execution.run_id}}\\n- **状态**: {{execution.status}} {{execution.status_emoji}}\\n- **退出码**: {{execution.exit_code}}\\n- **开始时间**: {{execution.started_at}}\\n- **完成时间**: {{execution.completed_at}}\\n- **执行时长**: {{execution.duration}}\\n- **时长(秒)**: {{execution.duration_seconds}}\\n\\n## 任务信息\\n- **任务ID**: {{task.id}}\\n- **任务名称**: {{task.name}}\\n- **目标主机**: {{task.target_hosts}}\\n- **主机数量**: {{task.host_count}}\\n- **执行器**: {{task.executor_type}}\\n- **是否定时**: {{task.is_recurring}}\\n\\n## 仓库信息\\n- **仓库ID**: {{repository.id}}\\n- **仓库名称**: {{repository.name}}\\n- **仓库URL**: {{repository.url}}\\n- **主 Playbook**: {{repository.main_playbook}}\\n- **分支**: {{repository.branch}}\\n\\n## 执行统计\\n| 指标 | 数量 |\\n|------|------|\\n| OK | {{stats.ok}} |\\n| Changed | {{stats.changed}} |\\n| Failed | {{stats.failed}} |\\n| Unreachable | {{stats.unreachable}} |\\n| Skipped | {{stats.skipped}} |\\n| Rescued | {{stats.rescued}} |\\n| Ignored | {{stats.ignored}} |\\n| **总计** | {{stats.total}} |\\n| **成功率** | {{stats.success_rate}} |\\n\\n## 系统信息\\n- **系统名称**: {{system.name}}\\n- **版本**: {{system.version}}\\n- **环境**: {{system.env}}\\n\\n## 错误信息\\n{{error.message}}\\n{{error.host}}\\n\\n## Ansible 执行日志\\n\`\`\`\\n{{execution.stdout}}\\n\`\`\`\\n\\n## 错误输出\\n\`\`\`\\n{{execution.stderr}}\\n\`\`\`\",
    \"format\": \"markdown\",
    \"supported_channels\": [\"webhook\", \"dingtalk\", \"email\"]
  }")
TEMPLATE_ID=$(echo "$TEMPLATE_RESP" | jq -r '.id')
VARIABLE_COUNT=$(echo "$TEMPLATE_RESP" | jq '.available_variables | length')
echo "模板 ID: $TEMPLATE_ID"
echo "变量数量: $VARIABLE_COUNT 个"

echo ""
echo "--- 创建模板的完整响应 ---"
echo "$TEMPLATE_RESP" | jq '{id, name, description, event_type, subject_template, format, supported_channels}'

echo ""
echo "--- 模板正文 (body_template) ---"
echo "$TEMPLATE_RESP" | jq -r '.body_template'

echo ""
echo "--- 模板中使用的全部 $VARIABLE_COUNT 个变量 ---"
echo "$TEMPLATE_RESP" | jq -r '.available_variables | to_entries | .[] | "\(.key + 1). \(.value)"'

# 查找仓库
echo ""
echo "=== 4. 获取仓库 ==="
REPO_ID=$(curl -s "$BASE_URL/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '(.items // .data)[] | select(.is_active == true and .main_playbook == "test_ping.yml") | .id' | head -1)
echo "仓库: $REPO_ID"

echo ""
echo "=== 5. 创建任务 (6主机: 4正确 + 2错误) ==="
TARGET_HOSTS="192.168.31.100,192.168.31.101,192.168.31.102,192.168.31.103,192.168.31.98,192.168.31.99"
TASK_RESP=$(curl -s -X POST "$BASE_URL/execution-tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Mixed Notification Test '$SUFFIX'",
    "repository_id": "'$REPO_ID'",
    "target_hosts": "'$TARGET_HOSTS'",
    "executor_type": "docker",
    "notification_config": {
      "enabled": true,
      "on_success": true,
      "on_failure": true,
      "template_id": "'$TEMPLATE_ID'",
      "channel_ids": ["'$CHANNEL_ID'"],
      "extra_recipients": ["oncall@example.com"]
    }
  }')
TASK_ID=$(echo "$TASK_RESP" | jq -r '.id // .data.id')
echo "任务: $TASK_ID"
echo "主机:"
echo "  ✓ 192.168.31.100-103 (有密钥 - 应成功)"
echo "  ✗ 192.168.31.98-99 (不可达 - 应失败)"

echo ""
echo "=== 6. 执行任务 ==="
EXEC_RESP=$(curl -s -X POST "$BASE_URL/execution-tasks/$TASK_ID/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"triggered_by": "mixed_notification_test", "secrets_source_ids": ["'$SSH_SOURCE_ID'", "'$PWD_SOURCE_ID'"]}')
RUN_ID=$(echo "$EXEC_RESP" | jq -r '.id // .data.id')
echo "执行 ID: $RUN_ID"

echo ""
echo "=== 7. 等待执行完成 ==="
for i in $(seq 1 90); do
  STATUS=$(curl -s "$BASE_URL/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.status // .data.status')
  printf "\r  状态: %-12s (%ds)" "$STATUS" "$i"
  if [ "$STATUS" == "success" ] || [ "$STATUS" == "failed" ] || [ "$STATUS" == "partial_success" ]; then
    echo ""
    break
  fi
  sleep 2
done

echo ""
echo "=== 8. 执行结果 ==="
RUN_RESULT=$(curl -s "$BASE_URL/execution-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")
echo "$RUN_RESULT" | jq '{status, exit_code, stats}'

echo ""
echo "=== 9. Ansible 输出 ==="
curl -s "$BASE_URL/execution-runs/$RUN_ID/logs" -H "Authorization: Bearer $TOKEN" | jq -r '(.data // .)[] | select(.stage == "output") | .details.stdout // empty'

echo ""
echo "=== 10. 收到的通知 ==="
sleep 2
MOCK_RESP=$(curl -s "$MOCK_URL/notifications")
echo "通知数: $(echo "$MOCK_RESP" | jq '.total')"

echo ""
echo "--- 通知主题 ---"
echo "$MOCK_RESP" | jq -r '.notifications[-1].body.subject'

echo ""
echo "--- 通知正文（35+ 变量已替换）---"
echo "$MOCK_RESP" | jq -r '.notifications[-1].body.body'

echo ""
echo "=== 测试完成 ==="
