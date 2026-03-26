#!/bin/bash
# 端到端测试 - Git 仓库管理模块
# 测试: 所有认证方式（none, token, password, ssh_key）

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
GITEA_URL="${GITEA_URL:-http://localhost:3000}"
GITEA_SSH_PORT="${GITEA_SSH_PORT:-2222}"
GITEA_USER="${GITEA_USER:-testadmin}"
GITEA_PASS="${GITEA_PASS:-testadmin123}"
GITEA_TOKEN="${GITEA_TOKEN:-}"
SSH_KEY_PATH="${SSH_KEY_PATH:-/tmp/e2e-ssh/id_rsa}"

echo "=========================================="
echo "  Git 仓库管理端到端测试"
echo "=========================================="

# 登录
TOKEN=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  exit 1
fi

if [ -z "$GITEA_TOKEN" ]; then
  echo "❌ 缺少 GITEA_TOKEN，无法覆盖 Token 认证场景"
  exit 1
fi

# 清理已存在的测试仓库
echo ""
echo "--- 清理测试仓库 ---"
EXISTING=$(curl -s "$API_BASE/git-repos" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E")) | .id' 2>/dev/null || echo "")
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/git-repos/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除: $ID"
done

# ==================== 1. 公开仓库（无认证） ====================
echo ""
echo "========== 1. 公开仓库（无认证） =========="
REPO1_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Public Repo\",
    \"url\": \"$GITEA_URL/$GITEA_USER/e2e-test-repo.git\",
    \"default_branch\": \"main\",
    \"auth_type\": \"none\"
  }")

REPO1_ID=$(echo "$REPO1_RESULT" | jq -r '.data.id')
if [ "$REPO1_ID" != "null" ] && [ -n "$REPO1_ID" ]; then
  echo "✅ 创建成功 (ID: $REPO1_ID)"
  
  # 同步
  SYNC_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$REPO1_ID/sync" -H "Authorization: Bearer $TOKEN")
  sleep 3
  STATUS=$(curl -s "$API_BASE/git-repos/$REPO1_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
  if [ "$STATUS" == "ready" ]; then
    echo "✅ 同步成功 (状态: $STATUS)"
  else
    echo "❌ 公开仓库同步失败: $STATUS"
    exit 1
  fi
else
  echo "❌ 创建失败: $REPO1_RESULT"
  exit 1
fi

# ==================== 2. HTTPS + Token ====================
echo ""
echo "========== 2. HTTPS + Token =========="
REPO2_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Token Auth Repo\",
    \"url\": \"$GITEA_URL/$GITEA_USER/e2e-test-repo.git\",
    \"default_branch\": \"main\",
    \"auth_type\": \"token\",
    \"auth_config\": {
      \"token\": \"$GITEA_TOKEN\"
    }
  }")

REPO2_ID=$(echo "$REPO2_RESULT" | jq -r '.data.id')
if [ "$REPO2_ID" != "null" ] && [ -n "$REPO2_ID" ]; then
  echo "✅ 创建成功 (ID: $REPO2_ID)"
  
  SYNC_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$REPO2_ID/sync" -H "Authorization: Bearer $TOKEN")
  sleep 3
  STATUS=$(curl -s "$API_BASE/git-repos/$REPO2_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
  if [ "$STATUS" == "ready" ]; then
    echo "✅ Token 认证同步成功 (状态: $STATUS)"
  else
    echo "❌ Token 认证同步失败: $STATUS"
    exit 1
  fi
else
  echo "❌ 创建失败: $REPO2_RESULT"
  exit 1
fi

# ==================== 3. HTTPS + 用户名密码 ====================
echo ""
echo "========== 3. HTTPS + 用户名密码 =========="
REPO3_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E Password Auth Repo\",
    \"url\": \"$GITEA_URL/$GITEA_USER/e2e-test-repo.git\",
    \"default_branch\": \"main\",
    \"auth_type\": \"password\",
    \"auth_config\": {
      \"username\": \"$GITEA_USER\",
      \"password\": \"$GITEA_PASS\"
    }
  }")

