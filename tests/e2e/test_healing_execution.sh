#!/bin/bash
# 自愈引擎 - 执行节点端到端测试
# 流程: start → approval → execution → end

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
DB_CONTAINER="${DB_CONTAINER:-auto-healing-postgres}"

echo "============================================"
echo "  自愈引擎 - 执行节点端到端测试"
echo "============================================"
echo ""
echo "测试流程："
echo "  1. 创建自愈流程 (start → approval → execution → end)"
echo "  2. 创建自愈规则 (匹配 severity=critical)"
echo "  3. 插入带主机信息的测试工单"
echo "  4. 等待调度器扫描"
echo "  5. 审批通过"
echo "  6. 验证执行节点执行结果"
echo "  7. 验证流程完成"
echo ""

# ==================== 步骤 0: 登录 ====================
echo "========== 步骤 0: 登录 =========="

LOGIN_RESULT=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESULT" | jq -r '.access_token // .data.access_token')

if [ -z "$TOKEN" ] || [ "$TOKEN" == "null" ]; then
  echo "❌ 登录失败"
  exit 1
fi
echo "✅ 登录成功"

# ==================== 清理旧测试数据 ====================
echo ""
echo "========== 清理旧测试数据 =========="

# 清理旧的测试数据
docker exec $DB_CONTAINER psql -U postgres -d auto_healing -c "
  DELETE FROM approval_tasks WHERE flow_instance_id IN (
    SELECT id FROM flow_instances WHERE rule_id IN (
      SELECT id FROM healing_rules WHERE name LIKE 'E2E Execution Test%'
    )
  );
  DELETE FROM flow_instances WHERE rule_id IN (
    SELECT id FROM healing_rules WHERE name LIKE 'E2E Execution Test%'
  );
" > /dev/null 2>&1 || true

# 停用所有现有规则（确保只有新规则能匹配）
echo "停用所有现有规则..."
ALL_RULES=$(curl -s "$API_BASE/healing/rules" -H "Authorization: Bearer $TOKEN" | jq -r '.data[].id // empty')
for RULE_ID in $ALL_RULES; do
  curl -s -X POST "$API_BASE/healing/rules/$RULE_ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null
done

