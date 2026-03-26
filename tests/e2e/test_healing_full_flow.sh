#!/bin/bash
# 完整端到端测试 - 自愈引擎全流程
# 1. 创建流程和规则
# 2. 插入匹配的工单
# 3. 等待调度器扫描匹配
# 4. 验证流程实例和审批任务
# 5. 执行审批操作
# 6. 验证最终状态

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
DB_CONTAINER="${DB_CONTAINER:-auto-healing-postgres}"

echo "============================================"
echo "  自愈引擎 - 完整端到端测试"
echo "============================================"
echo ""
echo "测试流程："
echo "  1. 创建自愈流程 (start → approval → end)"
echo "  2. 创建自愈规则 (匹配 severity=critical)"
echo "  3. 插入测试工单到数据库"
echo "  4. 等待调度器扫描 (10-15秒)"
echo "  5. 验证流程实例和审批任务"
echo "  6. 执行审批操作"
echo "  7. 验证最终状态"
echo ""

# ==================== 登录 ====================
echo "========== 步骤 0: 登录 =========="
TOKEN=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  exit 1
fi
echo "✅ 登录成功"

# ==================== 清理旧数据 ====================
echo ""
echo "========== 清理旧测试数据 =========="

# 清理旧的审批任务和流程实例（通过数据库）
docker exec $DB_CONTAINER psql -U postgres -d auto_healing -c "
  DELETE FROM approval_tasks WHERE flow_instance_id IN (
    SELECT id FROM flow_instances WHERE rule_id IN (
      SELECT id FROM healing_rules WHERE name LIKE 'E2E Full Test%'
    )
  );
  DELETE FROM flow_instances WHERE rule_id IN (
    SELECT id FROM healing_rules WHERE name LIKE 'E2E Full Test%'
  );
" > /dev/null 2>&1 || true

