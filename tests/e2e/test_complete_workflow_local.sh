#!/bin/bash

# ================================================================
# 自愈引擎完整端到端测试 - 严格版本
# 
# 每一步都会：
# 1. 显示完整的 API 请求和响应
# 2. 验证响应是否成功
# 3. 只有成功才继续下一步
# ================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
API_BASE="${API_BASE:-http://localhost:8080}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

TEST_ID="E2E-$(date +%Y%m%d%H%M%S)"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║            自愈引擎 - 完整端到端测试                                   ║"
echo "╠════════════════════════════════════════════════════════════════════════╣"
echo "║  测试ID: $TEST_ID"
echo "║  API: $API_BASE"
echo "╚════════════════════════════════════════════════════════════════════════╝"
echo ""

# 通用函数：检查 API 响应
check_response() {
    local resp="$1"
    local step_name="$2"
    
    # 检查是否是有效 JSON
    if ! echo "$resp" | jq . > /dev/null 2>&1; then
        echo "  ❌ $step_name 失败: 响应不是有效 JSON"
        echo "  响应: $resp"
        exit 1
    fi
    
    # 检查是否有错误
    local code=$(echo "$resp" | jq -r '.code // 0')
    local error=$(echo "$resp" | jq -r '.error // empty')
    
    if [ "$code" != "0" ] && [ -n "$error" ]; then
        echo "  ❌ $step_name 失败: $error"
        echo "  完整响应:"
        echo "$resp" | jq .
        exit 1
    fi
}

# ============================================================================
# 步骤 1: 登录
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 1/9: 登录系统                                                     ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  请求: POST $API_BASE/api/v1/auth/login"
echo "  参数: {username: $USERNAME, password: ******}"
echo ""

LOGIN_RESP=$(curl -s -X POST "$API_BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}")

echo "  响应:"
echo "$LOGIN_RESP" | jq '{access_token: .access_token[0:50], user: {username: .user.username, roles: .user.roles}}'
echo ""

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.access_token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "  ❌ 登录失败"
    exit 1
fi
echo "  ✅ 登录成功"
echo ""

# ============================================================================
# 步骤 1.5: Clean Slate - 禁用冲突的旧规则
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ Clean Slate: 禁用冲突的旧规则                                          ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  查找并禁用所有匹配 'E2E-HEALING' 的活跃规则..."
echo ""

# 获取所有规则
ALL_RULES=$(curl -s -X GET "$API_BASE/api/v1/healing/rules?page=1&page_size=100" \
  -H "Authorization: Bearer $TOKEN")

# 找出所有匹配 E2E-HEALING 且处于活跃状态的规则
CONFLICTING_RULES=$(echo "$ALL_RULES" | jq -r '.data[] | select(.is_active == true) | select(.conditions[].value | contains("E2E-HEALING")) | .id')

DISABLED_COUNT=0
for RULE_ID in $CONFLICTING_RULES; do
    echo "    禁用规则 ID: $RULE_ID"
    curl -s -X POST "$API_BASE/api/v1/healing/rules/$RULE_ID/deactivate" \
      -H "Authorization: Bearer $TOKEN" > /dev/null
    DISABLED_COUNT=$((DISABLED_COUNT + 1))
done

if [ "$DISABLED_COUNT" -gt 0 ]; then
    echo ""
    echo "  ✅ 已禁用 $DISABLED_COUNT 个冲突规则"
else
    echo "  ✅ 无冲突规则需要禁用"
fi
echo ""

# ============================================================================
# 步骤 2: 创建 ITSM 插件
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 2/9: 创建 ITSM 插件                                               ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  请求: POST $API_BASE/api/v1/plugins"
echo "  参数:"
echo "    name: E2E-ITSM-$TEST_ID"
echo "    type: itsm"
echo "    adapter: servicenow"
echo "    config.endpoint: http://localhost:5000"
echo ""

