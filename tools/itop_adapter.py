#!/usr/bin/env python3
"""
iTop adapter for Auto-Healing generic plugins.

This process translates iTop REST/JSON into AHS-standard ITSM / CMDB JSON arrays.
"""

import json
import os
import re
from base64 import b64encode
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
            if resolution:
                close_fields.setdefault("solution", resolution)
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
        "ip_address": first_non_empty(fields, "managementip"),
        "hostname": first_non_empty(fields, "name", "friendlyname"),
        "os": first_non_empty(fields, "osfamily_name"),
        "os_version": first_non_empty(fields, "osversion_name", "iosversion_name"),
        "cpu": first_non_empty(fields, "cpu"),
        "memory": first_non_empty(fields, "ram"),
        "disk": volume_labels(fields.get("logicalvolumes_list")),
        "location": first_non_empty(fields, "location_name", "virtualhost_name"),
        "owner": first_non_empty(fields, "organization_name", "org_name"),
        "environment": environment,
        "manufacturer": first_non_empty(fields, "brand_name"),
        "model": first_non_empty(fields, "model_name", "networkdevicetype_name"),
        "serial_number": first_non_empty(fields, "serialnumber"),
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
        self._json(404, {"code": 404, "message": f"unknown path: {parsed.path}"})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
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
            comment = str(request_body.get("work_notes") or request_body.get("resolution") or "Closed by Auto-Healing adapter").strip()
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