REPO3_ID=$(echo "$REPO3_RESULT" | jq -r '.data.id')
if [ "$REPO3_ID" != "null" ] && [ -n "$REPO3_ID" ]; then
  echo "✅ 创建成功 (ID: $REPO3_ID)"
  
  SYNC_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$REPO3_ID/sync" -H "Authorization: Bearer $TOKEN")
  sleep 3
  STATUS=$(curl -s "$API_BASE/git-repos/$REPO3_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
  if [ "$STATUS" == "ready" ]; then
    echo "✅ 密码认证同步成功 (状态: $STATUS)"
  else
    echo "❌ 密码认证同步失败: $STATUS"
    exit 1
  fi
else
  echo "❌ 创建失败: $REPO3_RESULT"
  exit 1
fi

# ==================== 4. SSH 密钥 ====================
echo ""
echo "========== 4. SSH 密钥认证 =========="

# 读取私钥内容
if [ -f "$SSH_KEY_PATH" ]; then
  PRIVATE_KEY=$(cat "$SSH_KEY_PATH" | sed 's/$/\\n/' | tr -d '\n')
  
  REPO4_RESULT=$(curl -s -X POST "$API_BASE/git-repos" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"E2E SSH Key Repo\",
      \"url\": \"ssh://git@localhost:$GITEA_SSH_PORT/$GITEA_USER/e2e-test-repo.git\",
      \"default_branch\": \"main\",
      \"auth_type\": \"ssh_key\",
      \"auth_config\": {
        \"private_key\": \"$PRIVATE_KEY\"
      }
    }")
  
  REPO4_ID=$(echo "$REPO4_RESULT" | jq -r '.data.id')
  if [ "$REPO4_ID" != "null" ] && [ -n "$REPO4_ID" ]; then
    echo "✅ 创建成功 (ID: $REPO4_ID)"
    
    SYNC_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$REPO4_ID/sync" -H "Authorization: Bearer $TOKEN")
    sleep 5
    STATUS=$(curl -s "$API_BASE/git-repos/$REPO4_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')
    if [ "$STATUS" == "ready" ]; then
      echo "✅ SSH 密钥认证同步成功 (状态: $STATUS)"
    else
      echo "❌ SSH 密钥认证同步失败: $STATUS"
      curl -s "$API_BASE/git-repos/$REPO4_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.last_error // "无"'
      exit 1
    fi
  else
    echo "❌ 创建失败: $REPO4_RESULT"
    exit 1
  fi
else
  echo "❌ SSH 密钥文件不存在: $SSH_KEY_PATH"
  echo "   请先运行: ssh-keygen -t rsa -b 3072 -f $SSH_KEY_PATH -N \"\""
  exit 1
fi

# ==================== 5. 文件浏览 API ====================
echo ""
echo "========== 5. 文件浏览 API =========="

# 使用第一个成功同步的仓库测试
TEST_REPO_ID=""
if [ "$REPO1_ID" != "null" ] && [ -n "$REPO1_ID" ]; then
  TEST_REPO_ID="$REPO1_ID"
elif [ "$REPO2_ID" != "null" ] && [ -n "$REPO2_ID" ]; then
  TEST_REPO_ID="$REPO2_ID"
fi

if [ -n "$TEST_REPO_ID" ]; then
  # 获取文件树
  FILES_RESULT=$(curl -s "$API_BASE/git-repos/$TEST_REPO_ID/files" \
    -H "Authorization: Bearer $TOKEN")
  
  FILE_COUNT=$(echo "$FILES_RESULT" | jq '(.data.files) | length')
  if [ "$FILE_COUNT" != "null" ] && [ "$FILE_COUNT" -gt 0 ]; then
    echo "✅ 文件浏览成功 (文件数: $FILE_COUNT)"
    echo "   文件列表: $(echo "$FILES_RESULT" | jq -r '(.data.files)[].name' | tr '\n' ', ')"
    
    # 获取第一个文件内容
    FIRST_FILE=$(echo "$FILES_RESULT" | jq -r '(.data.files)[] | select(.type=="file") | .path' | head -1)
    if [ -n "$FIRST_FILE" ]; then
      CONTENT_RESULT=$(curl -s "$API_BASE/git-repos/$TEST_REPO_ID/files?path=$FIRST_FILE" \
        -H "Authorization: Bearer $TOKEN")
      CONTENT_LEN=$(echo "$CONTENT_RESULT" | jq -r '(.data.content // "") | length')
      if [ "$CONTENT_LEN" != "null" ] && [ "$CONTENT_LEN" -gt 0 ]; then
        echo "✅ 获取文件内容成功 ($FIRST_FILE, ${CONTENT_LEN} 字符)"
      else
        echo "❌ 获取文件内容失败: $CONTENT_RESULT"
        exit 1
      fi
    fi
  else
    echo "❌ 文件浏览失败: $FILES_RESULT"
    exit 1
  fi
else
  echo "❌ 没有可用的测试仓库"
  exit 1
fi

# ==================== 6. 变量扫描 API ====================
echo ""
echo "========== 6. 变量扫描 API =========="

if [ -n "$TEST_REPO_ID" ]; then
  # 优先选择 site.yml，排除 .auto-healing.yml
  MAIN_PLAYBOOK=$(curl -s "$API_BASE/git-repos/$TEST_REPO_ID/files" \
-H "Authorization: Bearer $TOKEN" | jq -r '(.data.files)[] | select(.name == "site.yml") | .path' | head -1)
  
  if [ -z "$MAIN_PLAYBOOK" ] || [ "$MAIN_PLAYBOOK" == "null" ]; then
    MAIN_PLAYBOOK=$(curl -s "$API_BASE/git-repos/$TEST_REPO_ID/files" \
-H "Authorization: Bearer $TOKEN" | jq -r '(.data.files)[] | select(.name | endswith(".yml")) | select(.name | startswith(".") | not) | .path' | head -1)
  fi
  
  if [ -n "$MAIN_PLAYBOOK" ] && [ "$MAIN_PLAYBOOK" != "null" ]; then
    SCAN_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$TEST_REPO_ID/scan-variables" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"main_playbook\": \"$MAIN_PLAYBOOK\"}")
    
    VAR_COUNT=$(echo "$SCAN_RESULT" | jq '.variables | length')
    if [ "$VAR_COUNT" != "null" ] && [ "$VAR_COUNT" -gt 0 ]; then
      echo "✅ 变量扫描成功 (入口: $MAIN_PLAYBOOK, 变量数: $VAR_COUNT)"