ITSM_PLUGIN_RESP=$(curl -s -X POST "$API_BASE/api/v1/plugins" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "E2E-ITSM-'$TEST_ID'",
    "type": "itsm",
    "adapter": "servicenow",
    "sync_enabled": true,
    "sync_interval_minutes": 5,
    "config": {
      "endpoint": "http://localhost:5000",
      "username": "admin",
      "password": "admin"
    }
  }')

echo "  响应:"
echo "$ITSM_PLUGIN_RESP" | jq .
echo ""

ITSM_PLUGIN_ID=$(echo "$ITSM_PLUGIN_RESP" | jq -r '.data.id')
if [ -z "$ITSM_PLUGIN_ID" ] || [ "$ITSM_PLUGIN_ID" = "null" ]; then
    echo "  ❌ 创建 ITSM 插件失败"
    exit 1
fi
echo "  ✅ 创建成功，插件ID: $ITSM_PLUGIN_ID"
echo ""

# ============================================================================
# 步骤 3: 创建 CMDB 插件
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 3/9: 创建 CMDB 插件                                               ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  请求: POST $API_BASE/api/v1/plugins"
echo "  参数:"
echo "    name: E2E-CMDB-$TEST_ID"
echo "    type: cmdb"
echo "    adapter: servicenow"
echo "    config.endpoint: http://localhost:5001"
echo ""

CMDB_PLUGIN_RESP=$(curl -s -X POST "$API_BASE/api/v1/plugins" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "E2E-CMDB-'$TEST_ID'",
    "type": "cmdb",
    "adapter": "servicenow",
    "sync_enabled": true,
    "sync_interval_minutes": 60,
    "config": {
      "endpoint": "http://localhost:5001",
      "username": "admin",
      "password": "admin"
    }
  }')

echo "  响应:"
echo "$CMDB_PLUGIN_RESP" | jq .
echo ""

CMDB_PLUGIN_ID=$(echo "$CMDB_PLUGIN_RESP" | jq -r '.data.id')
if [ -z "$CMDB_PLUGIN_ID" ] || [ "$CMDB_PLUGIN_ID" = "null" ]; then
    echo "  ❌ 创建 CMDB 插件失败"
    exit 1
fi
echo "  ✅ 创建成功，插件ID: $CMDB_PLUGIN_ID"
echo ""

# ============================================================================
# 步骤 4: 创建密钥来源
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 4/9: 创建密钥来源                                                 ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  请求: POST $API_BASE/api/v1/secrets-sources"
echo "  参数:"
echo "    name: E2E-Secrets-$TEST_ID"
echo "    type: webhook"
echo "    webhook_url: http://localhost:5002"
echo ""

SECRETS_RESP=$(curl -s -X POST "$API_BASE/api/v1/secrets-sources" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "E2E-Secrets-'$TEST_ID'",
    "type": "webhook",
    "auth_type": "password",
    "config": {
      "url": "http://localhost:5002/api/v1/secrets/query"
    }
  }')


echo "  响应:"
echo "$SECRETS_RESP" | jq .
echo ""

SECRETS_SOURCE_ID=$(echo "$SECRETS_RESP" | jq -r '.data.id')
if [ -z "$SECRETS_SOURCE_ID" ] || [ "$SECRETS_SOURCE_ID" = "null" ]; then
    echo "  ❌ 创建密钥来源失败: $SECRETS_RESP"
    exit 1
fi
echo "  ✅ 密钥来源ID: $SECRETS_SOURCE_ID"
echo ""

# ============================================================================
# 步骤 4.5: 创建通知渠道和模板
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 4.5: 创建通知渠道和模板                                           ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""

# 创建通知渠道
echo "  [4.5a] 创建通知渠道"
echo "  请求: POST $API_BASE/api/v1/channels"
echo "  参数:"
echo "    name: E2E-Channel-$TEST_ID"
echo "    type: webhook"
echo "    webhook_url: http://localhost:9999/webhook"
echo ""

