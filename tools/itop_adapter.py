#!/usr/bin/env python3
"""
iTop adapter for Auto-Healing generic plugins.

This process translates iTop REST/JSON into AHS-standard ITSM / CMDB JSON arrays.
"""

import json
import os
import re
import secrets
from base64 import b64encode
from datetime import datetime
from html import unescape
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any, Dict, Iterable, List, Optional
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, quote, urlparse
from urllib.request import Request, urlopen


def env(name: str, default: str = "") -> str:
    value = os.getenv(name, default).strip()
    if not value:
        raise RuntimeError(f"missing required environment variable: {name}")
    return value


def optional_env(name: str, default: str = "") -> str:
    return os.getenv(name, default).strip()


def parse_json_env(name: str) -> Dict[str, Any]:
    raw = optional_env(name, "{}") or "{}"
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise RuntimeError(f"{name} must be a JSON object")
    return value


def split_csv(raw: str) -> List[str]:
    return [item.strip() for item in raw.split(",") if item.strip()]


class AdapterConfig:
    def __init__(self) -> None:
        base_url = env("ITOP_BASE_URL", "http://itop").rstrip("/")
        rest_path = optional_env("ITOP_REST_PATH", "/webservices/rest.php") or "/webservices/rest.php"
        self.adapter_host = optional_env("ADAPTER_HOST", "0.0.0.0") or "0.0.0.0"
        self.adapter_port = int(optional_env("ADAPTER_PORT", "18085") or "18085")
        self.rest_endpoint = f"{base_url}{rest_path}"
        self.rest_version = optional_env("ITOP_REST_VERSION", "1.3") or "1.3"
        self.auth_user = env("ITOP_AUTH_USER", "admin")
        self.auth_pwd = env("ITOP_AUTH_PWD")
        self.ticket_class = optional_env("ITOP_TICKET_CLASS", "UserRequest") or "UserRequest"
        self.ticket_oql = optional_env("ITOP_TICKET_OQL", "SELECT UserRequest") or "SELECT UserRequest"
        self.assign_stimulus = optional_env("ITOP_ASSIGN_STIMULUS", "ev_assign") or "ev_assign"
        self.close_stimulus = optional_env("ITOP_CLOSE_STIMULUS", "ev_resolve") or "ev_resolve"
        self.final_close_stimulus = optional_env("ITOP_FINAL_CLOSE_STIMULUS", "ev_close") or "ev_close"
        self.close_fields = parse_json_env("ITOP_CLOSE_FIELDS_JSON")
        self.excluded_statuses = tuple(split_csv(optional_env("ITOP_LIST_EXCLUDE_STATUSES", "closed")))
        self.cmdb_classes = split_csv(optional_env("ITOP_CMDB_CLASSES", "Server,VirtualMachine,NetworkDevice,ApplicationSolution"))
        self.cmdb_oqls = parse_json_env("ITOP_CMDB_OQLS_JSON")
        self.cmdb_environment = optional_env("ITOP_CMDB_ENVIRONMENT", "production") or "production"
        self.demo_org_id = optional_env("ITOP_DEMO_ORG_ID", "3") or "3"
        self.demo_caller_id = optional_env("ITOP_DEMO_CALLER_ID", "9") or "9"
        self.demo_service_id = optional_env("ITOP_DEMO_SERVICE_ID", "2") or "2"
        self.demo_subcategory_id = optional_env("ITOP_DEMO_SUBCATEGORY_ID", "15") or "15"
        self.demo_agent_id = optional_env("ITOP_DEMO_AGENT_ID", "33") or "33"
        self.demo_ci_id = optional_env("ITOP_DEMO_CI_ID", "32") or "32"
        self.demo_ci_name = optional_env("ITOP_DEMO_CI_NAME", "e2e-target-01") or "e2e-target-01"
        self.demo_fault_base_url = optional_env("AHS_DEMO_FAULT_BASE_URL", "http://ssh-target-01:19081").rstrip("/")


class ITopError(RuntimeError):
    pass


