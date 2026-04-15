#!/usr/bin/env python3

from pathlib import Path
import re
import subprocess
import sys
from typing import Any, Dict, List, Optional
import yaml

ROOT = Path(__file__).resolve().parents[1]
OPENAPI_PATH = ROOT / "api/openapi.yaml"
OPENAPI_BUILD_SCRIPT = ROOT / "tools/build_openapi.py"
DICTIONARY_CONTRACT_SCRIPT = ROOT / "tools/validate_dictionary_contracts.py"
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
NOTIFICATION_EVENT_TYPES = "execution_started, execution_result, flow_result, approval_required, manual_notification"
NOTIFICATION_EVENT_TYPES_MD = "`execution_started` / `execution_result` / `flow_result` / `approval_required` / `manual_notification`"
NOTIFICATION_LOG_STATUSES_MD = "`pending` / `sent` / `delivered` / `failed` / `bounced`"
NOTIFICATION_CHANNEL_TYPES_MD = "`email` / `dingtalk` / `wecom` / `slack` / `teams` / `webhook`"


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def ensure_openapi_bundle_fresh(errors: List[str]) -> None:
    result = subprocess.run(
        [sys.executable, str(OPENAPI_BUILD_SCRIPT), "--check"],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    if result.returncode == 0:
        return
    output = (result.stdout + result.stderr).strip()
    errors.append(output or "openapi bundle freshness check failed")


def ensure_dictionary_contracts(errors: List[str]) -> None:
    result = subprocess.run(
        [sys.executable, str(DICTIONARY_CONTRACT_SCRIPT)],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    if result.returncode == 0:
        return
    output = (result.stdout + result.stderr).strip()
    errors.append(output or "dictionary contracts check failed")


def require(errors: List[str], condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)


def has_regex(content: str, pattern: str) -> bool:
    return re.search(pattern, content, re.S) is not None


def load_openapi_document(content: str, errors: List[str]) -> Optional[Dict[str, Any]]:
    try:
        document = yaml.safe_load(content)
    except yaml.YAMLError as exc:
        errors.append(f"openapi.yaml 不是合法 YAML: {exc}")
        return None
    if not isinstance(document, dict):
        errors.append("openapi.yaml 顶层结构必须是 object")
        return None
    paths = document.get("paths")
    if not isinstance(paths, dict):
        errors.append("openapi.yaml 缺少合法的 paths 对象")
        return None
    return document


def parse_openapi_methods(document: Dict[str, Any]) -> Dict[str, set]:
    openapi_methods = {}
    for path, item in document.get("paths", {}).items():
        if not isinstance(item, dict):
            continue
        methods = {method.upper() for method in item.keys() if method.lower() in {"get", "post", "put", "patch", "delete"}}
        openapi_methods[path] = methods
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


def validate_route_openapi_sync(document: Dict[str, Any], errors: List[str]) -> None:
    openapi_methods = parse_openapi_methods(document)
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


def get_operation(document: Dict[str, Any], path: str, method: str) -> Dict[str, Any]:
    path_item = document.get("paths", {}).get(path, {})
    operation = path_item.get(method.lower())
    return operation if isinstance(operation, dict) else {}


def get_parameter(operation: Dict[str, Any], name: str) -> Optional[Dict[str, Any]]:
    for parameter in operation.get("parameters", []):
        if parameter.get("name") == name:
            return parameter
    return None


def get_parameter_enum(operation: Dict[str, Any], name: str) -> List[str]:
    parameter = get_parameter(operation, name)
    if not parameter:
        return []
    return parameter.get("schema", {}).get("enum", [])


def get_request_property(operation: Dict[str, Any], property_name: str) -> Dict[str, Any]:
    schema = operation.get("requestBody", {}).get("content", {}).get("application/json", {}).get("schema", {})
    return schema.get("properties", {}).get(property_name, {})


def get_schema(document: Dict[str, Any], name: str) -> Dict[str, Any]:
    schema = document.get("components", {}).get("schemas", {}).get(name, {})
    return schema if isinstance(schema, dict) else {}


def get_schema_property(document: Dict[str, Any], schema_name: str, property_name: str) -> Dict[str, Any]:
    return get_schema(document, schema_name).get("properties", {}).get(property_name, {})


def collect_bad_refs(document: Dict[str, Any]) -> List[str]:
    refs: List[tuple[str, Any]] = []

    def walk(node: Any, path: str = "") -> None:
        if isinstance(node, dict):
            if "$ref" in node:
                refs.append((path, node["$ref"]))
            for key, value in node.items():
                walk(value, f"{path}/{key}")
        elif isinstance(node, list):
            for index, value in enumerate(node):
                walk(value, f"{path}/{index}")

    walk(document)
    bad: List[str] = []
    for path, ref in refs:
        if not isinstance(ref, str) or not ref.startswith("#/"):
            bad.append(f"{path} -> {ref}")
            continue
        current: Any = document
        ok = True
        for part in ref[2:].split("/"):
            if isinstance(current, dict) and part in current:
                current = current[part]
                continue
            ok = False
            break
        if not ok:
            bad.append(f"{path} -> {ref}")
    return bad


def response_schema(document: Dict[str, Any], path: str, method: str, status: str) -> Dict[str, Any]:
    return (
        get_operation(document, path, method)
        .get("responses", {})
        .get(status, {})
        .get("content", {})
        .get("application/json", {})
        .get("schema", {})
    )


def validate_openapi(content: str, document: Optional[Dict[str, Any]], errors: List[str]) -> None:
    if document is None:
        return
    validate_route_openapi_sync(document, errors)
    bad_refs = collect_bad_refs(document)
    require(errors, not bad_refs, "openapi 存在坏 $ref: " + ", ".join(bad_refs[:12]))
    operation_ids = []
    missing_operation_ids = []
    for path, item in document.get("paths", {}).items():
        if not isinstance(item, dict):
            continue
        for method, operation in item.items():
            if method.lower() not in {"get", "post", "put", "patch", "delete"}:
                continue
            if not isinstance(operation, dict):
                continue
            op_id = operation.get("operationId")
            if not isinstance(op_id, str) or not op_id.strip():
                missing_operation_ids.append(f"{method.upper()} {path}")
                continue
            operation_ids.append(op_id)
    require(errors, not missing_operation_ids, "openapi 存在缺失 operationId 的操作: " + ", ".join(missing_operation_ids[:12]))
    require(errors, len(operation_ids) == len(set(operation_ids)), "openapi 存在重复 operationId")
    servers = document.get("servers", [])
    require(errors, isinstance(servers, list) and len(servers) == 1, "openapi servers 必须收口为单一稳定入口")
    if isinstance(servers, list) and servers:
        server_url = servers[0].get("url")
        require(errors, server_url == "/api/v1", "openapi server url 必须为 /api/v1")
    execution_tasks_post = get_operation(document, "/tenant/execution-tasks", "post")
    require(errors, "responses" in execution_tasks_post, "openapi 的 POST /tenant/execution-tasks 缺少 responses")
    execution_tasks_list = response_schema(document, "/tenant/execution-tasks", "get", "200")
    require(errors, execution_tasks_list.get("allOf", [{}])[0].get("$ref") == "#/components/schemas/PaginatedResponse", "openapi 的执行任务列表响应未对齐统一分页壳")
    execution_schedules_list = response_schema(document, "/tenant/execution-schedules", "get", "200")
    require(errors, execution_schedules_list.get("allOf", [{}])[0].get("$ref") == "#/components/schemas/PaginatedResponse", "openapi 的调度列表响应未对齐统一分页壳")
    git_repos_list = response_schema(document, "/tenant/git-repos", "get", "200")
    require(errors, git_repos_list.get("allOf", [{}])[0].get("$ref") == "#/components/schemas/PaginatedResponse", "openapi 的 Git 仓库列表响应未对齐统一分页壳")
    cmdb_batch_test = response_schema(document, "/tenant/cmdb/batch-test-connection", "post", "200")
    require(errors, cmdb_batch_test.get("allOf", [{}])[0].get("$ref") == "#/components/schemas/Success", "openapi 的 CMDB 批量连接测试响应未对齐统一 success.data 壳")
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
    require(errors, "healing_status" in get_schema(document, "Incident").get("properties", {}), "openapi 的 Incident schema 缺少 healing_status 字段")
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
    execution_task_schema = get_operation(document, "/tenant/execution-tasks", "post").get("requestBody", {}).get("content", {}).get("application/json", {}).get("schema", {})
    require(errors, execution_task_schema.get("required") == ["playbook_id", "target_hosts"] and "playbook_id" in execution_task_schema.get("properties", {}), "openapi 的执行任务创建请求仍未对齐 playbook_id")
    require(errors, "default_recipients:" not in content, "openapi 仍使用 default_recipients 旧字段")
    require(
        errors,
        has_regex(content, r"/auth/login:.*?allOf:.*?LoginPayload"),
        "openapi 的 auth login 响应仍未对齐统一 success.data 包装",
    )
    require(
        errors,
        has_regex(content, r"/auth/refresh:.*?allOf:.*?LoginPayload"),
        "openapi 的 auth refresh 响应仍未对齐统一 success.data 包装",
    )
    require(
        errors,
        has_regex(content, r"/templates:\s+get:.*?name: name.*?name: supported_channel.*?name: is_active.*?name: sort_by.*?name: sort_order"),
        "openapi 的通知模板列表筛选参数仍未对齐 name/supported_channel/is_active/sort",
    )
    template_schema = get_schema(document, "NotificationTemplate")
    template_create_schema = get_schema(document, "NotificationTemplateCreate")
    template_update_schema = get_schema(document, "NotificationTemplateUpdate")
    require(errors, "event_type" in template_schema.get("properties", {}) and "supported_channels" in template_schema.get("properties", {}), "openapi 的 NotificationTemplate schema 未对齐 event_type/supported_channels")
    require(errors, template_create_schema.get("required") == ["name", "body_template"] and "event_type" in template_create_schema.get("properties", {}) and "supported_channels" in template_create_schema.get("properties", {}), "openapi 的 NotificationTemplateCreate schema 未对齐真实请求字段")
    require(errors, "event_type" in template_update_schema.get("properties", {}) and "supported_channels" in template_update_schema.get("properties", {}), "openapi 的 NotificationTemplateUpdate schema 未对齐真实请求字段")
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
    notification_schema = get_schema(document, "Notification")
    require(errors, "execution_run_id" in notification_schema.get("properties", {}) and "status" in notification_schema.get("properties", {}), "openapi 的 Notification schema 未对齐 execution_run_id 或状态字段")
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
    require(errors, "jq -r '.data.access_token'" in content, "docs/api/README.md 登录示例仍未读取 .data.access_token")
    require(errors, "业务接口统一使用如下响应包裹格式" in content, "docs/api/README.md 仍保留 auth 响应特例说明")


def validate_auth_doc(content: str, errors: List[str]) -> None:
    login_section = content.split("## 1. 用户登录", 1)[1].split("## 2. 刷新 Token", 1)[0]
    refresh_section = content.split("## 2. 刷新 Token", 1)[1].split("## 3. 用户登出", 1)[0]
    require(errors, '"code": 0' in login_section and '"data": {' in login_section, "docs/api/auth.md 登录示例仍未使用统一包装响应")
    require(errors, '"code": 0' in refresh_section and '"data": {' in refresh_section, "docs/api/auth.md 刷新示例仍未使用统一包装响应")
    require(errors, "原始返回格式" not in content, "docs/api/auth.md 仍说明登录/刷新走原始响应格式")
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
    require(errors, f"| `supported_channels` | []string | ❌ | 支持的渠道类型列表：{NOTIFICATION_CHANNEL_TYPES_MD} |" in notifications_doc, "docs/api/notifications.md 的模板字段仍未对齐 supported_channels")
    require(errors, "| `event_type` | string | ❌ | 事件类型（见下方枚举） |" in notifications_doc, "docs/api/notifications.md 的模板 event_type 仍错误标为必填")
    for event_type in ["execution_started", "execution_result", "flow_result", "approval_required", "manual_notification"]:
        require(errors, f"| `{event_type}` |" in notifications_doc, f"docs/api/notifications.md 缺少通知事件类型 `{event_type}`")
    require(errors, NOTIFICATION_LOG_STATUSES_MD in notifications_doc, "docs/api/notifications.md 的通知日志状态枚举仍未对齐 sent/delivered/bounced")
    require(errors, f"| `supported_channel` | string | ❌ | 支持的渠道类型：{NOTIFICATION_CHANNEL_TYPES_MD} |" in notifications_doc, "docs/api/notifications.md 缺少 supported_channel 筛选参数")
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
    ensure_openapi_bundle_fresh(errors)
    ensure_dictionary_contracts(errors)
    openapi_content = read_text(OPENAPI_PATH)
    openapi_document = load_openapi_document(openapi_content, errors)
    validate_openapi(openapi_content, openapi_document, errors)
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
