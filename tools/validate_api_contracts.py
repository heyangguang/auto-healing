#!/usr/bin/env python3

from pathlib import Path
import re
import sys
from typing import List

ROOT = Path(__file__).resolve().parents[1]
OPENAPI_PATH = ROOT / "api/openapi.yaml"
API_README_PATH = ROOT / "docs/api/README.md"
AUTH_DOC_PATH = ROOT / "docs/api/auth.md"
INCIDENTS_DOC_PATH = ROOT / "docs/api/incidents.md"
PLUGINS_DOC_PATH = ROOT / "docs/api/plugins.md"
HEALING_DOC_PATH = ROOT / "docs/api/healing.md"
EXECUTION_DOC_PATH = ROOT / "docs/api/execution.md"
NOTIFICATIONS_DOC_PATH = ROOT / "docs/api/notifications.md"
GIT_REPOS_DOC_PATH = ROOT / "docs/api/git-repos.md"
PLAYBOOKS_DOC_PATH = ROOT / "docs/api/playbooks.md"
DASHBOARD_DOC_PATH = ROOT / "docs/api/dashboard.md"
SITE_MESSAGES_DOC_PATH = ROOT / "docs/api/site-messages.md"
SECRETS_DOC_PATH = ROOT / "docs/api/secrets.md"
PLATFORM_USERS_DOC_PATH = ROOT / "docs/api/platform-users.md"
PLATFORM_ROLES_DOC_PATH = ROOT / "docs/api/platform-roles.md"
AUDIT_LOGS_DOC_PATH = ROOT / "docs/api/audit-logs.md"
PLATFORM_AUDIT_DOC_PATH = ROOT / "docs/api/platform-audit-logs.md"
ROUTE_FILES = sorted(ROOT.glob("internal/modules/*/httpapi/routes*.go"))

INCIDENT_HEALING_ENUM = "pending, processing, healed, failed, skipped, dismissed"
INCIDENT_HEALING_MD = "`pending` / `processing` / `healed` / `failed` / `skipped` / `dismissed`"
FLOW_INSTANCE_ENUM = "pending, running, waiting_approval, completed, failed, cancelled"
FLOW_INSTANCE_MD = "`pending` / `running` / `waiting_approval` / `completed` / `failed` / `cancelled`"
NOTIFICATION_EVENT_TYPES = "incident_created, incident_resolved, approval_required, execution_result, custom"
NOTIFICATION_EVENT_TYPES_MD = "`incident_created` / `incident_resolved` / `approval_required` / `execution_result` / `custom`"
NOTIFICATION_LOG_STATUSES_MD = "`pending` / `sent` / `delivered` / `failed` / `bounced`"


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def require(errors: List[str], condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)


def has_regex(content: str, pattern: str) -> bool:
    return re.search(pattern, content, re.S) is not None


def parse_openapi_methods(content: str) -> dict:
    openapi_methods = {}
    current_path = None
    for line in content.splitlines():
        path_match = re.match(r"^  (/[^:]+):$", line)
        if path_match:
            current_path = path_match.group(1)
            openapi_methods[current_path] = set()
            continue
        method_match = re.match(r"^    (get|post|put|patch|delete):$", line)
        if method_match and current_path:
            openapi_methods[current_path].add(method_match.group(1).upper())
    return openapi_methods


def collect_route_methods(route_files: List[Path]) -> dict:
    route_methods = {}
    function_re = re.compile(r"func \(r Registrar\) Register(Auth|Common|Platform|Tenant)Routes\((\w+) \*gin\.RouterGroup\)")
    group_re = re.compile(r'(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\)')
    route_re = re.compile(r'(\w+)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)"')
    root_prefixes = {
        "Auth": "",
        "Common": "/common",
        "Platform": "/platform",
        "Tenant": "/tenant",
    }

    for route_file in route_files:
        prefixes = {}
        route_content = read_text(route_file)
        for line in route_content.splitlines():
            function_match = function_re.search(line)
            if function_match:
                route_kind, var_name = function_match.groups()
                prefixes[var_name] = root_prefixes[route_kind]
                continue
            group_match = group_re.search(line)
            if group_match:
                group_name, parent_name, suffix = group_match.groups()
                if parent_name in prefixes:
                    prefixes[group_name] = prefixes[parent_name] + suffix
            route_match = route_re.search(line)
            if not route_match:
                continue
            group_name, method, suffix = route_match.groups()
            if group_name not in prefixes:
                continue
            full_path = prefixes[group_name] + suffix
            if full_path.startswith("/public"):
                continue
            full_path = re.sub(r":([A-Za-z0-9_]+)", r"{\1}", full_path)
            route_methods.setdefault(full_path, set()).add(method)
    return route_methods