CHANNEL_RESP=$(curl -s -X POST "$API_BASE/api/v1/channels" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "E2E-Channel-'$TEST_ID'",
    "type": "webhook",
    "description": "E2E 测试通知渠道",
    "config": {
      "url": "http://localhost:9999/webhook",
      "method": "POST",
      "timeout_seconds": 30
    },
    "recipients": ["admin@example.com"]
  }')

echo "  响应:"
echo "$CHANNEL_RESP" | jq .
echo ""

CHANNEL_ID=$(echo "$CHANNEL_RESP" | jq -r '.data.id // empty')
if [ -z "$CHANNEL_ID" ]; then
    echo "  ❌ 创建渠道失败: $CHANNEL_RESP"
    exit 1
fi
echo "  ✅ 通知渠道ID: $CHANNEL_ID"
echo ""

# 创建通知模板 (包含所有 40 个变量)
echo "  [4.5b] 创建通知模板 (包含 40 个变量)"
echo "  请求: POST $API_BASE/api/v1/templates"
echo "  参数:"
echo "    name: E2E-Template-$TEST_ID"
echo "    event_type: flow_result"
echo "    format: markdown"
echo "    supported_channels: [webhook, dingtalk, email]"
echo ""

TEMPLATE_RESP=$(curl -s -X POST "$API_BASE/api/v1/templates" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "E2E-Template-'$TEST_ID'",
    "description": "E2E 测试通知模板 - 包含所有变量",
    "event_type": "flow_result",
    "subject_template": "[Auto-Healing] {{execution_status_emoji}} 流程 #{{flow_instance_id}} - {{execution_status}}",
    "body_template": "# 自愈流程执行结果\n\n## 基本信息\n- 时间: {{timestamp}}\n- 日期: {{date}}\n- 时间: {{time}}\n\n## 流程信息\n- 实例ID: {{flow_instance_id}}\n- 状态: {{flow_status}}\n\n## 系统信息\n- 系统名称: {{system_name}}\n- 版本: {{system_version}}\n- 环境: {{system_env}}\n\n## 工单信息\n- 工单ID: {{incident_id}}\n- 标题: {{incident_title}}\n- 严重程度: {{incident_severity}}\n- 来源: {{incident_source}}\n- 外部ID: {{incident_external_id}}\n- 状态: {{incident_status}}\n\n## 执行结果\n- 状态: {{execution_status}} {{execution_status_emoji}}\n- 消息: {{execution_message}}\n- 退出码: {{execution_exit_code}}\n- 耗时: {{execution_duration_ms}} ms\n- Playbook: {{execution_playbook_path}}\n\n## 统计信息\n| OK | Changed | Failed | Unreachable | Skipped | Total | 成功率 |\n|----|---------|--------|-------------|---------|-------|---------|\n| {{stats.ok}} | {{stats.changed}} | {{stats.failed}} | {{stats.unreachable}} | {{stats.skipped}} | {{stats.total}} | {{stats.success_rate}} |\n\n## 验证摘要\n- 总数: {{validation.total}}\n- 匹配: {{validation.matched}}\n- 不匹配: {{validation.unmatched}}\n\n## 主机信息\n- 目标主机: {{target_hosts}}\n- 主机数量: {{host_count}}\n\n## Ansible 输出\n```\n{{execution_stdout}}\n```\n\n## 错误输出\n```\n{{execution_stderr}}\n```",
    "format": "markdown",
    "supported_channels": ["webhook", "dingtalk", "email"]
  }')

echo "  响应:"
echo "$TEMPLATE_RESP" | jq .
echo ""

TEMPLATE_ID=$(echo "$TEMPLATE_RESP" | jq -r '.data.id // empty')
if [ -z "$TEMPLATE_ID" ]; then
    echo "  ❌ 创建模板失败: $TEMPLATE_RESP"
    exit 1
fi
echo "  ✅ 通知模板ID: ${TEMPLATE_ID:-N/A}"
echo ""

# ============================================================================
# 步骤 5: 创建 Git 仓库
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 5/9: 创建 Git 仓库                                                ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  请求: POST $API_BASE/api/v1/git-repos"
echo "  参数:"
echo "    name: E2E-Playbook-$TEST_ID"
echo "    url: ${REPO_ROOT}/test-playbook-repo"
echo ""