class ITopClient:
    def __init__(self, config: AdapterConfig):
        self.config = config
        token = b64encode(f"{config.auth_user}:{config.auth_pwd}".encode("utf-8")).decode("ascii")
        self.auth_header = f"Basic {token}"

    def _call(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        body = f"json_data={quote(json.dumps(payload, ensure_ascii=False))}".encode("utf-8")
        request = Request(
            f"{self.config.rest_endpoint}?version={quote(self.config.rest_version)}",
            data=body,
            method="POST",
            headers={
                "Authorization": self.auth_header,
                "Content-Type": "application/x-www-form-urlencoded",
                "Accept": "application/json",
            },
        )
        try:
            with urlopen(request, timeout=30) as response:
                raw = response.read().decode("utf-8")
        except HTTPError as exc:
            raise ITopError(f"iTop HTTP {exc.code}: {exc.read().decode('utf-8', 'replace')}") from exc
        except URLError as exc:
            raise ITopError(f"iTop unavailable: {exc}") from exc
        data = json.loads(raw)
        if data.get("code") not in (0, "0", None):
            raise ITopError(json.dumps(data, ensure_ascii=False))
        return data

    def list_operations(self) -> Dict[str, Any]:
        return self._call({"operation": "list_operations"})

    def get_incidents(self) -> List[Dict[str, Any]]:
        response = self._call({
            "operation": "core/get",
            "class": self.config.ticket_class,
            "key": self.config.ticket_oql,
            "output_fields": "ref,title,description,status,request_type,impact,urgency,priority,origin,start_date,last_update,functionalcis_list,service_name,servicesubcategory_name,agent_name,team_name,caller_name,org_name,friendlyname",
        })
        incidents: List[Dict[str, Any]] = []
        for item in iter_objects(response):
            normalized = normalize_incident(item)
            if normalized["status"] in self.config.excluded_statuses:
                continue
            incidents.append(normalized)
        return incidents

    def get_incident_by_ref(self, external_id: str) -> Dict[str, Any]:
        response = self._call({
            "operation": "core/get",
            "class": self.config.ticket_class,
            "key": {"ref": external_id},
            "output_fields": "ref,title,description,status,request_type,impact,urgency,priority,origin,start_date,last_update,functionalcis_list,service_name,servicesubcategory_name,agent_name,team_name,caller_name,org_name,friendlyname",
        })
        incidents = [normalize_incident(item) for item in iter_objects(response)]
        if not incidents:
            raise ITopError(f"incident not found: {external_id}")
        return incidents[0]

    def get_cmdb_items(self, classes: Optional[List[str]] = None) -> List[Dict[str, Any]]:
        items: List[Dict[str, Any]] = []
        for class_name in classes or self.config.cmdb_classes:
            key = self.config.cmdb_oqls.get(class_name, f"SELECT {class_name}")
            response = self._call({
                "operation": "core/get",
                "class": class_name,
                "key": key,
                "output_fields": "*",
            })
            for item in iter_objects(response):
                items.append(normalize_cmdb_item(class_name, item, self.config.cmdb_environment))
        return items

    def apply_incident_stimulus(
        self,
        external_id: str,
        stimulus: str,
        comment: str,
        fields: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        return self._call({
            "operation": "core/apply_stimulus",
            "comment": comment,
            "class": self.config.ticket_class,
            "key": {"ref": external_id},
            "stimulus": stimulus,
            "output_fields": "ref,title,status,friendlyname",
            "fields": fields or {},
        })

    def close_incident(
        self,
        external_id: str,
        target_status: str = "resolved",
        comment: str = "Closed by Auto-Healing adapter",
        resolution: str = "",
        work_notes: str = "",
    ) -> Dict[str, Any]:
        incident = self.get_incident_by_ref(external_id)
        status = incident.get("status", "").strip().lower()
        if status == "closed":
            return incident

        if status == "new":
            self.apply_incident_stimulus(external_id, self.config.assign_stimulus, comment)
            incident = self.get_incident_by_ref(external_id)
            status = incident.get("status", "").strip().lower()

        if status not in ("resolved", "closed"):
            close_fields = dict(self.config.close_fields)
            solution_text = merge_solution_text(resolution, work_notes)
            if solution_text:
                close_fields.setdefault("solution", solution_text)
            if work_notes:
                close_fields.setdefault("public_log", work_notes)
            self.apply_incident_stimulus(
                external_id,
                self.config.close_stimulus,
                comment,
                close_fields,
            )
            incident = self.get_incident_by_ref(external_id)
            status = incident.get("status", "").strip().lower()

        if target_status == "closed" and status != "closed":
            self.apply_incident_stimulus(external_id, self.config.final_close_stimulus, comment)
            incident = self.get_incident_by_ref(external_id)
        return incident

    def create_demo_incident(self, scenario: str, inject_fault: bool = True) -> Dict[str, Any]:
        demo = demo_scenario_payload(self.config, scenario)
        fault_result = None
        if inject_fault and demo.get("fault_scenario"):
            fault_result = self.inject_demo_fault(demo["fault_scenario"])

        response = self._call({
            "operation": "core/create",
            "class": self.config.ticket_class,
            "comment": f"Created by Auto-Healing demo scenario: {scenario}",
            "fields": demo["fields"],
            "output_fields": "ref,title,status,friendlyname,functionalcis_list,start_date,last_update",
        })
        incident = normalize_incident(next(iter_objects(response)))
        incident["scenario"] = scenario
        incident["fault_injection"] = fault_result
        return incident

    def get_demo_fault_status(self) -> Dict[str, Any]:
        raw = self.call_fault_lab("status")
        output = str(raw.get("output") or "")
        return {
            "ok": bool(raw.get("ok")),
            "updated_at": datetime.now().isoformat(timespec="seconds"),
            "target": self.config.demo_ci_name,
            "items": parse_fault_status(output),
            "raw": raw,
        }

    def inject_demo_fault(self, scenario: str) -> Dict[str, Any]:
        return self.call_fault_lab("inject", scenario)

    def call_fault_lab(self, action: str, scenario: str = "") -> Dict[str, Any]:
        if action == "status" and not scenario:
            url = f"{self.config.demo_fault_base_url}/fault-lab/status"
            request = Request(url, method="GET", headers={"Accept": "application/json"})
        elif action in {"inject", "reset", "status"} and scenario:
            url = f"{self.config.demo_fault_base_url}/fault-lab/{quote(action)}/{quote(scenario)}"
            request = Request(
                url,
                data=b"{}",
                method="POST",
                headers={"Content-Type": "application/json", "Accept": "application/json"},
            )
        else:
            raise ITopError(f"invalid fault lab action: {action}/{scenario}")

        try:
            with urlopen(request, timeout=30) as response:
                raw = response.read().decode("utf-8")
                return json.loads(raw) if raw else {"ok": True}
        except HTTPError as exc:
            raw = exc.read().decode("utf-8", "replace")
            if exc.code == 409 and action == "inject" and ("已经处于注入状态" in raw or "already injected" in raw):
                return {"ok": True, "message": "fault already injected", "raw": raw}
            raise ITopError(f"fault injection HTTP {exc.code}: {raw}") from exc
        except URLError as exc:
            raise ITopError(f"fault injection unavailable: {exc}") from exc


def merge_solution_text(resolution: str, work_notes: str) -> str:
    resolution = resolution.strip()
    work_notes = work_notes.strip()
    if resolution and work_notes:
        if resolution in work_notes:
            return work_notes
        return f"{work_notes}\n\n最终结论：\n{resolution}"
    return resolution or work_notes


def build_close_comment(resolution: str, work_notes: str) -> str:
    comment = resolution.strip() or work_notes.strip() or "Closed by Auto-Healing adapter"
    if len(comment) <= 240:
        return comment
    return comment[:237] + "..."


def demo_scenario_payload(config: AdapterConfig, scenario: str) -> Dict[str, Any]:
    scenario = (scenario or "").strip().lower().replace("_", "-")
    suffix = datetime.now().strftime("%m%d%H%M%S") + "-" + secrets.token_hex(2)
    base_fields = {
        "org_id": config.demo_org_id,
        "caller_id": config.demo_caller_id,
        "service_id": config.demo_service_id,
        "servicesubcategory_id": config.demo_subcategory_id,
        "agent_id": config.demo_agent_id,
        "impact": "2",
        "urgency": "2",
        "origin": "monitoring",
        "functionalcis_list": [{"functionalci_id": config.demo_ci_id, "impact_code": "manual"}],
    }
    scenarios = {
        "clean-logs": {
            "title": f"[AHS-DEMO][clean_logs] 日志目录快速膨胀 on {config.demo_ci_name} #{suffix}",
            "description": "\n".join([
                "演示场景: 清理日志",
                f"affected_ci={config.demo_ci_name}",
                "fault_type=clean_logs",
                "预期动作: 运维人员在 AHS 中手动执行 Demo Clean Logs Task，清理实验日志目录并回写工单。",
            ]),
            "fault_scenario": "clean_logs",
        },
        "kill-process": {
            "title": f"[AHS-DEMO][kill_process] 异常进程 CPU 空转 on {config.demo_ci_name} #{suffix}",
            "description": "\n".join([
                "演示场景: 杀死异常进程",
                f"affected_ci={config.demo_ci_name}",
                "fault_type=kill_process",
                "预期动作: AHS 同步工单后自动匹配自愈规则，执行 Demo Kill Process Task。",
            ]),
            "fault_scenario": "kill_process",
        },
        "blacklist": {
            "title": f"[AHS-DEMO][blacklist] 高危指令拦截验证 on {config.demo_ci_name} #{suffix}",
            "description": "\n".join([
                "演示场景: 黑名单指令防御",
                f"affected_ci={config.demo_ci_name}",
                "fault_type=blacklist",
                "预期动作: 手动执行 Demo Blacklist Interception Task，AHS 应在执行前拦截 rm -rf / 等高危指令。",
            ]),
            "fault_scenario": "",
        },
    }
    if scenario not in scenarios:
        raise ITopError(f"unknown demo scenario: {scenario}")
    demo = scenarios[scenario]
    fields = dict(base_fields)
    fields.update({
        "title": demo["title"],
        "description": demo["description"],
    })
    return {"fields": fields, "fault_scenario": demo["fault_scenario"]}


def parse_fault_status(output: str) -> List[Dict[str, Any]]:
    specs = {
        "clean_logs": {
            "title": "日志大文件",
            "ready": "大文件存在",
            "cleared": "已清理",
            "primary": "bytes",
            "path": "dir",
        },
        "kill_process": {
            "title": "异常进程",
            "ready": "进程运行中",
            "cleared": "已关闭",
            "primary": "workers",
            "path": "process",
        },
    }
    parsed: Dict[str, Dict[str, str]] = {}
    for raw_line in output.splitlines():
        line = raw_line.strip()
        if line.startswith("[fault-lab]"):
            line = line[len("[fault-lab]"):].strip()
        if not line or "=" not in line:
            continue
        tokens = line.split()
        key, state = tokens[0].split("=", 1)
        details: Dict[str, str] = {"state": state}
        for token in tokens[1:]:
            if "=" not in token:
                continue
            detail_key, detail_value = token.split("=", 1)
            details[detail_key] = detail_value
        parsed[key] = details

    items: List[Dict[str, Any]] = []
    for key, spec in specs.items():
        details = parsed.get(key, {})
        injected = details.get("state") == "injected"
        primary_key = spec["primary"]
        path_key = spec["path"]
        primary_value = details.get(primary_key, "0")
        if primary_key == "bytes":
            primary_value = human_bytes(primary_value)
        items.append({
            "key": key,
            "title": spec["title"],
            "ready": injected,
            "state": details.get("state", "unknown"),
            "state_label": spec["ready"] if injected else spec["cleared"],
            "metric_label": "大小" if primary_key == "bytes" else "数量",
            "metric_value": primary_value,
            "path_label": "目录" if path_key == "dir" else "进程名",
            "path_value": details.get(path_key, ""),
            "raw": details,
        })
    return items


def human_bytes(value: str) -> str:
    try:
        size = float(value)
    except (TypeError, ValueError):
        return str(value or "0")
    units = ["B", "KB", "MB", "GB", "TB"]
    unit = 0
    while size >= 1024 and unit < len(units) - 1:
        size /= 1024
        unit += 1
    if unit == 0:
        return f"{int(size)} {units[unit]}"
    return f"{size:.1f} {units[unit]}"


def iter_objects(response: Dict[str, Any]) -> Iterable[Dict[str, Any]]:
    for key, value in (response.get("objects") or {}).items():
        fields = value.get("fields") or {}
        fields["_object_key"] = key
        yield fields


def first_non_empty(fields: Dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = fields.get(key)
        if value is None:
            continue
        if isinstance(value, list):
            continue
        text = str(value).strip()
        if text:
            return text
    return ""


def names_from_linked_set(value: Any, *name_keys: str) -> str:
    if not isinstance(value, list):
        return ""
    names = []
    for item in value:
        if isinstance(item, dict):
            text = ""
            for key in name_keys:
                text = str(item.get(key, "")).strip()
                if text:
                    break
            if text:
                names.append(text)
    return ", ".join(names)


def volume_labels(value: Any) -> str:
    if not isinstance(value, list):
        return ""
    labels = []
    for item in value:
        if not isinstance(item, dict):
            continue
        name = str(item.get("volume_name") or item.get("name") or "").strip()
        size = str(item.get("size_used") or item.get("size") or "").strip()
        if name and size:
            labels.append(f"{name} ({size})")
            continue
        if name:
            labels.append(name)
    return ", ".join(labels)


DEMO_CMDB_FALLBACKS: Dict[str, Dict[str, str]] = {
    "e2e-target-01": {
        "ip_address": "118.196.22.79",
        "os": "Linux",
        "os_version": "Ubuntu 22.04 LTS",
        "cpu": "4 vCPU",
        "memory": "8 GB",
        "disk": "root (80 GB), /var/log (20 GB)",
        "location": "Shanghai Demo Lab",
        "manufacturer": "HP",
        "model": "DL380",
        "serial_number": "AHS-DEMO-0001",
    },
    "Server1": {
        "ip_address": "10.10.24.11",
        "cpu": "8 vCPU",
        "memory": "32 GB",
        "disk": "root (200 GB), data (1 TB)",
        "location": "Bordeaux",
        "manufacturer": "HP",
        "model": "DL380",
        "serial_number": "AHS-SRV-0001",
    },
    "Server2": {
        "ip_address": "10.10.24.12",
        "os": "Linux",
        "os_version": "Ubuntu 20.04",
        "cpu": "8 vCPU",
        "memory": "64 GB",
        "disk": "root (200 GB), data (2 TB)",
        "location": "Grenoble",
        "manufacturer": "HP",
        "model": "DL380",
        "serial_number": "AHS-SRV-0002",
    },
    "Server3": {
        "ip_address": "10.10.24.13",
        "cpu": "16 vCPU",
        "memory": "128 GB",
        "disk": "root (300 GB), data (4 TB)",
        "location": "Paris",
        "manufacturer": "HP",
        "model": "DL380",
        "serial_number": "AHS-SRV-0003",
    },
    "Server4": {
        "ip_address": "10.10.24.14",
        "cpu": "16 vCPU",
        "memory": "128 GB",
        "disk": "root (300 GB), data (4 TB)",
        "location": "Paris",
        "manufacturer": "HP",
        "model": "DL380",
        "serial_number": "AHS-SRV-0004",
    },
    "VM1": {
        "ip_address": "10.10.31.21",
        "cpu": "2 vCPU",
        "memory": "8 GB",
        "disk": "vda (80 GB)",
        "manufacturer": "VMware",
        "model": "Virtual Machine",
        "serial_number": "AHS-VM-0001",
    },
    "VM2": {
        "ip_address": "10.10.31.22",
        "cpu": "4 vCPU",
        "memory": "16 GB",
        "disk": "vda (120 GB)",
        "manufacturer": "VMware",
        "model": "Virtual Machine",
        "serial_number": "AHS-VM-0002",
    },
    "VM3": {
        "ip_address": "10.10.31.23",
        "cpu": "4 vCPU",
        "memory": "16 GB",
        "disk": "vda (160 GB)",
        "manufacturer": "VMware",
        "model": "Virtual Machine",
        "serial_number": "AHS-VM-0003",
    },
    "VM4": {
        "ip_address": "10.10.31.24",
        "cpu": "8 vCPU",
        "memory": "32 GB",
        "disk": "vda (200 GB)",
        "manufacturer": "VMware",
        "model": "Virtual Machine",
        "serial_number": "AHS-VM-0004",
    },
    "Router1": {
        "ip_address": "10.10.1.1",
        "location": "Bordeaux",
        "manufacturer": "Cisco",
        "model": "Router",
        "serial_number": "AHS-NET-0001",
    },
    "Switch1": {
        "ip_address": "10.10.1.2",
        "location": "Grenoble",
        "manufacturer": "HP",
        "model": "Procurve 2450",
        "serial_number": "AHS-NET-0002",
    },
}


def demo_cmdb_fallback(fields: Dict[str, Any], key: str) -> str:
    name = first_non_empty(fields, "name", "friendlyname")
    return DEMO_CMDB_FALLBACKS.get(name, {}).get(key, "")


def first_cmdb_value(fields: Dict[str, Any], fallback_key: str, *keys: str) -> str:
    return first_non_empty(fields, *keys) or demo_cmdb_fallback(fields, fallback_key)


def cmdb_disk_label(fields: Dict[str, Any]) -> str:
    return volume_labels(fields.get("logicalvolumes_list")) or demo_cmdb_fallback(fields, "disk")


def strip_html(value: str) -> str:
    value = value.replace("<br>", "\n").replace("<br/>", "\n").replace("<br />", "\n")
    value = value.replace("</p>", "\n").replace("<p>", "")
    value = re.sub(r"<[^>]+>", "", value)
    return unescape(value).strip()


def normalize_incident(fields: Dict[str, Any]) -> Dict[str, Any]:
    external_id = first_non_empty(fields, "ref", "friendlyname", "_object_key")
    return {
        "external_id": external_id,
        "title": first_non_empty(fields, "title", "friendlyname") or external_id,
        "description": strip_html(first_non_empty(fields, "description", "public_log")),
        "severity": first_non_empty(fields, "priority", "impact"),
        "priority": first_non_empty(fields, "priority", "impact"),
        "status": first_non_empty(fields, "status") or "new",
        "category": first_non_empty(fields, "request_type", "finalclass") or "incident",
        "affected_ci": names_from_linked_set(fields.get("functionalcis_list"), "functionalci_name"),
        "affected_service": first_non_empty(fields, "servicesubcategory_name", "service_name"),
        "assignee": first_non_empty(fields, "agent_name", "team_name"),
        "reporter": first_non_empty(fields, "caller_name", "org_name"),
        "source_created_at": first_non_empty(fields, "start_date"),
        "source_updated_at": first_non_empty(fields, "last_update"),
        "raw_data": fields,
    }


def normalize_cmdb_item(class_name: str, fields: Dict[str, Any], environment: str) -> Dict[str, Any]:
    item_id = first_non_empty(fields, "_object_key", "id", "friendlyname")
    return {
        "external_id": item_id,
        "name": first_non_empty(fields, "name", "friendlyname"),
        "type": cmdb_type_for_class(class_name),
        "status": cmdb_status(first_non_empty(fields, "status")),
        "ip_address": first_cmdb_value(fields, "ip_address", "managementip"),
        "hostname": first_non_empty(fields, "name", "friendlyname"),
        "os": first_cmdb_value(fields, "os", "osfamily_name"),
        "os_version": first_cmdb_value(fields, "os_version", "osversion_name", "iosversion_name"),
        "cpu": first_cmdb_value(fields, "cpu", "cpu"),
        "memory": first_cmdb_value(fields, "memory", "ram"),
        "disk": cmdb_disk_label(fields),
        "location": first_cmdb_value(fields, "location", "location_name", "virtualhost_name"),
        "owner": first_non_empty(fields, "organization_name", "org_name"),
        "environment": environment,
        "manufacturer": first_cmdb_value(fields, "manufacturer", "brand_name"),
        "model": first_cmdb_value(fields, "model", "model_name", "networkdevicetype_name"),
        "serial_number": first_cmdb_value(fields, "serial_number", "serialnumber"),
        "department": first_non_empty(fields, "organization_name", "org_name"),
        "source_created_at": first_non_empty(fields, "move2production", "purchase_date"),
        "source_updated_at": "",
        "raw_data": fields,
    }


def cmdb_type_for_class(class_name: str) -> str:
    lowered = class_name.lower()
    if "application" in lowered:
        return "application"
    if "network" in lowered:
        return "network"
    if "database" in lowered or "db" in lowered:
        return "database"
    return "server"


def cmdb_status(raw: str) -> str:
    normalized = raw.lower().strip()
    if normalized in ("production", "active"):
        return "active"
    if normalized == "maintenance":
        return "maintenance"
    if normalized in ("inactive", "obsolete", "implementation", "stock"):
        return "offline"
    return normalized or "active"


class AdapterHandler(BaseHTTPRequestHandler):
    client: ITopClient

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path == "/health":
            self._json(200, {"status": "ok"})
            return
        if parsed.path == "/health/deep":
            try:
                operations = self.client.list_operations()
                self._json(200, {"status": "ok", "message": operations.get("message", "connected")})
            except ITopError as exc:
                self._json(502, {"status": "error", "message": str(exc)})
            return
        if parsed.path == "/api/incidents":
            return self._handle_list(self.client.get_incidents)
        if parsed.path == "/api/cmdb-items":
            query = parse_qs(parsed.query)
            classes = split_csv(",".join(query.get("class", [])))
            return self._handle_list(lambda: self.client.get_cmdb_items(classes or None))
        if parsed.path == "/api/demo/scenarios":
            self._json(200, [
                {"key": "clean-logs", "name": "清理日志", "trigger_mode": "manual"},
                {"key": "kill-process", "name": "杀死异常进程", "trigger_mode": "auto"},
                {"key": "blacklist", "name": "黑名单指令防御", "trigger_mode": "manual"},
            ])
            return
        if parsed.path == "/api/demo/fault-status":
            try:
                self._json(200, self.client.get_demo_fault_status())
            except ITopError as exc:
                self._json(502, {"code": 502, "message": str(exc)})
            return
        self._json(404, {"code": 404, "message": f"unknown path: {parsed.path}"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path == "/api/demo/incidents":
            try:
                content_length = int(self.headers.get("Content-Length", "0") or "0")
                request_body = {}
                if content_length > 0:
                    request_body = json.loads(self.rfile.read(content_length).decode("utf-8"))
                scenario = str(request_body.get("scenario") or "").strip()
                inject_fault = bool(request_body.get("inject_fault", True))
                incident = self.client.create_demo_incident(scenario, inject_fault=inject_fault)
                self._json(201, incident)
            except ITopError as exc:
                self._json(400, {"code": 400, "message": str(exc)})
            except Exception as exc:
                self._json(500, {"code": 500, "message": str(exc)})
            return
        prefix = "/api/incidents/"
        suffix = "/close"
        if not (parsed.path.startswith(prefix) and parsed.path.endswith(suffix)):
            self._json(404, {"code": 404, "message": f"unknown path: {parsed.path}"})
            return
        external_id = parsed.path[len(prefix):-len(suffix)]
        if not external_id:
            self._json(400, {"code": 400, "message": "missing external_id"})
            return
        try:
            content_length = int(self.headers.get("Content-Length", "0") or "0")
            request_body = {}
            if content_length > 0:
                request_body = json.loads(self.rfile.read(content_length).decode("utf-8"))
            target_status = str(request_body.get("close_status") or "resolved").strip().lower() or "resolved"
            resolution = str(request_body.get("resolution") or "").strip()
            work_notes = str(request_body.get("work_notes") or "").strip()
            comment = build_close_comment(resolution, work_notes)
            result = self.client.close_incident(
                external_id,
                target_status=target_status,
                comment=comment,
                resolution=resolution,
                work_notes=work_notes,
            )
            self._json(200, {"message": "incident close stimulus applied", "external_id": external_id, "itop": result})
        except ITopError as exc:
            self._json(502, {"code": 502, "message": str(exc), "external_id": external_id})

    def _handle_list(self, loader) -> None:
        try:
            self._json(200, loader())
        except ITopError as exc:
            self._json(502, {"code": 502, "message": str(exc)})

    def log_message(self, fmt: str, *args: Any) -> None:
        print(f"[iTop-adapter] {self.address_string()} - {fmt % args}", flush=True)

    def _json(self, status: int, payload: Any) -> None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main() -> None:
    config = AdapterConfig()
    AdapterHandler.client = ITopClient(config)
    server = ThreadingHTTPServer((config.adapter_host, config.adapter_port), AdapterHandler)
    print(f"[iTop-adapter] serving on http://{config.adapter_host}:{config.adapter_port} -> {config.rest_endpoint}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