def validate_route_openapi_sync(content: str, errors: List[str]) -> None:
    openapi_methods = parse_openapi_methods(content)
    route_methods = collect_route_methods(ROUTE_FILES)

    missing = []
    for full_path, methods in route_methods.items():
        for method in methods:
            if full_path not in openapi_methods:
                missing.append(f"{method} {full_path}")
                continue
            if method not in openapi_methods[full_path]:
                missing.append(f"{method} {full_path}")

    require(errors, not missing, "openapi 与路由存在未同步 path/method: " + ", ".join(missing[:12]))


def validate_openapi(content: str, errors: List[str]) -> None:
    validate_route_openapi_sync(content, errors)
    require(errors, "/incidents/{id}/dismiss:" in content, "openapi 缺少 /incidents/{id}/dismiss")
    require(errors, "/incidents/{id}/trigger:" in content, "openapi 缺少 /incidents/{id}/trigger")
    require(errors, "/healing/instances/stats:" in content, "openapi 缺少 /healing/instances/stats")
    require(errors, "/healing/pending/trigger:" in content, "openapi 缺少 /healing/pending/trigger")
    require(errors, "/healing/pending/dismissed:" in content, "openapi 缺少 /healing/pending/dismissed")
    require(
        errors,
        has_regex(
            content,
            rf"/incidents/stats:.*?dismissed:\s+type: integer",
        ),
        "openapi 的工单统计缺少 dismissed 计数",
    )
    require(
        errors,
        has_regex(
            content,
            rf"/incidents:\s+get:.*?name: healing_status.*?enum: \[{INCIDENT_HEALING_ENUM}\]",
        ),
        "openapi 的工单列表缺少完整 healing_status 枚举",
    )
    require(
        errors,
        has_regex(
            content,
            rf"/incidents/batch-reset-scan:.*?healing_status:.*?enum: \[{INCIDENT_HEALING_ENUM}\]",
        ),
        "openapi 的批量重置缺少完整 healing_status 枚举",
    )
    require(
        errors,
        has_regex(
            content,
            rf"Incident:\s+type: object.*?healing_status:\s+type: string\s+enum: \[{INCIDENT_HEALING_ENUM}\]",
        ),
        "openapi 的 Incident schema 缺少 dismissed",
    )
    require(
        errors,
        has_regex(
            content,
            rf"/healing/instances:\s+get:.*?name: status.*?enum: \[{FLOW_INSTANCE_ENUM}\]",
        ),
        "openapi 的流程实例列表状态枚举不完整",
    )
    require(
        errors,
        has_regex(
            content,
            r"/healing/instances/stats:.*?by_status:",
        ),
        "openapi 的流程实例统计缺少 by_status 响应",
    )
    require(errors, "/healing/instances/{id}/logs:" not in content, "openapi 仍声明了不存在的 /healing/instances/{id}/logs")
    require(
        errors,
        has_regex(content, r"responses:\s+Success:.*?code:\s+type: integer.*?message:",),
        "openapi 的 Success 响应未声明顶层 code/message",
    )
    require(
        errors,
        has_regex(content, r"PaginatedResponse:\s+type: object.*?total:\s+type: integer.*?page:\s+type: integer.*?page_size:\s+type: integer",),
        "openapi 的 PaginatedResponse 仍未对齐顶层 total/page/page_size",
    )
    require(
        errors,
        has_regex(content, r"Error:\s+type: object.*?code:\s+type: integer.*?message:\s+type: string",),
        "openapi 的 Error schema 仍未对齐顶层 code/message",
    )
    require(
        errors,
        has_regex(content, r"/healing/approvals:\s+get:.*?name: flow_instance_id",),
        "openapi 的审批任务列表缺少 flow_instance_id 参数",
    )
    require(
        errors,
        has_regex(content, r"/healing/approvals/pending:\s+get:.*?name: node_name.*?name: date_from.*?name: date_to",),
        "openapi 的待审批列表缺少 node_name/date_from/date_to 参数",
    )
    require(
        errors,
        has_regex(content, r"/healing/approvals:\s+get:.*?properties:\s+data:\s+type: array",),
        "openapi 的审批任务列表响应仍未使用顶层 data 数组",
    )
    require(
        errors,
        has_regex(content, r"/healing/approvals/pending:\s+get:.*?properties:\s+data:\s+type: array",),
        "openapi 的待审批列表响应仍未使用顶层 data 数组",
    )
    require(
        errors,
        has_regex(content, r"/execution-tasks:.*?required: \[playbook_id, target_hosts\].*?playbook_id:",),
        "openapi 的执行任务创建请求仍未对齐 playbook_id",
    )
    require(errors, "default_recipients:" not in content, "openapi 仍使用 default_recipients 旧字段")
    require(
        errors,
        has_regex(content, r"/auth/login:.*?access_token:"),
        "openapi 缺少 auth login 原始 token 响应定义",
    )
    require(
        errors,
        has_regex(content, rf"/templates:\s+get:.*?name: event_type.*?enum: \[{NOTIFICATION_EVENT_TYPES}\]"),
        "openapi 的通知模板 event_type 枚举仍未对齐真实字典值",
    )
    require(
        errors,
        has_regex(content, r"/templates:\s+get:.*?name: name.*?name: supported_channel.*?name: is_active.*?name: sort_by.*?name: sort_order"),
        "openapi 的通知模板列表筛选参数仍未对齐 name/supported_channel/is_active/sort",
    )
    require(
        errors,
        has_regex(content, rf"NotificationTemplate:\s+type: object.*?event_type:\s+type: string\s+enum: \[{NOTIFICATION_EVENT_TYPES}\].*?supported_channels:"),
        "openapi 的 NotificationTemplate schema 未对齐 event_type/supported_channels",
    )
    require(
        errors,
        has_regex(content, rf"NotificationTemplateCreate:\s+type: object.*?required: \[name, body_template\].*?event_type:\s+type: string\s+enum: \[{NOTIFICATION_EVENT_TYPES}\].*?supported_channels:"),
        "openapi 的 NotificationTemplateCreate schema 未对齐真实请求字段",
    )
    require(
        errors,
        has_regex(content, rf"NotificationTemplateUpdate:\s+type: object.*?event_type:\s+type: string\s+enum: \[{NOTIFICATION_EVENT_TYPES}\].*?supported_channels:"),
        "openapi 的 NotificationTemplateUpdate schema 未对齐真实请求字段",
    )
    require(
        errors,
        has_regex(content, r"/notifications/send:\s+post:.*?notification_ids:.*?logs:"),
        "openapi 的通知发送响应仍未对齐 notification_ids/logs",
    )
    require(
        errors,
        has_regex(content, r"/notifications:\s+get:.*?name: subject.*?name: execution_run_id.*?name: created_after.*?name: created_before"),
        "openapi 的通知日志列表筛选参数仍未对齐真实 handler",
    )
    require(
        errors,
        has_regex(content, r"/notifications:\s+get:.*?name: status.*?enum: \[pending, sent, delivered, failed, bounced\]"),
        "openapi 的通知日志状态枚举仍未对齐真实字典值",
    )
    require(
        errors,
        has_regex(content, r"Notification:\s+type: object.*?execution_run_id:.*?status:\s+type: string\s+enum: \[pending, sent, delivered, failed, bounced\]"),
        "openapi 的 Notification schema 未对齐 execution_run_id 或状态枚举",
    )
    require(errors, "/notifications/{id}/retry:" not in content, "openapi 仍声明了不存在的 /notifications/{id}/retry")
    require(errors, "/notifications/stats:" in content, "openapi 缺少 /notifications/stats")
    require(errors, "/incidents/{id}/close:" in content, "openapi 缺少 /incidents/{id}/close")
    require(errors, "/healing/instances/{id}/retry:" in content, "openapi 缺少 /healing/instances/{id}/retry")
    require(errors, "/healing/instances/{id}/events:" in content, "openapi 缺少 /healing/instances/{id}/events")
    for path in [
        "/audit-logs:",
        "/plugins/search-schema:",
        "/incidents/search-schema:",
        "/secrets-sources/stats:",
        "/git-repos/stats:",
        "/git-repos/search-schema:",
        "/playbooks/stats:",
        "/execution-tasks/stats:",
        "/execution-tasks/search-schema:",
        "/execution-runs:",
        "/execution-runs/search-schema:",
    ]:
        require(errors, path in content, f"openapi 缺少 {path.rstrip(':')}")
    require(
        errors,
        has_regex(content, r"/execution-runs/\{id\}/logs:\s+get:.*?allOf:.*?data:\s+type: array"),
        "openapi 的执行日志响应仍未对齐 success.data[] 包装",
    )
    require(
        errors,
        has_regex(content, r"/template-variables:\s+get:.*?allOf:.*?variables:\s+type: array"),
        "openapi 的 template-variables 响应仍未对齐 success.data.variables",
    )
    for path in [
        "/execution-runs/stats:",
        "/execution-runs/trend:",
        "/execution-runs/trigger-distribution:",
        "/execution-runs/top-failed:",
        "/execution-runs/top-active:",
    ]:
        require(errors, path in content, f"openapi 缺少 {path.rstrip(':')}")