GIT_REPO_RESP=$(curl -s -X POST "$API_BASE/api/v1/git-repos" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "E2E-Playbook-'$TEST_ID'",
    "url": "http://localhost:3000/testadmin/healing-playbooks.git",
    "branch": "main",
    "main_playbook": "e2e-healing.yml",
    "is_active": true
  }')

echo "  响应:"
echo "$GIT_REPO_RESP" | jq .
echo ""

GIT_REPO_ID=$(echo "$GIT_REPO_RESP" | jq -r '.data.id // empty')
if [ -z "$GIT_REPO_ID" ] || [ "$GIT_REPO_ID" = "null" ]; then
    echo "  ❌ 创建 Git 仓库失败: $GIT_REPO_RESP"
    exit 1
fi
echo "  ✅ Git仓库ID: $GIT_REPO_ID"

# 同步仓库（将内容 clone 到 local_path）
echo "  正在同步仓库..."
SYNC_RESP=$(curl -s -X POST "$API_BASE/api/v1/git-repos/$GIT_REPO_ID/sync" \
  -H "Authorization: Bearer $TOKEN")
echo "  同步结果: $(echo "$SYNC_RESP" | jq -r '.message // .code')"

# 激活仓库（设置 main_playbook）
echo "  正在激活仓库..."
ACTIVATE_RESP=$(curl -s -X POST "$API_BASE/api/v1/git-repos/$GIT_REPO_ID/activate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "main_playbook": "e2e-healing.yml",
    "config_mode": "auto"
  }')
echo "  激活结果: $(echo "$ACTIVATE_RESP" | jq -r '.message // .code')"
if ! echo "$ACTIVATE_RESP" | jq -e '.code == 0 or ((.message // "") | test("成功|success|activated"; "i"))' > /dev/null 2>&1; then
    echo "  ❌ Git 仓库激活失败"
    exit 1
fi
echo ""

# ============================================================================
# 步骤 6: 创建自愈流程
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 6/9: 创建自愈流程                                                 ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  流程节点: start -> host_extractor -> cmdb_validator -> [approval] -> execution -> notification -> end"
echo "  (包含审批节点，流程将暂停等待人工审批)"
echo ""

FLOW_NAME="E2E流程-$TEST_ID"
FLOW_RESP=$(curl -s -X POST "$API_BASE/api/v1/healing/flows" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "'"$FLOW_NAME"'",
    "description": "端到端测试流程（含审批节点）",
    "is_active": true,
    "nodes": [
      {"id": "start_1", "type": "start", "config": {}},
      {"id": "host_extractor_1", "type": "host_extractor", "config": {"source_field": "raw_data.cmdb_ci", "extract_mode": "split", "split_by": ",", "output_key": "hosts"}},
      {"id": "cmdb_validator_1", "type": "cmdb_validator", "config": {"input_key": "hosts", "output_key": "validated_hosts"}},
      {"id": "approval_1", "type": "approval", "config": {"title": "E2E 自愈执行审批", "description": "请确认是否执行自愈操作", "timeout_hours": 1, "approvers": ["admin"], "approver_roles": ["admin"]}},
      {"id": "execution_1", "type": "execution", "config": {"git_repo_id": "'$GIT_REPO_ID'", "executor_type": "local", "hosts_key": "validated_hosts", "secrets_source_id": "'$SECRETS_SOURCE_ID'", "extra_vars": {"service_name": "nginx", "service_action": "check"}}},
      {"id": "notification_1", "type": "notification", "config": {"channel_ids": ["'$CHANNEL_ID'"], "template_id": "'$TEMPLATE_ID'"}},
      {"id": "end_1", "type": "end", "config": {}}
    ],
    "edges": [
      {"source": "start_1", "target": "host_extractor_1"},
      {"source": "host_extractor_1", "target": "cmdb_validator_1"},
      {"source": "cmdb_validator_1", "target": "approval_1"},
      {"source": "approval_1", "target": "execution_1"},
      {"source": "execution_1", "target": "notification_1"},
      {"source": "notification_1", "target": "end_1"}
    ]
  }')