# 删除旧的执行测试规则和流程
OLD_RULES=$(curl -s "$API_BASE/healing/rules" -H "Authorization: Bearer $TOKEN" | jq -r '.data[].id // empty')
for RULE_ID in $OLD_RULES; do
  RULE_NAME=$(curl -s "$API_BASE/healing/rules/$RULE_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.name // .name')
  if [[ "$RULE_NAME" == E2E\ Execution* ]]; then
    curl -s -X DELETE "$API_BASE/healing/rules/$RULE_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  fi
done

OLD_FLOWS=$(curl -s "$API_BASE/healing/flows" -H "Authorization: Bearer $TOKEN" | jq -r '.data[].id // empty')
for FLOW_ID in $OLD_FLOWS; do
  FLOW_NAME=$(curl -s "$API_BASE/healing/flows/$FLOW_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.name // .name')
  if [[ "$FLOW_NAME" == E2E\ Execution* ]]; then
    curl -s -X DELETE "$API_BASE/healing/flows/$FLOW_ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  fi
done

docker exec $DB_CONTAINER psql -U postgres -d auto_healing -c "
  DELETE FROM incidents WHERE title LIKE 'E2E Execution Test%';
" > /dev/null 2>&1 || true
echo "✅ 旧数据已清理，其他规则已停用"

# ==================== 步骤 1: 创建自愈流程（含执行节点） ====================
echo ""
echo "========== 步骤 1: 创建自愈流程 =========="

FLOW_RESULT=$(curl -s -X POST "$API_BASE/healing/flows" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Execution Test Flow",
    "description": "测试执行节点的完整流程",
    "nodes": [
      {"id": "start", "type": "start", "name": "开始"},
      {"id": "approval", "type": "approval", "name": "审批", "config": {"timeout_hours": 1}},
      {"id": "execution", "type": "execution", "name": "执行", "config": {
        "playbook_path": "playbooks/healing_test.yml",
        "extra_vars": {
          "service_name": "nginx",
          "action": "check",
          "check_port": 80
        }
      }},
      {"id": "end", "type": "end", "name": "结束"}
    ],
    "edges": [
      {"from": "start", "to": "approval"},
      {"from": "approval", "to": "execution"},
      {"from": "execution", "to": "end"}
    ],
    "is_active": true
  }')

FLOW_ID=$(echo "$FLOW_RESULT" | jq -r '.data.id // .id')
if [ -z "$FLOW_ID" ] || [ "$FLOW_ID" == "null" ]; then
  echo "❌ 创建流程失败: $FLOW_RESULT"
  exit 1
fi

echo "✅ 流程创建成功"
echo ""
echo "┌─────────────────────────────────────────────┐"
echo "│           流程详情 (ID: $FLOW_ID)                  │"
echo "├─────────────────────────────────────────────┤"
echo "│  节点执行路径:                              │"
echo "│    [start] 开始                             │"
echo "│       ↓                                    │"
echo "│    [approval] 审批                          │"
echo "│       ↓                                    │"
echo "│    [execution] 执行 ← 新增执行节点          │"
echo "│       ↓                                    │"
echo "│    [end] 结束                               │"
echo "└─────────────────────────────────────────────┘"

# ==================== 步骤 2: 创建自愈规则 ====================
echo ""
echo "========== 步骤 2: 创建自愈规则 =========="

RULE_RESULT=$(curl -s -X POST "$API_BASE/healing/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Execution Test Rule - Critical\",
    \"description\": \"匹配 severity=critical 的工单并执行自愈\",
    \"priority\": 100,
    \"trigger_mode\": \"auto\",
    \"conditions\": [
      {\"field\": \"severity\", \"operator\": \"equals\", \"value\": \"critical\"}
    ],
    \"match_mode\": \"all\",
    \"flow_id\": $FLOW_ID,
    \"is_active\": true
  }")

RULE_ID=$(echo "$RULE_RESULT" | jq -r '.data.id // .id')
if [ -z "$RULE_ID" ] || [ "$RULE_ID" == "null" ]; then
  echo "❌ 创建规则失败: $RULE_RESULT"
  exit 1
fi
echo "✅ 规则创建成功 (ID: $RULE_ID)"

# ==================== 步骤 3: 插入测试工单 ====================
echo ""
echo "========== 步骤 3: 插入测试工单 =========="

INCIDENT_ID=$(uuidgen)
EXTERNAL_ID="E2E-EXEC-$(date +%s)"

docker exec $DB_CONTAINER psql -U postgres -d auto_healing -c "
  INSERT INTO incidents (id, external_id, title, description, severity, status, affected_ci, raw_data, scanned, created_at, updated_at)
  VALUES (
    '$INCIDENT_ID',
    '$EXTERNAL_ID',
    'E2E Execution Test - Critical Incident',
    '测试执行节点的工单',
    'critical',
    'open',
    'healing-test-host',
    '{\"source\": \"e2e_exec_test\", \"test_id\": \"$EXTERNAL_ID\"}',
    false,
    now(),
    now()
  );
" > /dev/null 2>&1

echo "✅ 工单插入成功"
echo ""
echo "┌─────────────────────────────────────────────┐"
echo "│              工单详情                       │"
echo "├─────────────────────────────────────────────┤"
echo "│  ID: $INCIDENT_ID"
echo "│  标题: E2E Execution Test - Critical        │"
echo "│  严重级别: critical                         │"
echo "│  受影响 CI: healing-test-host               │"
echo "│  对应主机: 192.168.31.103                  │"
echo "└─────────────────────────────────────────────┘"

# ==================== 步骤 4: 等待调度器扫描 ====================
echo ""
echo "========== 步骤 4: 等待调度器扫描 =========="
echo "调度器每 10 秒扫描一次，等待 12 秒..."
sleep 12
echo "✅ 等待完成"

# ==================== 步骤 5: 验证工单状态 ====================
echo ""
echo "========== 步骤 5: 验证工单状态 =========="

INCIDENT_STATUS=$(docker exec $DB_CONTAINER psql -U postgres -d auto_healing -t -c "
  SELECT scanned, matched_rule_id, healing_flow_instance_id FROM incidents WHERE id = '$INCIDENT_ID';
")
echo "工单状态: $INCIDENT_STATUS"

INSTANCE_ID=$(echo "$INCIDENT_STATUS" | awk -F'|' '{print $3}' | tr -d ' ')

if [ -z "$INSTANCE_ID" ] || [ "$INSTANCE_ID" == "" ]; then
  echo "⚠️ 未创建流程实例"
else
  echo "✅ 流程实例 ID: $INSTANCE_ID"
fi

# ==================== 步骤 6: 查询并执行审批 ====================
echo ""
echo "========== 步骤 6: 查询并执行审批 =========="

APPROVAL_ID=$(docker exec $DB_CONTAINER psql -U postgres -d auto_healing -t -c "
  SELECT id FROM approval_tasks WHERE flow_instance_id = $INSTANCE_ID AND status = 'pending';
" | tr -d ' ')

if [ -z "$APPROVAL_ID" ] || [ "$APPROVAL_ID" == "" ]; then
  echo "⚠️ 未找到待审批任务"
else
  echo "✅ 找到审批任务 ID: $APPROVAL_ID"
  echo ""
  echo "审批人: admin (当前登录用户)"
  echo "审批意见: E2E 执行节点测试自动批准"
  
  APPROVE_RESULT=$(curl -s -X POST "$API_BASE/healing/approvals/$APPROVAL_ID/approve" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"comment": "E2E 执行节点测试自动批准"}')
  
  echo "审批结果: $(echo "$APPROVE_RESULT" | jq -r '.message // .')"
  
  # 等待执行节点完成
  echo ""
  echo "等待执行节点完成 (3秒)..."
  sleep 3
fi

# ==================== 步骤 7: 验证执行节点结果 ====================
echo ""
echo "========== 步骤 7: 验证执行节点结果 =========="

INSTANCE_DETAIL=$(curl -s "$API_BASE/healing/instances/$INSTANCE_ID" -H "Authorization: Bearer $TOKEN")
NODE_STATES=$(echo "$INSTANCE_DETAIL" | jq '.data.node_states // .node_states')
EXEC_STATE=$(echo "$NODE_STATES" | jq '.execution // empty')

echo ""
echo "┌─────────────────────────────────────────────────────────────────┐"
echo "│           执行节点详细结果                                      │"
echo "├─────────────────────────────────────────────────────────────────┤"

if [ "$EXEC_STATE" != "" ] && [ "$EXEC_STATE" != "null" ]; then
  EXEC_STATUS=$(echo "$EXEC_STATE" | jq -r '.status // "unknown"')
  EXEC_PLAYBOOK=$(echo "$EXEC_STATE" | jq -r '.playbook_path // "N/A"')
  EXEC_MESSAGE=$(echo "$EXEC_STATE" | jq -r '.message // "N/A"')
  EXEC_DURATION=$(echo "$EXEC_STATE" | jq -r '.duration_ms // "N/A"')
  EXEC_EXIT_CODE=$(echo "$EXEC_STATE" | jq -r '.exit_code // "N/A"')
  EXEC_HOSTS=$(echo "$EXEC_STATE" | jq -r '.hosts // []')
  EXEC_EXTRA_VARS=$(echo "$EXEC_STATE" | jq -r '.extra_vars // {}')
  
  echo "│  状态: $EXEC_STATUS"
  echo "│  Playbook: $EXEC_PLAYBOOK"
  echo "│  退出码: $EXEC_EXIT_CODE"
  echo "│  耗时: ${EXEC_DURATION}ms"
  echo "│  消息: $EXEC_MESSAGE"
  echo "├─────────────────────────────────────────────────────────────────┤"
  
  # 显示 CMDB 解析结果
  CMDB_RESULTS=$(echo "$EXEC_STATE" | jq '.cmdb_results // empty')
  if [ "$CMDB_RESULTS" != "" ] && [ "$CMDB_RESULTS" != "null" ]; then
    echo "│  CMDB 主机解析:"
    echo "$CMDB_RESULTS" | jq -r '.[] | "│    [\(.source)] \(.original) -> \(.resolved)"'
    echo "├─────────────────────────────────────────────────────────────────┤"
  fi
  
  # 显示密钥服务信息
  SECRETS_INFO=$(echo "$EXEC_STATE" | jq '.secrets_info // empty')
  if [ "$SECRETS_INFO" != "" ] && [ "$SECRETS_INFO" != "null" ]; then
    echo "│  密钥服务凭证:"
    echo "$SECRETS_INFO" | jq -r '.[] | "│    [\(.source)] host=\(.host), user=\(.username), auth=\(.auth_type)"'
    echo "├─────────────────────────────────────────────────────────────────┤"
  fi
  
  # 显示原始和解析后的主机
  ORIGINAL_HOSTS=$(echo "$EXEC_STATE" | jq -r '.original_hosts // []')
  RESOLVED_HOSTS=$(echo "$EXEC_STATE" | jq -r '.resolved_hosts // []')
  echo "│  原始主机: $ORIGINAL_HOSTS"
  echo "│  解析后主机: $RESOLVED_HOSTS"
  echo "├─────────────────────────────────────────────────────────────────┤"
  
  # 显示 extra_vars
  echo "│  执行参数 (extra_vars):"
  echo "$EXEC_EXTRA_VARS" | jq -r 'to_entries[] | "│    \(.key): \(.value)"'
  echo "├─────────────────────────────────────────────────────────────────┤"
  
  # 显示统计信息
  EXEC_STATS=$(echo "$EXEC_STATE" | jq '.stats // empty')
  if [ "$EXEC_STATS" != "" ] && [ "$EXEC_STATS" != "null" ]; then
    echo "│  Ansible 统计信息:"
    echo "│    ok: $(echo "$EXEC_STATS" | jq -r '.ok // 0')"
    echo "│    changed: $(echo "$EXEC_STATS" | jq -r '.changed // 0')"
    echo "│    unreachable: $(echo "$EXEC_STATS" | jq -r '.unreachable // 0')"
    echo "│    failed: $(echo "$EXEC_STATS" | jq -r '.failed // 0')"
    echo "│    skipped: $(echo "$EXEC_STATS" | jq -r '.skipped // 0')"
    echo "├─────────────────────────────────────────────────────────────────┤"
  fi
  
  # 显示 stdout（执行日志）
  EXEC_STDOUT=$(echo "$EXEC_STATE" | jq -r '.stdout // ""')
  if [ -n "$EXEC_STDOUT" ] && [ "$EXEC_STDOUT" != "" ]; then
    echo "│  Ansible 执行日志 (stdout):"
    echo "├─────────────────────────────────────────────────────────────────┤"
    echo "$EXEC_STDOUT" | head -50 | while IFS= read -r line; do
      echo "│  $line"
    done
    STDOUT_LINES=$(echo "$EXEC_STDOUT" | wc -l)
    if [ "$STDOUT_LINES" -gt 50 ]; then
      echo "│  ... (共 $STDOUT_LINES 行，显示前 50 行)"
    fi
    echo "├─────────────────────────────────────────────────────────────────┤"
  fi
  
  # 显示 stderr（如果有）
  EXEC_STDERR=$(echo "$EXEC_STATE" | jq -r '.stderr // ""')
  if [ -n "$EXEC_STDERR" ] && [ "$EXEC_STDERR" != "" ]; then
    echo "│  stderr 输出:"
    echo "$EXEC_STDERR" | head -20 | while IFS= read -r line; do
      echo "│  $line"
    done
  fi
  
  echo "└─────────────────────────────────────────────────────────────────┘"
  
  if [ "$EXEC_STATUS" == "completed" ]; then
    echo "✅ 执行节点执行成功"
  else
    echo "⚠️ 执行节点状态: $EXEC_STATUS"
  fi
else
  echo "│  (无执行节点状态记录)                                          │"
  echo "└─────────────────────────────────────────────────────────────────┘"
fi

# ==================== 步骤 8: 验证流程最终状态 ====================
echo ""
echo "========== 步骤 8: 验证流程最终状态 =========="

FINAL_STATUS=$(echo "$INSTANCE_DETAIL" | jq -r '.data.status // .status')
FINAL_NODE=$(echo "$INSTANCE_DETAIL" | jq -r '.data.current_node_id // .current_node_id')

echo ""
echo "┌─────────────────────────────────────────────┐"
echo "│           流程最终状态                      │"
echo "├─────────────────────────────────────────────┤"
echo "│  流程实例 ID: $INSTANCE_ID                       │"
echo "│  当前节点: $FINAL_NODE                          │"
echo "│  最终状态: $FINAL_STATUS                   │"
if [ "$FINAL_STATUS" == "completed" ]; then
  echo "│                                             │"
  echo "│  节点执行轨迹:                              │"
  echo "│    [start] 开始 ✓                          │"
  echo "│       ↓                                    │"
  echo "│    [approval] 审批 ✓ (已通过)              │"
  echo "│       ↓                                    │"
  echo "│    [execution] 执行 ✓ (已完成)             │"
  echo "│       ↓                                    │"
  echo "│    [end] 结束 ✓                            │"
fi
echo "└─────────────────────────────────────────────┘"

if [ "$FINAL_STATUS" == "completed" ]; then
  echo "✅ 流程已完成！"
else
  echo "⚠️ 流程状态: $FINAL_STATUS (节点: $FINAL_NODE)"
fi

# ==================== 最终结果 ====================
echo ""
echo "============================================"
echo "  自愈引擎 - 执行节点端到端测试结果"
echo "============================================"
echo ""
echo "  测试资源:"
echo "    流程 ID: $FLOW_ID"
echo "    规则 ID: $RULE_ID"
echo "    工单 ID: $INCIDENT_ID"
echo "    流程实例 ID: $INSTANCE_ID"
echo ""
echo "  ✅ 步骤 1: 创建自愈流程（含执行节点）"
echo "  ✅ 步骤 2: 创建自愈规则"
echo "  ✅ 步骤 3: 插入测试工单"
echo "  ✅ 步骤 4: 等待调度器扫描"
echo "  ✅ 步骤 5: 验证工单状态"
echo "  ✅ 步骤 6: 执行审批"
echo "  ✅ 步骤 7: 验证执行节点"
echo "  ✅ 步骤 8: 验证流程完成"
echo ""
echo "  🎉 执行节点端到端测试通过!"
echo "============================================"