def validate_incidents_doc(content: str, errors: List[str]) -> None:
    healing_rows = re.findall(r"^\| `healing_status` \| .*$", content, re.M)
    require(errors, len(healing_rows) >= 2, "incidents.md 缺少 healing_status 文档行")
    require(
        errors,
        all(INCIDENT_HEALING_MD in row for row in healing_rows),
        "incidents.md 的 healing_status 文档未同步为 processing/healed/dismissed",
    )
    require(errors, '"dismissed":' in content, "incidents.md 的统计示例缺少 dismissed")
    require(errors, "**POST** `/api/v1/incidents/:id/dismiss`" in content, "incidents.md 缺少 dismiss 接口")
    require(errors, "**POST** `/api/v1/incidents/:id/close`" in content, "incidents.md 缺少 close 接口")


def validate_plugins_doc(content: str, errors: List[str]) -> None:
    healing_rows = re.findall(r"^\| `healing_status` \| .*$", content, re.M)
    require(errors, len(healing_rows) >= 2, "plugins.md 缺少 healing_status 文档行")
    require(
        errors,
        all(INCIDENT_HEALING_MD in row for row in healing_rows),
        "plugins.md 的 healing_status 文档未同步为 processing/healed/dismissed",
    )
    require(errors, '"healing_status": "processing"' in content, "plugins.md 的工单示例未更新为合法状态值")