echo "   变量列表: $(echo "$SCAN_RESULT" | jq -r '(.data.variables)[].name' | tr '\n' ', ')"
    else
      echo "❌ 变量扫描失败: $VAR_COUNT 个变量"
      exit 1
    fi
  else
    echo "❌ 没有找到 playbook 文件进行扫描"
    exit 1
  fi
else
  echo "❌ 没有可用的测试仓库"
  exit 1
fi

# ==================== 7. 激活测试 (auto 模式) ====================
echo ""
echo "========== 7. 激活测试 (auto 模式) =========="

if [ -n "$TEST_REPO_ID" ] && [ -n "$MAIN_PLAYBOOK" ]; then
  # 激活仓库 (auto 模式)
  ACTIVATE_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$TEST_REPO_ID/activate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"main_playbook\": \"$MAIN_PLAYBOOK\", \"config_mode\": \"auto\"}")
  
  if echo "$ACTIVATE_RESULT" | jq -e '.code == 0 or ((.message // "") | test("成功|success|activated"; "i"))' > /dev/null 2>&1; then
    echo "✅ auto 模式激活成功"
    
    REPO_STATUS=$(curl -s "$API_BASE/git-repos/$TEST_REPO_ID" -H "Authorization: Bearer $TOKEN")
echo "   is_active: $(echo "$REPO_STATUS" | jq -r '.data.is_active')"
echo "   config_mode: $(echo "$REPO_STATUS" | jq -r '.data.config_mode')"
echo "   variables: $(echo "$REPO_STATUS" | jq -r '(.data.variables) | length') 个"
echo "   变量列表: $(echo "$REPO_STATUS" | jq -r '(.data.variables)[].name' | tr '\n' ', ')"
    
    # 停用
    curl -s -X POST "$API_BASE/git-repos/$TEST_REPO_ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null
    echo "   ✅ 已停用"
  else
    echo "❌ auto 模式激活失败: $ACTIVATE_RESULT"
    exit 1
  fi
else
  echo "❌ 没有可用的测试仓库或入口文件"
  exit 1
fi

# ==================== 8. 激活测试 (enhanced 模式) ====================
echo ""
echo "========== 8. 激活测试 (enhanced 模式) =========="

if [ -n "$TEST_REPO_ID" ] && [ -n "$MAIN_PLAYBOOK" ]; then
  # 激活仓库 (enhanced 模式，手动定义变量)
  ACTIVATE_RESULT=$(curl -s -X POST "$API_BASE/git-repos/$TEST_REPO_ID/activate" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"main_playbook\": \"$MAIN_PLAYBOOK\",
      \"config_mode\": \"enhanced\",
      \"variables\": [
        {\"name\": \"target_host\", \"type\": \"string\", \"required\": true, \"description\": \"目标主机\"},
        {\"name\": \"service_name\", \"type\": \"select\", \"enum\": [\"nginx\", \"mysql\", \"redis\"], \"default\": \"nginx\"},
        {\"name\": \"action_type\", \"type\": \"select\", \"enum\": [\"start\", \"stop\", \"restart\"], \"default\": \"restart\"},
        {\"name\": \"timeout\", \"type\": \"int\", \"min\": 30, \"max\": 3600, \"default\": 300}
      ]
    }")
  
  if echo "$ACTIVATE_RESULT" | jq -e '.code == 0 or ((.message // "") | test("成功|success|activated"; "i"))' > /dev/null 2>&1; then
    echo "✅ enhanced 模式激活成功"
    
    REPO_STATUS=$(curl -s "$API_BASE/git-repos/$TEST_REPO_ID" -H "Authorization: Bearer $TOKEN")
echo "   is_active: $(echo "$REPO_STATUS" | jq -r '.data.is_active')"
echo "   config_mode: $(echo "$REPO_STATUS" | jq -r '.data.config_mode')"
echo "   variables: $(echo "$REPO_STATUS" | jq -r '(.data.variables) | length') 个"
    echo "   变量详情:"
echo "$REPO_STATUS" | jq -r '(.data.variables)[] | "     - \(.name) (\(.type))"'
    
    # 停用
    curl -s -X POST "$API_BASE/git-repos/$TEST_REPO_ID/deactivate" -H "Authorization: Bearer $TOKEN" > /dev/null
    echo "   ✅ 已停用"
  else
    echo "❌ enhanced 模式激活失败: $ACTIVATE_RESULT"
    exit 1
  fi
else
  echo "❌ 没有可用的测试仓库或入口文件"
  exit 1
fi

echo ""
echo "=========================================="
echo "  Git 仓库管理测试完成"
echo "=========================================="