echo "  响应:"
echo "$FLOW_RESP" | jq '{code, message, data: {id: .data.id, name: .data.name, nodes_count: (.data.nodes | length)}}'
echo ""

FLOW_ID=$(echo "$FLOW_RESP" | jq -r '.data.id')
if [ -z "$FLOW_ID" ] || [ "$FLOW_ID" = "null" ]; then
    echo "  ❌ 创建流程失败"
    echo "$FLOW_RESP" | jq .
    exit 1
fi
echo "  ✅ 创建成功，流程ID: $FLOW_ID"
echo ""

# ============================================================================
# 步骤 7: 创建匹配规则
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 7/9: 创建匹配规则                                                 ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  规则: 工单标题包含 'E2E-HEALING' -> 触发流程 $FLOW_ID"
echo ""

RULE_RESP=$(curl -s -X POST "$API_BASE/api/v1/healing/rules" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"name\": \"E2E规则-$TEST_ID\",
    \"priority\": 100,
    \"trigger_mode\": \"auto\",
    \"match_mode\": \"all\",
    \"conditions\": [{\"field\": \"title\", \"operator\": \"contains\", \"value\": \"E2E-HEALING\"}],
    \"flow_id\": \"$FLOW_ID\",
    \"is_active\": true
  }")

echo "  响应:"
echo "$RULE_RESP" | jq .
echo ""

RULE_ID=$(echo "$RULE_RESP" | jq -r '.data.id')
if [ -z "$RULE_ID" ] || [ "$RULE_ID" = "null" ]; then
    echo "  ❌ 创建规则失败"
    exit 1
fi
echo "  ✅ 创建成功，规则ID: $RULE_ID"
echo ""

# ============================================================================
# 步骤 8: 触发 ITSM 同步
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 8/9: 触发 ITSM 同步                                               ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  请求: POST $API_BASE/api/v1/plugins/$ITSM_PLUGIN_ID/sync"
echo ""

SYNC_RESP=$(curl -s -X POST "$API_BASE/api/v1/plugins/$ITSM_PLUGIN_ID/sync" \
  -H "Authorization: Bearer $TOKEN")

echo "  同步响应:"
echo "$SYNC_RESP" | jq .
echo ""

SYNC_LOG_ID=$(echo "$SYNC_RESP" | jq -r '.data.id')
echo "  同步日志ID: $SYNC_LOG_ID"
echo ""

# 等待同步完成
echo "  等待同步完成..."
for i in {1..10}; do
    sleep 2
    SYNC_STATUS=$(curl -s -X GET "$API_BASE/api/v1/plugins/$ITSM_PLUGIN_ID/logs?page=1&page_size=1" \
      -H "Authorization: Bearer $TOKEN")
    
    STATUS=$(echo "$SYNC_STATUS" | jq -r '.data[0].status')
    FETCHED=$(echo "$SYNC_STATUS" | jq -r '.data[0].records_fetched')
    PROCESSED=$(echo "$SYNC_STATUS" | jq -r '.data[0].records_processed')
    ERROR=$(echo "$SYNC_STATUS" | jq -r '.data[0].error_message // empty')
    
    echo "    [$i/10] 状态=$STATUS 获取=$FETCHED 处理=$PROCESSED"
    
    if [ "$STATUS" = "success" ]; then
        echo ""
        echo "  ✅ 同步成功! 获取 $FETCHED 条, 处理 $PROCESSED 条"
        break
    elif [ "$STATUS" = "failed" ]; then
        echo ""
        echo "  ❌ 同步失败: $ERROR"
        echo "  完整日志:"
        echo "$SYNC_STATUS" | jq '.data[0]'
        exit 1
    fi
done
echo ""