def validate_healing_doc(content: str, errors: List[str]) -> None:
    require(
        errors,
        f"| `status` | string | ❌ | 状态：{FLOW_INSTANCE_MD} |" in content,
        "healing.md 的流程实例状态说明未同步为 waiting_approval/completed",
    )
    require(errors, '"by_status": [' in content, "healing.md 的实例统计示例未同步为 by_status")
    require(errors, "**GET** `/api/v1/healing/pending/dismissed`" in content, "healing.md 缺少 dismissed 列表接口")
    require(errors, "**POST** `/api/v1/incidents/:id/dismiss`" in content, "healing.md 缺少 dismiss 接口")
    require(errors, "**POST** `/api/v1/healing/instances/:id/retry`" in content, "healing.md 缺少 retry 接口")
    require(errors, "**GET** `/api/v1/healing/instances/:id/events`" in content, "healing.md 缺少实例事件 SSE 接口")
    require(errors, "**权限**: `healing:flows:update`" in content.split("### 21. 取消实例", 1)[1].split("### 23. 实例事件 SSE 流", 1)[0], "healing.md 的 cancel/retry 权限仍未对齐 healing:flows:update")
    require(errors, "| `waiting_approval` | 等待审批 |" in content, "healing.md 缺少 waiting_approval 状态说明")
    require(errors, "| `completed` | 已完成 |" in content, "healing.md 缺少 completed 状态说明")
    require(errors, "| `status` | string | ❌ | 状态：`pending` / `approved` / `rejected` / `expired` |" in content, "healing.md 的审批状态仍错误写成 timeout")
    require(errors, "| `node_name` | string | ❌ | 按节点名称模糊筛选（匹配 `node_id`） |" in content, "healing.md 的待审批筛选参数仍未对齐 node_name")


