#!/bin/bash

# 登录获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

echo "=== 创建通知渠道 (15个) ==="

# Webhook 渠道 (5个)
for i in {1..5}; do
  curl -s -X POST http://localhost:8080/api/v1/channels \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"Webhook-运维告警-${i}\",
      \"type\": \"webhook\",
      \"description\": \"运维告警 Webhook 渠道 ${i}\",
      \"config\": {
        \"webhook_url\": \"http://localhost:5000/webhook/alert-${i}\",
        \"method\": \"POST\"
      },
      \"default_recipients\": [],
      \"is_default\": $([ $i -eq 1 ] && echo 'true' || echo 'false')
    }" | jq -r '.data.id // .message'
done

# 邮件渠道 (5个)
for i in {1..5}; do
  curl -s -X POST http://localhost:8080/api/v1/channels \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"邮件-团队${i}\",
      \"type\": \"email\",
      \"description\": \"团队${i}邮件通知渠道\",
      \"config\": {
        \"smtp_host\": \"smtp.example.com\",
        \"smtp_port\": 587,
        \"username\": \"noreply@company.com\",
        \"password\": \"password\",
        \"from_address\": \"noreply@company.com\",
        \"use_tls\": true
      },
      \"default_recipients\": [\"team${i}@company.com\"],
      \"is_default\": false
    }" | jq -r '.data.id // .message'
done

# 钉钉渠道 (5个)
for i in {1..5}; do
  curl -s -X POST http://localhost:8080/api/v1/channels \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"钉钉-群组${i}\",
      \"type\": \"dingtalk\",
      \"description\": \"钉钉机器人群组${i}\",
      \"config\": {
        \"webhook_url\": \"https://oapi.dingtalk.com/robot/send?access_token=token${i}\",
        \"secret\": \"SEC_secret_${i}\"
      },
      \"default_recipients\": [],
      \"is_default\": false
    }" | jq -r '.data.id // .message'
done

echo ""
echo "=== 创建通知模板 (15个) ==="

# 执行结果模板 - webhook (3个)
for i in {1..3}; do
  curl -s -X POST http://localhost:8080/api/v1/templates \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"Webhook-执行结果通知-${i}\",
      \"description\": \"任务执行完成后的 Webhook 通知模板\",
      \"event_type\": \"execution_result\",
      \"supported_channels\": [\"webhook\"],
      \"subject_template\": \"【{{execution_status}}】任务执行通知\",
      \"body_template\": \"# {{execution_status_emoji}} 任务执行{{execution_status}}\n\n时间: {{timestamp}}\n耗时: {{execution_duration_ms}} ms\",
      \"format\": \"markdown\"
    }" | jq -r '.data.id // .message'
done

# 执行结果模板 - email (3个)
for i in {1..3}; do
  curl -s -X POST http://localhost:8080/api/v1/templates \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"邮件-执行结果通知-${i}\",
      \"description\": \"任务执行完成后的邮件通知模板\",
      \"event_type\": \"execution_result\",
      \"supported_channels\": [\"email\"],
      \"subject_template\": \"【{{execution_status}}】任务执行通知\",
      \"body_template\": \"<h1>任务执行{{execution_status}}</h1><p>时间: {{timestamp}}</p><p>耗时: {{execution_duration_ms}} ms</p>\",
      \"format\": \"html\"
    }" | jq -r '.data.id // .message'
done

# 执行结果模板 - webhook+email (3个)
for i in {1..3}; do
  curl -s -X POST http://localhost:8080/api/v1/templates \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"多渠道-执行结果通知-${i}\",
      \"description\": \"支持多渠道的任务执行通知模板\",
      \"event_type\": \"execution_result\",
      \"supported_channels\": [\"webhook\", \"email\"],
      \"subject_template\": \"任务执行报告-${i}\",
      \"body_template\": \"执行状态: {{execution_status}}\n主机: {{target_hosts}}\n时间: {{timestamp}}\",
      \"format\": \"text\"
    }" | jq -r '.data.id // .message'
done

# 钉钉模板 (3个)
for i in {1..3}; do
  curl -s -X POST http://localhost:8080/api/v1/templates \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"钉钉-告警通知-${i}\",
      \"description\": \"钉钉机器人告警通知模板\",
      \"event_type\": \"custom\",
      \"supported_channels\": [\"dingtalk\"],
      \"subject_template\": \"告警通知\",
      \"body_template\": \"### 告警通知\n- 状态: {{execution_status}}\n- 时间: {{timestamp}}\",
      \"format\": \"markdown\"
    }" | jq -r '.data.id // .message'
done

# 全渠道模板 (3个)
for i in {1..3}; do
  curl -s -X POST http://localhost:8080/api/v1/templates \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"全渠道-通用通知-${i}\",
      \"description\": \"支持所有渠道类型的通用通知模板\",
      \"event_type\": \"custom\",
      \"supported_channels\": [\"webhook\", \"email\", \"dingtalk\"],
      \"subject_template\": \"通用通知-${i}\",
      \"body_template\": \"{{execution_status}} | {{timestamp}} | {{target_hosts}}\",
      \"format\": \"text\"
    }" | jq -r '.data.id // .message'
done

echo ""
echo "=== 完成！==="
echo "渠道: 15个 (5 webhook + 5 email + 5 dingtalk)"
echo "模板: 15个 (各种 supported_channels 组合)"