# 查询同步后的工单
echo "  查询工单列表..."
INCIDENTS=$(curl -s -X GET "$API_BASE/api/v1/incidents?page=1&page_size=5" \
  -H "Authorization: Bearer $TOKEN")

echo "  工单响应:"
echo "$INCIDENTS" | jq '{total: .total, page: .page, items: [.data[:3][] | {external_id, title, scanned, matched_rule_id}]}'
echo ""

TOTAL_INCIDENTS=$(echo "$INCIDENTS" | jq -r '.total')
echo "  工单总数: $TOTAL_INCIDENTS"
echo ""

# ============================================================================
# 步骤 9: 等待规则引擎并处理审批
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 9/10: 等待规则引擎触发 -> waiting_approval                        ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  规则引擎调度器每 10 秒扫描一次未处理的工单"
echo "  流程将在审批节点暂停，等待人工批准"
echo ""

INSTANCE_ID=""
STATUS=""
APPROVAL_TASK_ID=""

# 等待流程进入 waiting_approval 状态
for i in {1..12}; do
    echo "  [$i/12] 检查流程实例..."
    sleep 3
    
    INSTANCES=$(curl -s -X GET "$API_BASE/api/v1/healing/instances?flow_id=$FLOW_ID&page=1&page_size=1" \
      -H "Authorization: Bearer $TOKEN")
    
    INSTANCE_ID=$(echo "$INSTANCES" | jq -r '.data[0].id // empty')
    STATUS=$(echo "$INSTANCES" | jq -r '.data[0].status // empty')
    CURRENT_NODE=$(echo "$INSTANCES" | jq -r '.data[0].current_node_id // empty')
    
    if [ -n "$INSTANCE_ID" ] && [ "$INSTANCE_ID" != "null" ]; then
        echo "        实例ID=$INSTANCE_ID 状态=$STATUS 当前节点=$CURRENT_NODE"
        
        if [ "$STATUS" = "waiting_approval" ]; then
            echo ""
            echo "  ✅ 流程已进入审批等待状态!"
            break
        elif [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
            echo ""
            echo "  ❌ 流程在审批前异常结束: $STATUS"
            exit 1
        fi
    else
        echo "        ⏳ 等待实例创建..."
    fi
done
echo ""

# ============================================================================
# 步骤 10: 审批流程
# ============================================================================
if [ "$STATUS" = "waiting_approval" ]; then
    echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
    echo "┃ 步骤 10/10: 审批流程                                                   ┃"
    echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
    echo ""
    
    # 10.1 显示流程实例详情
    echo "  [10.1] 流程实例详情"
    echo "  请求: GET $API_BASE/api/v1/healing/instances/$INSTANCE_ID"
    echo ""
    
    INSTANCE_DETAIL=$(curl -s -X GET "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
      -H "Authorization: Bearer $TOKEN")
    
    echo "  响应:"
    echo "$INSTANCE_DETAIL" | jq '.data | {id, status, current_node_id, flow_id, rule_id, incident_id, created_at}'
    echo ""
    
    # 显示审批节点状态
    echo "  审批节点状态:"
    echo "$INSTANCE_DETAIL" | jq '.data.node_states.approval_1 // "等待审批"'
    echo ""
    
    # 10.2 查询待审批任务
    echo "  [10.2] 查询待审批任务"
    echo "  请求: GET $API_BASE/api/v1/healing/approvals/pending"
    echo ""
    
    PENDING_APPROVALS=$(curl -s -X GET "$API_BASE/api/v1/healing/approvals/pending?page=1&page_size=10" \
      -H "Authorization: Bearer $TOKEN")
    
    echo "  响应 (待审批任务列表):"
    echo "$PENDING_APPROVALS" | jq '.'
    echo ""
    
    # 找到当前实例的审批任务
    APPROVAL_TASK_ID=$(echo "$PENDING_APPROVALS" | jq -r '.data[] | select(.flow_instance_id == "'$INSTANCE_ID'") | .id' | head -1)
    
    if [ -n "$APPROVAL_TASK_ID" ] && [ "$APPROVAL_TASK_ID" != "null" ]; then
        echo "  ✅ 找到审批任务 ID: $APPROVAL_TASK_ID"
        echo ""
        
        # 10.3 获取审批任务详情
        echo "  [10.3] 获取审批任务详情"
        echo "  请求: GET $API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID"
        echo ""
        
        APPROVAL_DETAIL=$(curl -s -X GET "$API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID" \
          -H "Authorization: Bearer $TOKEN")
        
        echo "  响应:"
        echo "$APPROVAL_DETAIL" | jq '.'
        echo ""
        
        # 提取审批任务的关键信息
        APPROVAL_STATUS=$(echo "$APPROVAL_DETAIL" | jq -r '.data.status')
        APPROVAL_NODE_ID=$(echo "$APPROVAL_DETAIL" | jq -r '.data.node_id')
        APPROVAL_TIMEOUT=$(echo "$APPROVAL_DETAIL" | jq -r '.data.timeout_at')
        APPROVAL_CREATED=$(echo "$APPROVAL_DETAIL" | jq -r '.data.created_at')
        
        echo "  审批任务摘要:"
        echo "    - 任务ID: $APPROVAL_TASK_ID"
        echo "    - 状态: $APPROVAL_STATUS"
        echo "    - 节点: $APPROVAL_NODE_ID"
        echo "    - 创建时间: $APPROVAL_CREATED"
        echo "    - 超时时间: $APPROVAL_TIMEOUT"
        echo ""
        
        # 10.4 批准任务
        echo "  [10.4] 批准审批任务"
        echo "  请求: POST $API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID/approve"
        echo "  参数: {\"comment\": \"E2E 测试自动批准\"}"
        echo ""
        
        APPROVE_RESP=$(curl -s -X POST "$API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID/approve" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer $TOKEN" \
          -d '{"comment": "E2E 测试自动批准"}')
        
        echo "  响应:"
        echo "$APPROVE_RESP" | jq '.'
        echo ""
        
        APPROVE_MSG=$(echo "$APPROVE_RESP" | jq -r '.message // .error // "未知"')
        if echo "$APPROVE_MSG" | grep -qi "成功\|success\|approved"; then
            echo "  ✅ 审批通过成功!"
        else
            echo "  ❌ 审批结果异常: $APPROVE_MSG"
            exit 1
        fi
        echo ""
        
        # 10.5 验证审批任务状态已更新
        echo "  [10.5] 验证审批任务状态"
        echo "  请求: GET $API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID"
        echo ""
        
        APPROVAL_UPDATED=$(curl -s -X GET "$API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID" \
          -H "Authorization: Bearer $TOKEN")
        
        echo "  响应:"
        echo "$APPROVAL_UPDATED" | jq '.data | {id, status, decided_by, decided_at, decision_comment} // .'
        echo ""
        
        # 10.6 等待流程继续执行并完成
        echo "  [10.6] 等待流程完成执行"
        echo ""
        
        for j in {1..15}; do
            sleep 2
            
            INSTANCE_STATUS=$(curl -s -X GET "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
              -H "Authorization: Bearer $TOKEN")
            
            STATUS=$(echo "$INSTANCE_STATUS" | jq -r '.data.status // empty')
            CURRENT_NODE=$(echo "$INSTANCE_STATUS" | jq -r '.data.current_node_id // empty')
            
            echo "    [$j/15] 状态=$STATUS 当前节点=$CURRENT_NODE"
            
            if [ "$STATUS" = "completed" ]; then
                echo ""
                echo "  ✅ 流程执行完成!"
                break
            elif [ "$STATUS" = "failed" ]; then
                echo ""
                echo "  ❌ 流程执行失败"
                break
            fi
        done
    else
        echo "  ❌ 未找到对应的审批任务"
        APPROVAL_TASK_ID=""
        exit 1
    fi
fi
echo ""

# ============================================================================
# 结果总结
# ============================================================================
echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║                         测试结果                                       ║"
echo "╚════════════════════════════════════════════════════════════════════════╝"
echo ""

if [ "$STATUS" != "completed" ]; then
    echo "  ❌ 最终流程状态异常: ${STATUS:-未知}"
    exit 1
fi

if [ -n "$INSTANCE_ID" ] && [ "$INSTANCE_ID" != "null" ]; then
    echo "  流程实例详情:"
    INSTANCE_DATA=$(curl -s -X GET "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
      -H "Authorization: Bearer $TOKEN")
    echo "$INSTANCE_DATA" | jq '.data | {id, status, flow_id, rule_id, incident_id, node_states: (.node_states | keys), created_at}'
    echo ""
    
    # 格式化显示 Ansible 输出
    echo "  📋 Ansible 执行输出:"
    echo "  ─────────────────────────────────────────────────────────────────"
    echo "$INSTANCE_DATA" | jq -r '.data.node_states.execution_1.stdout // "无输出"'
    echo "  ─────────────────────────────────────────────────────────────────"
    echo ""
    echo "  ✅✅✅ 端到端测试成功! ✅✅✅"
else
    echo "  ❌ 未找到流程实例"
    echo ""
    echo "  调试信息:"
    echo "    - 查看服务器日志: tail -f ${REPO_ROOT}/server.log | grep -E 'Healing|调度'"
    echo "    - 检查工单是否已被扫描处理"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  测试ID:     $TEST_ID"
echo "  ITSM插件:   $ITSM_PLUGIN_ID"
echo "  CMDB插件:   $CMDB_PLUGIN_ID"
echo "  密钥来源:   $SECRETS_SOURCE_ID"
echo "  Git仓库:    $GIT_REPO_ID"
echo "  流程ID:     $FLOW_ID"
echo "  规则ID:     $RULE_ID"
echo "  实例ID:     ${INSTANCE_ID:-未创建}"
echo "  审批任务ID: ${APPROVAL_TASK_ID:-N/A}"
echo "  最终状态:   ${STATUS:-未知}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 显示 Flow 详情
echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📊 自愈流程详情 (Flow ID: $FLOW_ID)                                  │"
echo "└──────────────────────────────────────────────────────────────────────┘"
curl -s -X GET "$API_BASE/api/v1/healing/flows/$FLOW_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, is_active, nodes, edges}'
echo ""

# 显示 Rule 详情
echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📋 匹配规则详情 (Rule ID: $RULE_ID)                                  │"
echo "└──────────────────────────────────────────────────────────────────────┘"
curl -s -X GET "$API_BASE/api/v1/healing/rules/$RULE_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.data | {id, name, priority, trigger_mode, match_mode, is_active, conditions, flow_id}'
echo ""

# ============================================================================
# 验证通知模块
# ============================================================================
echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📬 通知验证 (Mock 服务: localhost:9999)                              │"
echo "└──────────────────────────────────────────────────────────────────────┘"

NOTIFICATION_RESP=$(curl -s http://localhost:9999/notifications 2>/dev/null)
NOTIFICATION_COUNT=$(echo "$NOTIFICATION_RESP" | jq '.total // 0')

if [ "$NOTIFICATION_COUNT" -gt 0 ]; then
    echo "  ✅ Mock 服务收到 $NOTIFICATION_COUNT 条通知"
    echo ""
    echo "  最新通知内容:"
    echo "  ─────────────────────────────────────────────────────────────────"
    echo "$NOTIFICATION_RESP" | jq -r '.notifications[-1].body | to_entries[] | "  \(.key): \(.value | tostring | .[0:100])"' 2>/dev/null || \
    echo "$NOTIFICATION_RESP" | jq '.notifications[-1]'
    echo "  ─────────────────────────────────────────────────────────────────"
    echo ""
    echo "  📊 通知变量数量: $(echo "$NOTIFICATION_RESP" | jq '.notifications[-1].body.variables | length // 0')"
else
    echo "  ❌ Mock 服务未收到通知"
    exit 1
fi
echo ""