def validate_api_readme(content: str, errors: List[str]) -> None:
    require(errors, "jq -r '.access_token'" in content, "docs/api/README.md 登录示例仍在读取 .data.access_token")
    require(errors, "除 `/auth/login` 与 `/auth/refresh` 外" in content, "docs/api/README.md 未说明 auth 接口的响应特例")


def validate_auth_doc(content: str, errors: List[str]) -> None:
    require(errors, '"code": 0' not in content.split("## 1. 用户登录", 1)[1].split("## 2. 刷新 Token", 1)[0], "docs/api/auth.md 登录示例仍使用统一包装响应")
    require(errors, '"code": 0' not in content.split("## 2. 刷新 Token", 1)[1].split("## 3. 用户登出", 1)[0], "docs/api/auth.md 刷新示例仍使用统一包装响应")
    require(errors, "登录接口保持原始返回格式" in content, "docs/api/auth.md 未说明登录原始响应格式")
    require(errors, "刷新接口与登录接口一样" in content, "docs/api/auth.md 未说明 refresh 原始响应格式")
    require(errors, '"code": 40000' in content, "docs/api/auth.md 的错误码示例仍使用 HTTP 状态码")


def validate_platform_users_doc(content: str, errors: List[str]) -> None:
    require(errors, "**权限**: `platform:users:list`" in content, "docs/api/platform-users.md 缺少 platform:users:list")
    require(errors, "**权限**: `platform:users:create`" in content, "docs/api/platform-users.md 缺少 platform:users:create")
    require(errors, "**权限**: `platform:users:update`" in content, "docs/api/platform-users.md 缺少 platform:users:update")
    require(errors, "**权限**: `platform:users:delete`" in content, "docs/api/platform-users.md 缺少 platform:users:delete")
    require(errors, "**权限**: `platform:users:reset_password`" in content, "docs/api/platform-users.md 缺少 platform:users:reset_password")
    require(errors, "**权限**: `platform:roles:manage`" in content, "docs/api/platform-users.md 缺少 platform:roles:manage")
    require(errors, "`user:`" not in content, "docs/api/platform-users.md 仍残留旧 user:* 权限名")
    require(errors, "| `name` | string | ❌ | 模糊搜索（用户名、显示名称） |" in content, "docs/api/platform-users.md 的 simple users 查询参数仍未对齐 name")
    require(errors, '{"id": "uuid", "username": "admin", "display_name": "管理员"}' in content, "docs/api/platform-users.md 的 simple users 响应仍未对齐 display_name")
    create_section = content.split("## 2. 创建用户", 1)[1].split("## 3. 获取简单用户列表", 1)[0]
    require(errors, "| `role_id` | uuid | ❌ | 初始分配的平台角色 ID |" in create_section, "docs/api/platform-users.md 的创建用户请求仍未对齐 role_id")
    require(errors, "| `role_ids` |" not in create_section, "docs/api/platform-users.md 的创建用户请求仍残留 role_ids")


