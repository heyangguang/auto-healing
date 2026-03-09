# API Testing Guide

> Quick reference for testing Auto-Healing Platform APIs with cURL.

## Authentication

```bash
# Login and get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

echo $TOKEN

# Verify authentication
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/auth/me | jq
```

## Health Check

```bash
curl -s http://localhost:8080/health | jq
# Expected: {"status":"ok"}
```

---

## Plugin Management

```bash
# List plugins
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/plugins" | jq

# Get plugin stats
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/plugins/stats" | jq

# Create ITSM plugin
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/plugins" \
  -d '{
    "name": "My ITSM Plugin",
    "type": "itsm",
    "config": {
      "url": "http://itsm-server:5000",
      "auth_type": "basic",
      "username": "api_user",
      "password": "api_pass"
    }
  }' | jq

# Get plugin by ID
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/plugins/{PLUGIN_ID}" | jq

# Trigger manual sync
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/plugins/{PLUGIN_ID}/sync" | jq

# View sync logs
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/plugins/{PLUGIN_ID}/logs" | jq
```

---

## Incidents (Tickets)

```bash
# List incidents
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/incidents" | jq

# Get incident stats
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/incidents/stats" | jq

# Get incident details
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/incidents/{INCIDENT_ID}" | jq

# Manually trigger healing for an incident
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/incidents/{INCIDENT_ID}/trigger" | jq
```

---

## CMDB Asset Management

```bash
# List CMDB items
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/cmdb" | jq

# Filter by status
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/cmdb?status=active" | jq

# Get CMDB stats
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/cmdb/stats" | jq

# Enter maintenance mode
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/cmdb/{ITEM_ID}/maintenance" \
  -d '{"reason": "Scheduled maintenance", "estimated_end": "2026-03-10T18:00:00Z"}' | jq

# Exit maintenance mode
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/cmdb/{ITEM_ID}/resume" | jq

# Batch enter maintenance
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/cmdb/batch/maintenance" \
  -d '{"ids": ["id1", "id2"], "reason": "Batch maintenance"}' | jq
```

---

## Healing Rules

```bash
# List healing rules
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/rules" | jq

# Create healing rule
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/healing/rules" \
  -d '{
    "name": "Disk Space Alert",
    "description": "Auto-clean when disk usage > 85%",
    "priority": 10,
    "trigger_mode": "auto",
    "flow_id": "{FLOW_ID}",
    "conditions": {
      "logic": "AND",
      "rules": [
        {"field": "title", "operator": "contains", "value": "disk"}
      ]
    }
  }' | jq

# Activate rule
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/rules/{RULE_ID}/activate" | jq

# Deactivate rule
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/rules/{RULE_ID}/deactivate" | jq
```

---

## Healing Flows

```bash
# List flows
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/flows" | jq

# Get flow details
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/flows/{FLOW_ID}" | jq

# Dry-run a flow
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/healing/flows/{FLOW_ID}/dry-run" \
  -d '{"incident_id": "{INCIDENT_ID}"}' | jq
```

---

## Flow Instances

```bash
# List flow instances
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/instances" | jq

# Filter by status
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/instances?status=running" | jq

# Get instance details
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/instances/{INSTANCE_ID}" | jq

# Cancel a running instance
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/instances/{INSTANCE_ID}/cancel" | jq

# SSE event stream (real-time)
curl -N -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/instances/{INSTANCE_ID}/events"
```

---

## Execution Tasks & Runs

```bash
# List execution task templates
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-tasks" | jq

# Execute a task
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/execution-tasks/{TASK_ID}/execute" \
  -d '{"target_hosts": ["192.168.1.10"], "variables": {"cleanup_days": "7"}}' | jq

# List execution runs
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-runs" | jq

# Get run details
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-runs/{RUN_ID}" | jq

# Get run logs
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-runs/{RUN_ID}/logs" | jq

# SSE log stream (real-time Ansible output)
curl -N -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-runs/{RUN_ID}/stream"

# Cancel a run
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-runs/{RUN_ID}/cancel" | jq
```

---

## Scheduled Tasks

```bash
# List schedules
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-schedules" | jq

# Create schedule
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/execution-schedules" \
  -d '{
    "task_id": "{TASK_ID}",
    "cron_expression": "0 2 * * 5",
    "description": "Weekly cleanup on Friday 2am"
  }' | jq

# Enable/disable schedule
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-schedules/{SCHEDULE_ID}/enable" | jq

curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/execution-schedules/{SCHEDULE_ID}/disable" | jq
```

---

## Notifications

```bash
# List channels
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/channels" | jq

# List templates
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/templates" | jq

# Get available template variables
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/template-variables" | jq

# List notification logs
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/notifications" | jq

# Get notification stats
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/notifications/stats" | jq
```

---

## Git Repositories & Playbooks

```bash
# List Git repos
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/git-repos" | jq

# Sync a repo
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/git-repos/{REPO_ID}/sync" | jq

# List playbooks
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/playbooks" | jq

# Scan playbook variables
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/playbooks/{PLAYBOOK_ID}/scan" | jq
```

---

## Secrets Management

```bash
# List secret sources
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/secrets-sources" | jq

# Query a secret
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/secrets/query" \
  -d '{"source_id": "{SOURCE_ID}", "key": "ssh_private_key"}' | jq
```

---

## Approvals (Pending Center)

```bash
# List pending approvals
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/healing/approvals/pending" | jq

# Approve a task
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/healing/approvals/{APPROVAL_ID}/approve" \
  -d '{"comment": "Approved - looks good"}' | jq

# Reject a task
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/tenant/healing/approvals/{APPROVAL_ID}/reject" \
  -d '{"comment": "Rejected - needs review"}' | jq
```

---

## Audit Logs

```bash
# List audit logs
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/audit-logs" | jq

# Get audit stats
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/audit-logs/stats" | jq

# Export audit logs
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/audit-logs/export" -o audit_logs.csv
```

---

## Dashboard

```bash
# Get dashboard overview
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/tenant/dashboard/overview" | jq
```

---

## Platform Administration

```bash
# List users (platform admin)
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/users" | jq

# List roles (platform admin)
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/roles" | jq

# List tenants
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/tenants" | jq

# Get permission tree
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/permissions/tree" | jq
```

---

## Tips

1. **Replace `{PLUGIN_ID}`, `{FLOW_ID}`, etc.** with actual UUID values from list responses
2. **Use `jq` filters** for large responses: `| jq '.data[0]'` to see first item
3. **SSE streams** require `-N` flag (no buffering): `curl -N -H "Authorization: Bearer $TOKEN" <url>`
4. **Token expiration**: Default 60 minutes. Use `/auth/refresh` to renew
5. **Pagination**: Add `?page=1&page_size=10` to list endpoints