# 清理测试规则
EXISTING=$(curl -s "$API_BASE/healing/rules" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Full Test")) | .id' 2>/dev/null || echo "")
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/healing/rules/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

# 清理测试流程
EXISTING=$(curl -s "$API_BASE/healing/flows" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E Full Test")) | .id' 2>/dev/null || echo "")
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/healing/flows/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
done

# 清理测试工单
docker exec $DB_CONTAINER psql -U postgres -d auto_healing -c "
  DELETE FROM incidents WHERE title LIKE 'E2E Full Test%';
" > /dev/null 2>&1 || true

echo "✅ 旧数据已清理"

# ==================== 步骤 1: 创建自愈流程 ====================
echo ""
echo "========== 步骤 1: 创建自愈流程 =========="
FLOW_RESULT=$(curl -s -X POST "$API_BASE/healing/flows" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "E2E Full Test Flow",
    "description": "完整端到端测试流程",
    "nodes": [
      {"id": "start", "type": "start", "name": "开始"},
      {"id": "approval", "type": "approval", "name": "审批", "config": {"timeout_hours": 1}},
      {"id": "end", "type": "end", "name": "结束"}
    ],
    "edges": [
      {"from": "start", "to": "approval"},
      {"from": "approval", "to": "end"}
    ],
    "is_active": true
  }')

FLOW_ID=$(echo "$FLOW_RESULT" | jq -r '.data.id')
if [ "$FLOW_ID" == "null" ] || [ -z "$FLOW_ID" ]; then
  echo "❌ 创建流程失败"
  echo "$FLOW_RESULT" | jq
  exit 1
fi
echo "✅ 流程创建成功"
echo "   流程 ID: $FLOW_ID"
echo "   节点: start → approval → end"

# ==================== 步骤 2: 创建自愈规则 ====================
echo ""
echo "========== 步骤 2: 创建自愈规则 =========="
RULE_RESULT=$(curl -s -X POST "$API_BASE/healing/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Full Test Rule - Critical\",
    \"description\": \"匹配 severity=critical 的工单\",
    \"priority\": 100,
    \"trigger_mode\": \"auto\",
    \"conditions\": [
      {\"field\": \"severity\", \"operator\": \"equals\", \"value\": \"critical\"}
    ],
    \"match_mode\": \"all\",
    \"flow_id\": $FLOW_ID,
    \"is_active\": true
  }")

RULE_ID=$(echo "$RULE_RESULT" | jq -r '.data.id')
if [ "$RULE_ID" == "null" ] || [ -z "$RULE_ID" ]; then
  echo "❌ 创建规则失败"
  echo "$RULE_RESULT" | jq
  exit 1
fi
echo "✅ 规则创建成功"
echo ""
echo "┌─────────────────────────────────────────────┐"
echo "│           规则详情 (ID: $RULE_ID)           │"
echo "├─────────────────────────────────────────────┤"
echo "│  名称: E2E Full Test Rule - Critical       │"
echo "│  触发模式: auto (自动触发)                 │"
echo "│  匹配模式: all (所有条件满足)              │"
echo "│  优先级: 100                               │"
echo "│  关联流程: $FLOW_ID                        │"
echo "├─────────────────────────────────────────────┤"
echo "│  匹配条件:                                 │"
echo "│    字段: severity                          │"
echo "│    操作符: equals (等于)                   │"
echo "│    值: critical                            │"
echo "└─────────────────────────────────────────────┘"

# ==================== 步骤 3: 插入测试工单 ====================
echo ""
echo "========== 步骤 3: 插入测试工单 =========="

# 生成 UUID
INCIDENT_ID=$(uuidgen)
EXTERNAL_ID="E2E-$(date +%s)"
TIMESTAMP=$(date -u +"%Y-%m-%d %H:%M:%S")

# 直接插入数据库
docker exec $DB_CONTAINER psql -U postgres -d auto_healing -c "
  INSERT INTO incidents (id, external_id, title, description, severity, status, raw_data, scanned, created_at, updated_at)
  VALUES (
    '$INCIDENT_ID',
    '$EXTERNAL_ID',
    'E2E Full Test - Critical Incident',
    '这是一个自动生成的测试工单，用于测试自愈引擎的完整流程',
    'critical',
    'open',
    '{\"source\": \"e2e_test\", \"test_id\": \"$EXTERNAL_ID\"}',
    false,
    '$TIMESTAMP',
    '$TIMESTAMP'
  );
" > /dev/null

echo "✅ 工单插入成功"
echo ""
echo "┌─────────────────────────────────────────────┐"
echo "│              工单详情                      │"
echo "├─────────────────────────────────────────────┤"
echo "│  ID: $INCIDENT_ID"
echo "│  标题: E2E Full Test - Critical Incident   │"
echo "│  严重级别: critical  ← 匹配规则条件        │"
echo "│  状态: open                               │"
echo "│  scanned: false  ← 等待调度器扫描       │"
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

# 查询工单状态
INCIDENT_STATUS=$(docker exec $DB_CONTAINER psql -U postgres -d auto_healing -t -c "
  SELECT scanned, matched_rule_id, healing_flow_instance_id
  FROM incidents WHERE id = '$INCIDENT_ID';
" | tr -d ' ')

SCANNED=$(echo "$INCIDENT_STATUS" | cut -d'|' -f1)
MATCHED_RULE=$(echo "$INCIDENT_STATUS" | cut -d'|' -f2)
INSTANCE_ID=$(echo "$INCIDENT_STATUS" | cut -d'|' -f3)

echo "工单状态:"
echo "  scanned: $SCANNED"
echo "  matched_rule_id: $MATCHED_RULE"
echo "  healing_flow_instance_id: $INSTANCE_ID"

if [ "$SCANNED" != "t" ]; then
  echo ""
  echo "⚠️ 工单尚未被扫描，可能调度器还未处理"
  echo "   请检查服务器日志确认调度器正在运行"
  echo ""
  echo "手动重新等待 10 秒..."
  sleep 10
  
  INCIDENT_STATUS=$(docker exec $DB_CONTAINER psql -U postgres -d auto_healing -t -c "
    SELECT scanned, matched_rule_id, healing_flow_instance_id
    FROM incidents WHERE id = '$INCIDENT_ID';
  " | tr -d ' ')
  SCANNED=$(echo "$INCIDENT_STATUS" | cut -d'|' -f1)
  MATCHED_RULE=$(echo "$INCIDENT_STATUS" | cut -d'|' -f2)
  INSTANCE_ID=$(echo "$INCIDENT_STATUS" | cut -d'|' -f3)
  
  echo "重新查询:"
  echo "  scanned: $SCANNED"
  echo "  matched_rule_id: $MATCHED_RULE"
  echo "  healing_flow_instance_id: $INSTANCE_ID"
fi

if [ "$SCANNED" == "t" ]; then
  echo "✅ 工单已被扫描"
else
  echo "❌ 工单未被扫描，测试失败"
  exit 1
fi

if [ -n "$MATCHED_RULE" ] && [ "$MATCHED_RULE" != "" ]; then
  echo ""
  echo "┌─────────────────────────────────────────────┐"
  echo "│           规则匹配过程                      │"
  echo "├─────────────────────────────────────────────┤"
  echo "│  1. 调度器扫描工单 (scanned=false)         │"
  echo "│  2. 检测工单字段: severity=critical       │"
  echo "│  3. 匹配规则 ID: $MATCHED_RULE                       │"
  echo "│  4. 规则条件: severity equals critical    │"
  echo "│  5. 匹配模式: all (所有条件满足)          │"
  echo "│  6. ✅ 匹配成功! 触发流程实例               │"
  echo "└─────────────────────────────────────────────┘"
else
  echo "⚠️ 未匹配到规则"
fi

# ==================== 步骤 6: 验证流程实例 ====================
echo ""
echo "========== 步骤 6: 验证流程实例 =========="

if [ -z "$INSTANCE_ID" ] || [ "$INSTANCE_ID" == "" ]; then
  echo "❌ 流程实例未创建，规则匹配或实例生成失败"
  exit 1
fi

# 查询流程实例
INSTANCE_DETAIL=$(curl -s "$API_BASE/healing/instances/$INSTANCE_ID" -H "Authorization: Bearer $TOKEN")
INSTANCE_STATUS=$(echo "$INSTANCE_DETAIL" | jq -r '.data.status')
CURRENT_NODE=$(echo "$INSTANCE_DETAIL" | jq -r '.data.current_node_id')

echo ""
echo "┌─────────────────────────────────────────────┐"
echo "│           流程执行过程                      │"
echo "├─────────────────────────────────────────────┤"
echo "│  流程实例 ID: $INSTANCE_ID                       │"
echo "│                                             │"
echo "│  节点执行轨迹:                              │"
echo "│    [start] 开始 ✓                          │"
if [ "$INSTANCE_STATUS" == "waiting_approval" ]; then
  echo "│       ↓                                    │"
  echo "│    [approval] 审批 ← 当前节点 (等待中)     │"
  echo "│       ↓                                    │"
  echo "│    [end] 结束 (待执行)                    │"
else
  echo "│       ↓                                    │"
  echo "│    [approval] 审批                         │"
  echo "│       ↓                                    │"
  echo "│    [end] 结束                             │"
fi
echo "│                                             │"
echo "│  当前状态: $INSTANCE_STATUS                 │"
echo "└─────────────────────────────────────────────┘"

if [ "$INSTANCE_STATUS" == "waiting_approval" ]; then
  echo "✅ 流程实例创建成功，等待审批"
else
  echo "❌ 流程实例状态异常: $INSTANCE_STATUS"
  exit 1
fi

# ==================== 步骤 7: 查询审批任务 ====================
echo ""
echo "========== 步骤 7: 查询审批任务 =========="

# 查询该流程实例的审批任务
APPROVAL_ID=$(docker exec $DB_CONTAINER psql -U postgres -d auto_healing -t -c "
  SELECT id FROM approval_tasks WHERE flow_instance_id = $INSTANCE_ID AND status = 'pending';
" | tr -d ' ')

if [ -z "$APPROVAL_ID" ] || [ "$APPROVAL_ID" == "" ]; then
  echo "❌ 未找到待审批任务"
  exit 1
else
  echo "✅ 找到审批任务"
  echo "   审批任务 ID: $APPROVAL_ID"
  
  # ==================== 步骤 8: 执行审批 ====================
  echo ""
  echo "========== 步骤 8: 执行审批 =========="
  echo ""
  echo "审批人: admin (当前登录用户)"
  echo "审批意见: E2E 测试自动批准"
  
  APPROVE_RESULT=$(curl -s -X POST "$API_BASE/healing/approvals/$APPROVAL_ID/approve" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"comment": "E2E 测试自动批准"}')
  
  echo "审批结果: $(echo "$APPROVE_RESULT" | jq -r '.message // .')"
  
  # 等待流程继续执行
  echo ""
  echo "等待流程继续执行 (3秒)..."
  sleep 3
  
  # ==================== 步骤 9: 验证审批记录 ====================
  echo ""
  echo "========== 步骤 9: 验证审批记录 =========="
  
  # 查询审批任务详细信息
  APPROVAL_DETAIL=$(docker exec $DB_CONTAINER psql -U postgres -d auto_healing -t -c "
    SELECT status, decided_by, decision_comment, decided_at 
    FROM approval_tasks WHERE id = $APPROVAL_ID;
  " | tr -d ' ')
  
  APPROVAL_STATUS=$(echo "$APPROVAL_DETAIL" | cut -d'|' -f1)
  DECIDED_BY=$(echo "$APPROVAL_DETAIL" | cut -d'|' -f2)
  DECISION_COMMENT=$(echo "$APPROVAL_DETAIL" | cut -d'|' -f3)
  DECIDED_AT=$(echo "$APPROVAL_DETAIL" | cut -d'|' -f4)
  
  echo ""
  echo "┌─────────────────────────────────────────────┐"
  echo "│           审批记录详情                      │"
  echo "├─────────────────────────────────────────────┤"
  echo "│  审批任务 ID: $APPROVAL_ID                          │"
  echo "│  审批状态: $APPROVAL_STATUS                        │"
  echo "│  审批人 ID: $DECIDED_BY"
  echo "│  审批意见: E2E 测试自动批准                │"
  echo "│  审批时间: $DECIDED_AT"
  echo "└─────────────────────────────────────────────┘"
  if [ "$APPROVAL_STATUS" != "approved" ]; then
    echo "❌ 审批状态校验失败: $APPROVAL_STATUS"
    exit 1
  fi
  
  # ==================== 步骤 10: 验证流程完成 ====================
  echo ""
  echo "========== 步骤 10: 验证流程完成 =========="
  
  FINAL_INSTANCE=$(curl -s "$API_BASE/healing/instances/$INSTANCE_ID" -H "Authorization: Bearer $TOKEN")
  FINAL_STATUS=$(echo "$FINAL_INSTANCE" | jq -r '.data.status')
  FINAL_NODE=$(echo "$FINAL_INSTANCE" | jq -r '.data.current_node_id')
  
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
    echo "│    [end] 结束 ✓                            │"
  fi
  echo "└─────────────────────────────────────────────┘"
  
  if [ "$FINAL_STATUS" == "completed" ]; then
    echo "✅ 流程已完成！"
  else
    echo "❌ 流程状态异常: $FINAL_STATUS (节点: $FINAL_NODE)"
    exit 1
  fi
fi

# ==================== 最终结果 ====================
echo ""
echo "============================================"
echo "  自愈引擎 - 完整端到端测试结果"
echo "============================================"
echo ""
echo "  测试资源:"
echo "    流程 ID: $FLOW_ID"
echo "    规则 ID: $RULE_ID"
echo "    工单 ID: $INCIDENT_ID"
if [ -n "$INSTANCE_ID" ]; then
  echo "    流程实例 ID: $INSTANCE_ID"
fi
if [ -n "$APPROVAL_ID" ]; then
  echo "    审批任务 ID: $APPROVAL_ID"
fi
echo ""
echo "  ✅ 步骤 1: 创建自愈流程"
echo "  ✅ 步骤 2: 创建自愈规则"
echo "  ✅ 步骤 3: 插入测试工单"
echo "  ✅ 步骤 4: 等待调度器扫描"
echo "  ✅ 步骤 5: 验证工单状态"
if [ -n "$INSTANCE_ID" ] && [ "$INSTANCE_ID" != "" ]; then
  echo "  ✅ 步骤 6: 验证流程实例"
  echo "  ✅ 步骤 7: 查询审批任务"
  if [ -n "$APPROVAL_ID" ]; then
    echo "  ✅ 步骤 8: 执行审批"
    echo "  ✅ 步骤 9: 验证最终状态"
  fi
fi
echo ""
echo "  🎉 完整端到端测试通过!"
echo "============================================"