def validate_platform_roles_doc(content: str, errors: List[str]) -> None:
    require(errors, "**权限**: `platform:roles:list`" in content, "docs/api/platform-roles.md 缺少 platform:roles:list")
    require(errors, content.count("**权限**: `platform:roles:manage`") >= 4, "docs/api/platform-roles.md 缺少 platform:roles:manage 权限说明")
    require(errors, content.count("**权限**: `platform:permissions:list`") >= 2, "docs/api/platform-roles.md 缺少 platform:permissions:list 权限说明")
    require(errors, "`role:list`" not in content, "docs/api/platform-roles.md 仍残留旧 role:list 权限")
    require(errors, "`role:create`" not in content, "docs/api/platform-roles.md 仍残留旧 role:create 权限")
    require(errors, "`role:update`" not in content, "docs/api/platform-roles.md 仍残留旧 role:update 权限")
    require(errors, "`role:delete`" not in content, "docs/api/platform-roles.md 仍残留旧 role:delete 权限")
    require(errors, "`role:assign`" not in content, "docs/api/platform-roles.md 仍残留旧 role:assign 权限")
    require(errors, "无特殊要求" not in content, "docs/api/platform-roles.md 仍残留无特殊要求说明")
    require(errors, '"code": "platform:users:list"' in content, "docs/api/platform-roles.md 的权限树示例仍未对齐 platform:* 代码")


def validate_site_messages_doc(content: str, errors: List[str]) -> None:
    require(errors, "**GET** `/api/v1/tenant/site-messages/unread-count`" in content, "docs/api/site-messages.md 未对齐 tenant unread-count")
    require(errors, "**GET** `/api/v1/common/site-messages/categories`" in content, "docs/api/site-messages.md 未对齐 common categories")
    require(errors, "**GET** `/api/v1/platform/site-messages/settings`" in content, "docs/api/site-messages.md 未对齐 platform settings GET")
    require(errors, "**PUT** `/api/v1/platform/site-messages/settings`" in content, "docs/api/site-messages.md 未对齐 platform settings PUT")
    require(errors, "**GET** `/api/v1/tenant/site-messages`" in content, "docs/api/site-messages.md 未对齐 tenant list")
    require(errors, "**POST** `/api/v1/platform/site-messages`" in content, "docs/api/site-messages.md 未对齐 platform create")
    require(errors, "**权限**: `site-message:list`" in content, "docs/api/site-messages.md 缺少 site-message:list")
    require(errors, "**权限**: `site-message:settings:view`" in content, "docs/api/site-messages.md 缺少 site-message:settings:view")
    require(errors, "**权限**: `site-message:settings:manage`" in content, "docs/api/site-messages.md 缺少 site-message:settings:manage")
    require(errors, "**权限**: `platform:messages:send`" in content, "docs/api/site-messages.md 缺少 platform:messages:send")
    require(errors, "**GET** `/api/v1/site-messages/categories`" not in content, "docs/api/site-messages.md 仍使用过期 categories 路径")
    require(errors, "**GET** `/api/v1/site-messages/settings`" not in content, "docs/api/site-messages.md 仍使用过期 settings 路径")
    require(errors, "**POST** `/api/v1/site-messages`" not in content, "docs/api/site-messages.md 仍使用过期 create 路径")


def validate_secrets_doc(content: str, errors: List[str]) -> None:
    require(errors, '"status": "active"' in content, "docs/api/secrets.md 的状态示例仍未对齐 active")
    require(errors, '"status": "enabled"' not in content, "docs/api/secrets.md 仍使用 enabled")
    require(errors, "| `status` | string | ❌ | 状态：`active` / `inactive` |" in content, "docs/api/secrets.md 缺少 status 枚举说明")
    query_section = content.split("### 11. 查询密钥值", 1)[1]
    require(errors, "**权限**: `secrets:query`" in query_section, "docs/api/secrets.md 的 secrets/query 权限仍未对齐 secrets:query")


def validate_audit_docs(audit_content: str, platform_content: str, errors: List[str]) -> None:
    require(errors, "**权限**: `audit:list`" in audit_content, "docs/api/audit-logs.md 缺少 audit:list")
    require(errors, "**权限**: `audit:export`" in audit_content.split("## 9. 导出审计日志（CSV）", 1)[1], "docs/api/audit-logs.md 的导出权限仍未对齐 audit:export")
    require(errors, "**权限**: `platform:audit:list`" in platform_content, "docs/api/platform-audit-logs.md 缺少 platform:audit:list")


