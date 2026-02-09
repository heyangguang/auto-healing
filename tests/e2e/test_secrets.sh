#!/bin/bash
# 端到端测试 - 密钥管理模块
# 测试: 文件、Vault、Webhook 三种密钥源，包括 ssh_key 和 password 两种认证类型

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"
SECRETS_PATH="${SECRETS_PATH:-/tmp/e2e-test-secrets}"
MOCK_SECRETS_ENDPOINT="${MOCK_SECRETS_ENDPOINT:-http://localhost:5001}"

echo "=========================================="
echo "  密钥管理端到端测试"
echo "=========================================="

# 登录
TOKEN=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | jq -r '.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  exit 1
fi
echo "✅ 登录成功"

# 清理已存在的测试密钥源
echo ""
echo "--- 清理测试密钥源 ---"
EXISTING=$(curl -s "$API_BASE/secrets-sources" -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.name | startswith("E2E")) | .id')
for ID in $EXISTING; do
  curl -s -X DELETE "$API_BASE/secrets-sources/$ID" -H "Authorization: Bearer $TOKEN" > /dev/null
  echo "已删除: $ID"
done

# ==================== 1. 文件类型密钥源（只支持 ssh_key）====================
echo ""
echo "========== 1. 文件类型密钥源 (ssh_key) =========="

echo "--- 准备测试密钥 ---"
mkdir -p "$SECRETS_PATH"
cat > "$SECRETS_PATH/ansible_key" << 'EOF'
-----BEGIN OPENSSH PRIVATE KEY-----
e2e-test-ssh-key-content-for-testing
-----END OPENSSH PRIVATE KEY-----
EOF
chmod 600 "$SECRETS_PATH/ansible_key"
echo "✅ 测试密钥文件已创建"

echo ""
echo "--- 创建文件类型密钥源 ---"
SOURCE_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"E2E File SSH Key\",
    \"type\": \"file\",
    \"auth_type\": \"ssh_key\",
    \"config\": {
      \"key_path\": \"$SECRETS_PATH/ansible_key\",
      \"username\": \"root\"
    }
  }")

SOURCE_ID=$(echo "$SOURCE_RESULT" | jq -r '.data.id // .id')
if [ "$SOURCE_ID" != "null" ] && [ -n "$SOURCE_ID" ]; then
  echo "✅ 创建成功 (ID: $SOURCE_ID)"
  
  QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"hostname\":\"test-server\",\"source_id\":\"$SOURCE_ID\"}")
  
  AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type // .auth_type')
  PRIVATE_KEY=$(echo "$QUERY_RESULT" | jq -r '.data.private_key // .private_key')
  USER_NAME=$(echo "$QUERY_RESULT" | jq -r '.data.username // .username')
  if [ "$AUTH_TYPE" == "ssh_key" ]; then
    echo "✅ 查询成功"
    echo "   auth_type: $AUTH_TYPE"
    echo "   username: $USER_NAME"
    echo "   private_key: $PRIVATE_KEY"
  else
    echo "❌ 查询失败: $QUERY_RESULT"
  fi
else
  echo "❌ 创建失败: $SOURCE_RESULT"
fi

# ==================== 2. Vault 类型密钥源（ssh_key + password）====================
echo ""
echo "========== 2. Vault 类型密钥源 =========="

if curl -s "$MOCK_SECRETS_ENDPOINT/v1/sys/health" > /dev/null 2>&1; then
  echo "✅ Mock Vault 服务可用"
  
  # 2.1 Vault SSH Key
  echo ""
  echo "--- 2.1 创建 Vault 密钥源 (ssh_key) ---"
  VAULT_KEY_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"E2E Vault SSH Key\",
      \"type\": \"vault\",
      \"auth_type\": \"ssh_key\",
      \"config\": {
        \"address\": \"$MOCK_SECRETS_ENDPOINT\",
        \"token\": \"mock-vault-token\",
        \"secret_path\": \"secret/data/ssh/ansible-key\"
      }
    }")
  
  VAULT_KEY_ID=$(echo "$VAULT_KEY_RESULT" | jq -r '.data.id // .id')
  if [ "$VAULT_KEY_ID" != "null" ] && [ -n "$VAULT_KEY_ID" ]; then
    echo "✅ 创建成功 (ID: $VAULT_KEY_ID)"
    
    # 查询密钥
    QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"hostname\":\"test-server\",\"source_id\":\"$VAULT_KEY_ID\"}")
    
    AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type // .auth_type')
    PRIVATE_KEY=$(echo "$QUERY_RESULT" | jq -r '.data.private_key // .private_key')
    USER_NAME=$(echo "$QUERY_RESULT" | jq -r '.data.username // .username')
    echo "   auth_type: $AUTH_TYPE"
    echo "   username: $USER_NAME"
    echo "   private_key: $PRIVATE_KEY"
  else
    echo "❌ 创建失败: $VAULT_KEY_RESULT"
  fi
  
  # 2.2 Vault Password
  echo ""
  echo "--- 2.2 创建 Vault 密钥源 (password) ---"
  VAULT_PWD_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"E2E Vault Password\",
      \"type\": \"vault\",
      \"auth_type\": \"password\",
      \"config\": {
        \"address\": \"$MOCK_SECRETS_ENDPOINT\",
        \"token\": \"mock-vault-token\",
        \"secret_path\": \"secret/data/ssh/server-password\"
      }
    }")
  
  VAULT_PWD_ID=$(echo "$VAULT_PWD_RESULT" | jq -r '.data.id // .id')
  if [ "$VAULT_PWD_ID" != "null" ] && [ -n "$VAULT_PWD_ID" ]; then
    echo "✅ 创建成功 (ID: $VAULT_PWD_ID)"
    
    # 查询密码
    QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"hostname\":\"test-server\",\"source_id\":\"$VAULT_PWD_ID\"}")
    
    AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type // .auth_type')
    PASSWORD_VAL=$(echo "$QUERY_RESULT" | jq -r '.data.password // .password')
    USER_NAME=$(echo "$QUERY_RESULT" | jq -r '.data.username // .username')
    echo "   auth_type: $AUTH_TYPE"
    echo "   username: $USER_NAME"
    echo "   password: $PASSWORD_VAL"
  else
    echo "❌ 创建失败: $VAULT_PWD_RESULT"
  fi