def validate_common_docs(errors: List[str]) -> None:
    api_readme = read_text(API_README_PATH)
    execution_doc = read_text(EXECUTION_DOC_PATH)
    notifications_doc = read_text(NOTIFICATIONS_DOC_PATH)
    git_repos_doc = read_text(GIT_REPOS_DOC_PATH)
    playbooks_doc = read_text(PLAYBOOKS_DOC_PATH)
    dashboard_doc = read_text(DASHBOARD_DOC_PATH)
    site_messages_doc = read_text(SITE_MESSAGES_DOC_PATH)
    secrets_doc = read_text(SECRETS_DOC_PATH)
    platform_users_doc = read_text(PLATFORM_USERS_DOC_PATH)
    platform_roles_doc = read_text(PLATFORM_ROLES_DOC_PATH)
    audit_doc = read_text(AUDIT_LOGS_DOC_PATH)
    platform_audit_doc = read_text(PLATFORM_AUDIT_DOC_PATH)

    require(errors, '"message": "success"' in api_readme, "docs/api/README.md 缺少通用成功 message 示例")
    require(errors, '"data": [...]' in api_readme, "docs/api/README.md 的列表响应仍是旧的 data.items 结构")
    require(errors, '"code": 40000' in api_readme, "docs/api/README.md 的错误码示例仍使用 HTTP 状态码")
    require(errors, "`site-message:create`" not in api_readme, "docs/api/README.md 的权限说明仍残留旧 site-message:create")
    require(errors, "`role:list` / `role:create` / `role:update` / `role:delete` / `role:assign`" in api_readme, "docs/api/README.md 的租户角色权限说明仍未补齐")
    require(errors, "`platform:roles:list` / `platform:roles:manage` / `platform:permissions:list`" in api_readme, "docs/api/README.md 缺少平台角色权限说明")
    require(errors, "`platform:messages:send`" in api_readme, "docs/api/README.md 缺少平台站内信权限说明")
    require(errors, "`site-message:list` / `site-message:settings:view` / `site-message:settings:manage`" in api_readme, "docs/api/README.md 缺少站内信权限说明")
    require(errors, '"message": "success"' in execution_doc, "docs/api/execution.md 缺少列表响应 message 示例")
    require(errors, '"data": [' in execution_doc, "docs/api/execution.md 的列表响应仍是旧的 data.items 结构")
    require(errors, '"executor_type": "local"' in execution_doc, "docs/api/execution.md 的执行器示例仍未对齐 local/docker")
    require(errors, '"message": "success"' in notifications_doc, "docs/api/notifications.md 缺少列表响应 message 示例")
    require(errors, '"data": [' in notifications_doc, "docs/api/notifications.md 的列表响应仍是旧的 data.items 结构")
    require(errors, '"is_active": true' in notifications_doc, "docs/api/notifications.md 的渠道示例仍未对齐 is_active")
    require(errors, "default_recipients" not in notifications_doc, "docs/api/notifications.md 仍使用 default_recipients 旧字段")
    require(errors, "| `supported_channels` | []string | ❌ | 支持的渠道类型列表：`email` / `dingtalk` / `webhook` |" in notifications_doc, "docs/api/notifications.md 的模板字段仍未对齐 supported_channels")
    require(errors, "| `event_type` | string | ❌ | 事件类型（见下方枚举） |" in notifications_doc, "docs/api/notifications.md 的模板 event_type 仍错误标为必填")
    for event_type in ["incident_created", "incident_resolved", "approval_required", "execution_result", "custom"]:
        require(errors, f"| `{event_type}` |" in notifications_doc, f"docs/api/notifications.md 缺少通知事件类型 `{event_type}`")
    require(errors, NOTIFICATION_LOG_STATUSES_MD in notifications_doc, "docs/api/notifications.md 的通知日志状态枚举仍未对齐 sent/delivered/bounced")
    require(errors, "| `supported_channel` | string | ❌ | 支持的渠道类型：`email` / `dingtalk` / `webhook` |" in notifications_doc, "docs/api/notifications.md 缺少 supported_channel 筛选参数")
    require(errors, "| `execution_run_id` | uuid | ❌ | 按执行记录筛选 |" in notifications_doc, "docs/api/notifications.md 仍未对齐 execution_run_id 筛选参数")
    require(errors, "| `subject` | string | ❌ | 按通知标题模糊搜索 |" in notifications_doc, "docs/api/notifications.md 仍未对齐 subject 筛选参数")
    require(errors, "**权限**: `channel:update`" in notifications_doc.split("### 6. 测试渠道", 1)[1].split("## 通知模板（Templates）", 1)[0], "docs/api/notifications.md 的测试渠道权限仍未对齐 channel:update")
    require(errors, "**权限**: `task:create`" in execution_doc.split("### 2. 创建任务模板", 1)[1].split("#### 请求体", 1)[0], "docs/api/execution.md 的创建任务模板权限仍未对齐 task:create")
    require(errors, "**权限**: `repository:validate`" in git_repos_doc.split("## 1. 验证仓库连接", 1)[1].split("### 请求体", 1)[0], "docs/api/git-repos.md 的 validate 权限仍未对齐 repository:validate")
    require(errors, "**权限**: `repository:list`" in git_repos_doc.split("## 2. 获取仓库列表", 1)[1].split("### 查询参数", 1)[0], "docs/api/git-repos.md 的列表权限仍未对齐 repository:list")
    require(errors, "**权限**: `repository:create`" in git_repos_doc.split("## 3. 创建仓库", 1)[1].split("### 请求体", 1)[0], "docs/api/git-repos.md 的创建权限仍未对齐 repository:create")
    require(errors, "plugin:" not in git_repos_doc, "docs/api/git-repos.md 仍残留旧 plugin:* 权限名")
    require(errors, "**权限**: `playbook:list`" in playbooks_doc.split("## 1. 获取 Playbook 列表", 1)[1].split("### 查询参数", 1)[0], "docs/api/playbooks.md 的列表权限仍未对齐 playbook:list")
    require(errors, "**权限**: `playbook:create`" in playbooks_doc.split("## 2. 创建 Playbook", 1)[1].split("### 请求体", 1)[0], "docs/api/playbooks.md 的创建权限仍未对齐 playbook:create")
    require(errors, "plugin:" not in playbooks_doc, "docs/api/playbooks.md 仍残留旧 plugin:* 权限名")
    require(errors, "无特殊权限要求" not in dashboard_doc, "docs/api/dashboard.md 仍残留无特殊权限要求的过期说明")
    require(errors, "**权限**: `dashboard:view`" in dashboard_doc, "docs/api/dashboard.md 缺少 dashboard:view 权限说明")
    require(errors, "**权限**: `dashboard:workspace:manage`" in dashboard_doc, "docs/api/dashboard.md 缺少 dashboard:workspace:manage 权限说明")
    require(errors, "**权限**: `dashboard:config:manage`" in dashboard_doc, "docs/api/dashboard.md 缺少 dashboard:config:manage 权限说明")
    validate_platform_users_doc(platform_users_doc, errors)
    validate_platform_roles_doc(platform_roles_doc, errors)
    validate_site_messages_doc(site_messages_doc, errors)
    validate_secrets_doc(secrets_doc, errors)
    validate_audit_docs(audit_doc, platform_audit_doc, errors)


def main() -> int:
    errors: List[str] = []
    validate_openapi(read_text(OPENAPI_PATH), errors)
    validate_api_readme(read_text(API_README_PATH), errors)
    validate_auth_doc(read_text(AUTH_DOC_PATH), errors)
    validate_incidents_doc(read_text(INCIDENTS_DOC_PATH), errors)
    validate_plugins_doc(read_text(PLUGINS_DOC_PATH), errors)
    validate_healing_doc(read_text(HEALING_DOC_PATH), errors)
    validate_common_docs(errors)

    if errors:
        for message in errors:
            print(f"[FAIL] {message}")
        return 1

    print("[OK] API 契约与文档关键面校验通过")
    return 0


if __name__ == "__main__":
    sys.exit(main())