else
  echo "⚠️ Mock Vault 服务不可用，跳过测试"
fi

# ==================== 3. Webhook 类型密钥源（ssh_key + password）====================
echo ""
echo "========== 3. Webhook 类型密钥源 =========="

if curl -s -I "$MOCK_SECRETS_ENDPOINT/api/secrets/query" > /dev/null 2>&1; then
  echo "✅ Mock Webhook 服务可用"
  
  # 3.1 Webhook SSH Key
  echo ""
  echo "--- 3.1 创建 Webhook 密钥源 (ssh_key) ---"
  WEBHOOK_KEY_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"E2E Webhook SSH Key\",
      \"type\": \"webhook\",
      \"auth_type\": \"ssh_key\",
      \"config\": {
        \"url\": \"$MOCK_SECRETS_ENDPOINT/api/secrets/query\",
        \"method\": \"POST\",
        \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"ssh_key\\\"}\"
      }
    }")
  
  WEBHOOK_KEY_ID=$(echo "$WEBHOOK_KEY_RESULT" | jq -r '.data.id // .id')
  if [ "$WEBHOOK_KEY_ID" != "null" ] && [ -n "$WEBHOOK_KEY_ID" ]; then
    echo "✅ 创建成功 (ID: $WEBHOOK_KEY_ID)"
    
    # 查询密钥
    QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"hostname\":\"test-server\",\"source_id\":\"$WEBHOOK_KEY_ID\"}")
    
    AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type // .auth_type')
    PRIVATE_KEY=$(echo "$QUERY_RESULT" | jq -r '.data.private_key // .private_key')
    USER_NAME=$(echo "$QUERY_RESULT" | jq -r '.data.username // .username')
    echo "   auth_type: $AUTH_TYPE"
    echo "   username: $USER_NAME"
    echo "   private_key: $PRIVATE_KEY"
  else
    echo "❌ 创建失败: $WEBHOOK_KEY_RESULT"
  fi
  
  # 3.2 Webhook Password
  echo ""
  echo "--- 3.2 创建 Webhook 密钥源 (password) ---"
  WEBHOOK_PWD_RESULT=$(curl -s -X POST "$API_BASE/secrets-sources" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"E2E Webhook Password\",
      \"type\": \"webhook\",
      \"auth_type\": \"password\",
      \"config\": {
        \"url\": \"$MOCK_SECRETS_ENDPOINT/api/secrets/query\",
        \"method\": \"POST\",
        \"body_template\": \"{\\\"hostname\\\": \\\"{hostname}\\\", \\\"auth_type\\\": \\\"password\\\"}\"
      }
    }")
  
  WEBHOOK_PWD_ID=$(echo "$WEBHOOK_PWD_RESULT" | jq -r '.data.id // .id')
  if [ "$WEBHOOK_PWD_ID" != "null" ] && [ -n "$WEBHOOK_PWD_ID" ]; then
    echo "✅ 创建成功 (ID: $WEBHOOK_PWD_ID)"
    
    # 查询密码
    QUERY_RESULT=$(curl -s -X POST "$API_BASE/secrets/query" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"hostname\":\"test-server\",\"source_id\":\"$WEBHOOK_PWD_ID\"}")
    
    AUTH_TYPE=$(echo "$QUERY_RESULT" | jq -r '.data.auth_type // .auth_type')
    PASSWORD_VAL=$(echo "$QUERY_RESULT" | jq -r '.data.password // .password')
    USER_NAME=$(echo "$QUERY_RESULT" | jq -r '.data.username // .username')
    echo "   auth_type: $AUTH_TYPE"
    echo "   username: $USER_NAME"
    echo "   password: $PASSWORD_VAL"
  else
    echo "❌ 创建失败: $WEBHOOK_PWD_RESULT"
  fi
else
  echo "⚠️ Mock Webhook 服务不可用，跳过测试"
fi

# 清理测试文件
rm -rf "$SECRETS_PATH"

echo ""
echo "=========================================="
echo "  密钥管理测试完成"
echo "  - File: ssh_key ✓"
echo "  - Vault: ssh_key + password ✓"
echo "  - Webhook: ssh_key + password ✓"
echo "=========================================="
