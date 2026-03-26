#!/usr/bin/env python3
import argparse
import atexit
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import traceback
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
BIN_DIR = ROOT / ".bin"
PG_CONTAINER = os.environ.get("ACCEPTANCE_PG_CONTAINER", "auto-healing-postgres")
REDIS_CONTAINER = os.environ.get("ACCEPTANCE_REDIS_CONTAINER", "auto-healing-redis")
DEFAULT_INIT_ADMIN_PASSWORD = "admin123456"

EXPOSED_PHASES = [
    "auth",
    "platform_tenants",
    "settings_secrets_dictionaries",
    "common",
    "profile_rbac_misc",
    "workbench_site_messages",
    "dashboard_overview_stats",
    "dashboard",
    "tenant_boundaries",
    "search_site_messages",
    "impersonation",
    "healing",
    "healing_queries",
    "git_execution",
    "plugin_cmdb",
    "execution_queries",
    "interface_contract_smoke",
    "notifications_audit",
    "notification_variables",
    "notification_failures",
    "notification_retry",
    "notification_retry_exhaustion",
    "notification_rate_limit",
    "notification_retry_tenant_scope",
    "secrets_default_fallback",
    "secrets_disabled_usage",
    "secrets_runtime_override",
    "secrets_reference_updates",
    "secrets_update_constraints",
    "blacklist_security",
    "blacklist_exemption_execution",
    "audit_action_assertions",
    "filters_pagination",
    "query_token_sse",
]

PHASE_DEPENDENCIES = {
    "auth": [],
    "platform_tenants": ["tenant_setup"],
    "settings_secrets_dictionaries": ["tenant_setup"],
    "common": ["tenant_setup"],
    "profile_rbac_misc": ["tenant_setup"],
    "workbench_site_messages": ["common", "execution_queries"],
    "dashboard_overview_stats": ["settings_secrets_dictionaries", "plugin_cmdb", "execution_queries", "healing_queries", "notifications_audit"],
    "dashboard": ["tenant_setup"],
    "tenant_boundaries": ["tenant_setup"],
    "search_site_messages": ["tenant_setup"],
    "impersonation": ["tenant_setup", "search_site_messages"],
    "healing": ["tenant_setup"],
    "healing_queries": ["healing"],
    "git_execution": ["tenant_setup"],
    "plugin_cmdb": ["tenant_setup", "settings_secrets_dictionaries"],
    "execution_queries": ["git_execution"],
    "interface_contract_smoke": ["git_execution"],
    "notifications_audit": ["tenant_setup", "platform_tenants", "impersonation"],
    "notification_variables": ["git_execution"],
    "notification_failures": ["tenant_setup"],
    "notification_retry": ["tenant_setup"],
    "notification_retry_exhaustion": ["tenant_setup"],
    "notification_rate_limit": ["tenant_setup"],
    "notification_retry_tenant_scope": ["tenant_setup"],
    "secrets_default_fallback": ["settings_secrets_dictionaries"],
    "secrets_disabled_usage": ["git_execution"],
    "secrets_runtime_override": ["git_execution"],
    "secrets_reference_updates": ["settings_secrets_dictionaries", "git_execution"],
    "secrets_update_constraints": ["git_execution"],
    "blacklist_security": ["tenant_setup"],
    "blacklist_exemption_execution": ["tenant_setup"],
    "audit_action_assertions": ["notifications_audit", "platform_tenants"],
    "filters_pagination": ["dashboard_overview_stats"],
    "query_token_sse": ["tenant_setup"],
}


def info(msg):
    print(msg, flush=True)


def require(cmd):
    if shutil.which(cmd) is None:
        raise RuntimeError(f"missing required command: {cmd}")


def pick_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def run(cmd, cwd=None, env=None, check=True):
    proc = subprocess.run(
        cmd,
        cwd=str(cwd) if cwd else None,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    if check and proc.returncode != 0:
        raise RuntimeError(
            "command failed\ncmd: {}\nstdout:\n{}\nstderr:\n{}".format(
                " ".join(cmd), proc.stdout, proc.stderr
            )
        )
    return proc


def curl_json(args):
    proc = run(["curl", "-sS"] + args)
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid json from curl: {proc.stdout}") from exc


def curl_status_json(args):
    with tempfile.NamedTemporaryFile(delete=False) as tmp:
        body_path = tmp.name
    try:
        proc = run(["curl", "-sS", "-o", body_path, "-w", "%{http_code}"] + args)
        with open(body_path) as fh:
            body = fh.read()
        try:
            parsed = json.loads(body)
        except json.JSONDecodeError:
            parsed = body
        return int(proc.stdout.strip()), parsed
    finally:
        try:
            os.unlink(body_path)
        except FileNotFoundError:
            pass


def curl_status_text(args):
    with tempfile.NamedTemporaryFile(delete=False) as tmp:
        body_path = tmp.name
    try:
        proc = run(["curl", "-sS", "-o", body_path, "-w", "%{http_code}"] + args)
        with open(body_path) as fh:
            body = fh.read()
        return int(proc.stdout.strip()), body
    finally:
        try:
            os.unlink(body_path)
        except FileNotFoundError:
            pass


def read_jsonl(path):
    records = []
    p = Path(path)
    if not p.exists():
        return records
    for line in p.read_text().splitlines():
        line = line.strip()
        if not line:
            continue
        records.append(json.loads(line))
    return records


def assert_true(condition, message):
    if not condition:
        raise AssertionError(message)


def assert_eq(actual, expected, message):
    if actual != expected:
        raise AssertionError(f"{message}: expected {expected!r}, got {actual!r}")


class AcceptanceRunner:
    def __init__(self, selected_phases=None):
        self.tmp_dir = Path(tempfile.mkdtemp(prefix="ah-acceptance-"))
        self.api_port = pick_port()
        self.itsm_port = pick_port()
        self.cmdb_port = pick_port()
        self.aux_port = pick_port()
        self.smtp_port = pick_port()
        self.redis_db = "15"
        self.db_name = f"auto_healing_acceptance_{time.time_ns()}"
        self.processes = []
        self.results = {}
        self.server_env = None
        self.keep_artifacts = os.environ.get("KEEP_ACCEPTANCE_ARTIFACTS", "1") == "1"
        self.selected_phases = selected_phases or EXPOSED_PHASES[:]
        self.webhook_hits_path = self.tmp_dir / "webhook_hits.jsonl"
        self.smtp_hits_path = self.tmp_dir / "smtp_hits.jsonl"
        info(f"acceptance artifacts: {self.tmp_dir}")

    def cleanup(self):
        for proc in reversed(self.processes):
            if proc.poll() is None:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
        try:
            run(
                [
                    "docker",
                    "exec",
                    PG_CONTAINER,
                    "dropdb",
                    "-U",
                    "postgres",
                    "--if-exists",
                    self.db_name,
                ],
                check=False,
            )
        except Exception:
            pass
        if not self.keep_artifacts:
            try:
                shutil.rmtree(self.tmp_dir)
            except Exception:
                pass

    def start_process(self, cmd, log_name, env=None, cwd=None):
        log_path = self.tmp_dir / log_name
        log_file = open(log_path, "w")
        proc = subprocess.Popen(
            cmd,
            cwd=str(cwd) if cwd else None,
            env=env,
            stdout=log_file,
            stderr=subprocess.STDOUT,
            universal_newlines=True,
        )
        self.processes.append(proc)
        return proc, log_path

    def wait_http_json(self, url, timeout=60):
        deadline = time.time() + timeout
        last_error = None
        while time.time() < deadline:
            try:
                return curl_json([url])
            except Exception as exc:
                last_error = exc
                time.sleep(1)
        raise RuntimeError(f"timeout waiting for {url}: {last_error}")

    def wait_tcp(self, host, port, timeout=30):
        deadline = time.time() + timeout
        last_error = None
        while time.time() < deadline:
            try:
                with socket.create_connection((host, port), timeout=1):
                    return
            except OSError as exc:
                last_error = exc
                time.sleep(1)
        raise RuntimeError(f"timeout waiting for tcp {host}:{port}: {last_error}")

    def build_binaries(self):
        info("==> build static binaries")
        BIN_DIR.mkdir(exist_ok=True)
        run(
            [
                "docker",
                "run",
                "--rm",
                "-v",
                f"{ROOT}:/workspace",
                "-w",
                "/workspace",
                "golang:1.24.5",
                "sh",
                "-c",
                "CGO_ENABLED=0 go build -o .bin/server-static ./cmd/server && CGO_ENABLED=0 go build -o .bin/init-admin-static ./cmd/init-admin",
            ]
        )

    def prepare_repo(self):
        info("==> prepare local acceptance repo")
        repo_dir = self.tmp_dir / "repo"
        repo_dir.mkdir(parents=True, exist_ok=True)
        run(["git", "init", "-b", "main"], cwd=repo_dir)
        files = {
            "local.yml": """---
- name: acceptance local run
  hosts: all
  connection: local
  gather_facts: false
  tasks:
    - name: echo acceptance
      ansible.builtin.command: /bin/echo acceptance-ok
""",
            "long.yml": """---
- name: acceptance long run
  hosts: all
  connection: local
  gather_facts: false
  tasks:
    - name: sleep a bit
      ansible.builtin.command: /bin/sleep 8
""",
            "ping.yml": """---
- name: acceptance ping
  hosts: all
  connection: local
  gather_facts: false
  tasks:
    - name: ping target
      ansible.builtin.command: /bin/echo ping-ok
""",
            "danger.yml": """---
- name: acceptance blacklist system rule
  hosts: all
  connection: local
  gather_facts: false
  tasks:
    - name: echo blocked database string
      ansible.builtin.command: /bin/echo DROP DATABASE
""",
        }
        for name, content in files.items():
            (repo_dir / name).write_text(content)
        run(["git", "add", "."], cwd=repo_dir)
        diff = run(["git", "diff", "--cached", "--quiet"], cwd=repo_dir, check=False)
        if diff.returncode != 0:
            run(
                [
                    "git",
                    "-c",
                    "user.name=Acceptance",
                    "-c",
                    "user.email=acceptance@example.com",
                    "commit",
                    "-m",
                    "acceptance fixtures",
                ],
                cwd=repo_dir,
            )
        self.repo_dir = repo_dir

    def prepare_db(self):
        info("==> create isolated acceptance db")
        run(
            [
                "docker",
                "exec",
                REDIS_CONTAINER,
                "redis-cli",
                "-n",
                self.redis_db,
                "FLUSHDB",
            ]
        )
        run(
            ["docker", "exec", PG_CONTAINER, "createdb", "-U", "postgres", self.db_name]
        )

    def start_mocks(self):
        info("==> start mock ITSM / CMDB / aux mocks")
        proc, _ = self.start_process(
            [
                "python3.11",
                "-c",
                (
                    "import sys; sys.path.insert(0, {!r}); import mock_itsm_healing; "
                    "mock_itsm_healing.app.run(host='127.0.0.1', port={}, debug=False)"
                ).format(str(ROOT / "tools"), self.itsm_port),
            ],
            "mock_itsm.log",
            cwd=ROOT,
        )
        proc2, _ = self.start_process(
            [
                "python3.11",
                "-c",
                (
                    "import sys; sys.path.insert(0, {!r}); import mock_cmdb_healing; "
                    "mock_cmdb_healing.app.run(host='127.0.0.1', port={}, debug=False)"
                ).format(str(ROOT / "tools"), self.cmdb_port),
            ],
            "mock_cmdb.log",
            cwd=ROOT,
        )
        aux_script = f"""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

WEBHOOK_LOG = Path({str(self.webhook_hits_path)!r})
SECRETS = {{
    "host-a": {{"data": {{"username": "ops-a", "password": "pw-a"}}}},
    "host-b": {{"data": {{"username": "ops-b", "password": "pw-b"}}}},
    "10.0.0.1": {{"data": {{"username": "ops-ip", "password": "pw-ip"}}}},
}}

class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        return

    def _send_json(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_HEAD(self):
        self.send_response(200)
        self.end_headers()

    def do_GET(self):
        if self.path == "/health":
            return self._send_json(200, {{"status": "ok"}})
        if self.path == "/secret":
            return self._send_json(200, {{"status": "ok"}})
        if self.path.startswith("/secret/"):
            key = self.path.split("/", 2)[2]
            payload = SECRETS.get(key)
            if payload is None:
                return self._send_json(404, {{"error": "not found"}})
            return self._send_json(200, payload)
        return self._send_json(404, {{"error": "unknown path", "path": self.path}})

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length).decode() if length else ""
        try:
            body = json.loads(raw) if raw else {{}}
        except json.JSONDecodeError:
            body = raw
        if self.path.startswith("/notify") or self.path.startswith("/dingtalk"):
            with WEBHOOK_LOG.open("a") as fh:
                fh.write(json.dumps({{"path": self.path, "body": body}}) + "\\n")
            if self.path.startswith("/dingtalk"):
                if self.path.startswith("/dingtalk-fail"):
                    return self._send_json(200, {{"errcode": 310000, "errmsg": "fail"}})
                return self._send_json(200, {{"errcode": 0, "errmsg": "ok"}})
            if self.path.startswith("/notify-fail"):
                return self._send_json(500, {{"ok": False}})
            return self._send_json(200, {{"ok": True}})
        return self._send_json(404, {{"error": "unknown path", "path": self.path}})

HTTPServer(("127.0.0.1", {self.aux_port}), Handler).serve_forever()
"""
        smtp_script = f"""
import json
import socketserver
from pathlib import Path

SMTP_LOG = Path({str(self.smtp_hits_path)!r})

class SMTPHandler(socketserver.StreamRequestHandler):
    def handle(self):
        self.mail_from = ""
        self.rcpts = []
        self.wfile.write(b"220 acceptance-smtp\\r\\n")
        self.wfile.flush()
        while True:
            line = self.rfile.readline()
            if not line:
                return
            cmd = line.decode(errors="ignore").rstrip("\\r\\n")
            upper = cmd.upper()
            if upper.startswith("EHLO") or upper.startswith("HELO"):
                self.wfile.write(b"250-acceptance-smtp\\r\\n250-AUTH PLAIN LOGIN\\r\\n250 OK\\r\\n")
            elif upper.startswith("AUTH PLAIN"):
                self.wfile.write(b"235 2.7.0 Authentication successful\\r\\n")
            elif upper == "AUTH LOGIN":
                self.wfile.write(b"334 VXNlcm5hbWU6\\r\\n")
            elif upper.startswith("MAIL FROM:"):
                self.mail_from = cmd[10:].strip()
                self.wfile.write(b"250 2.1.0 OK\\r\\n")
            elif upper.startswith("RCPT TO:"):
                self.rcpts.append(cmd[8:].strip())
                self.wfile.write(b"250 2.1.5 OK\\r\\n")
            elif upper == "DATA":
                self.wfile.write(b"354 End data with <CR><LF>.<CR><LF>\\r\\n")
                data_lines = []
                while True:
                    data_line = self.rfile.readline()
                    if not data_line:
                        break
                    if data_line in (b".\\r\\n", b".\\n"):
                        break
                    data_lines.append(data_line.decode(errors="ignore"))
                with SMTP_LOG.open("a") as fh:
                    fh.write(json.dumps({{"from": self.mail_from, "rcpt": self.rcpts, "data": "".join(data_lines)}}) + "\\n")
                self.wfile.write(b"250 2.0.0 Accepted\\r\\n")
            elif upper == "QUIT":
                self.wfile.write(b"221 2.0.0 Bye\\r\\n")
                self.wfile.flush()
                return
            else:
                self.wfile.write(b"250 OK\\r\\n")
            self.wfile.flush()

class ThreadingSMTPServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True

ThreadingSMTPServer(("127.0.0.1", {self.smtp_port}), SMTPHandler).serve_forever()
"""
        self.start_process(
            ["python3.11", "-c", aux_script],
            "mock_aux.log",
            cwd=ROOT,
        )
        self.start_process(
            ["python3.11", "-c", smtp_script],
            "mock_smtp.log",
            cwd=ROOT,
        )
        self.wait_http_json(f"http://127.0.0.1:{self.itsm_port}/api/now/table/incident")
        self.wait_http_json(f"http://127.0.0.1:{self.cmdb_port}/api/cmdb/hosts")
        self.wait_http_json(f"http://127.0.0.1:{self.aux_port}/health")
        self.wait_tcp("127.0.0.1", self.smtp_port)

    def start_server(self):
        info("==> start acceptance server")
        env = os.environ.copy()
        env.update(
            {
                "APP_URL": f"http://127.0.0.1:{self.api_port}",
                "APP_ENV": "acceptance",
                "SERVER_HOST": "127.0.0.1",
                "SERVER_PORT": str(self.api_port),
                "SERVER_MODE": "release",
                "DATABASE_DBNAME": self.db_name,
                "REDIS_DB": self.redis_db,
                "JWT_SECRET": "acceptance-secret-key",
                "JWT_ISSUER": "auto-healing-acceptance",
                "LOG_FILE_PATH": str(self.tmp_dir / "logs"),
                "ANSIBLE_WORKSPACE_DIR": str(self.tmp_dir / "workspace"),
                "GIT_REPOS_DIR": str(self.tmp_dir / "repos"),
                "NOTIFICATION_RETRY_INTERVAL": "2s",
                "BLACKLIST_EXEMPTION_INTERVAL": "2s",
                "INIT_ADMIN_PASSWORD": os.environ.get("INIT_ADMIN_PASSWORD", DEFAULT_INIT_ADMIN_PASSWORD),
            }
        )
        self.server_env = env
        self.start_process([str(BIN_DIR / "server-static")], "server.log", env=env, cwd=ROOT)
        health = self.wait_http_json(f"http://127.0.0.1:{self.api_port}/health")
        assert_eq(health["status"], "ok", "server health")

    def init_admin(self):
        info("==> init admin")
        run([str(BIN_DIR / "init-admin-static")], cwd=ROOT, env=self.server_env)

    def api_base(self):
        return f"http://127.0.0.1:{self.api_port}/api/v1"

    def auth_args(self, token, tenant_id=None, json_content=False, extra_headers=None):
        args = ["-H", f"Authorization: Bearer {token}"]
        if tenant_id:
            args.extend(["-H", f"X-Tenant-ID: {tenant_id}"])
        if json_content:
            args.extend(["-H", "Content-Type: application/json"])
        if extra_headers:
            for header in extra_headers:
                args.extend(["-H", header])
        return args

    def list_items(self, payload):
        data = payload["data"]
        if isinstance(data, list):
            return data
        if isinstance(data, dict) and "items" in data:
            return data["items"]
        return data

    def wait_until(self, fn, timeout=30, interval=1, description="condition"):
        deadline = time.time() + timeout
        last = None
        while time.time() < deadline:
            last = fn()
            if last:
                return last
            time.sleep(interval)
        raise AssertionError(f"timeout waiting for {description}")

    def wait_pending_trigger_items(self, token):
        def pending_items():
            payload = curl_json([f"{self.api_base()}/tenant/healing/pending/trigger", "-H", f"Authorization: Bearer {token}"])
            data = payload.get("data")
            if isinstance(data, list):
                return data
            if isinstance(data, dict) and data.get("items"):
                return data["items"]
            return None

        return self.wait_until(pending_items, timeout=40, interval=2, description="pending trigger incidents")

    def pick_pending_incident(self, items, exclude_ids=None, allow_fail=False):
        blocked = set(exclude_ids or [])
        for item in items:
            if item.get("id") in blocked:
                continue
            title = item.get("title", "")
            if not allow_fail and "FAIL" in title:
                continue
            return item
        for item in items:
            if item.get("id") not in blocked:
                return item
        raise AssertionError("no pending incident available")

    def psql(self, sql):
        run(
            [
                "docker",
                "exec",
                PG_CONTAINER,
                "psql",
                "-U",
                "postgres",
                "-d",
                self.db_name,
                "-c",
                sql,
            ]
        )

    def ensure_executor_image(self):
        image = os.environ.get("ACCEPTANCE_EXECUTOR_IMAGE", "auto-healing/ansible-executor:latest")
        inspect = run(["docker", "image", "inspect", image], check=False)
        if inspect.returncode == 0:
            return image
        info(f"==> build executor image: {image}")
        run(
            [
                "docker",
                "build",
                "-t",
                image,
                "-f",
                "docker/ansible-executor/Dockerfile",
                "docker/ansible-executor",
            ],
            cwd=ROOT,
        )
        return image

    def webhook_hits(self):
        return read_jsonl(self.webhook_hits_path)

    def provider_hits(self, prefix):
        return [item for item in self.webhook_hits() if item.get("path", "").startswith(prefix)]

    def smtp_hits(self):
        return read_jsonl(self.smtp_hits_path)

    def login(self, username, password):
        resp = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/login",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"username": username, "password": password}),
            ]
        )
        assert_true(resp.get("access_token"), f"login failed for {username}")
        return resp

    def run_auth_scenarios(self):
        info("==> auth scenarios")
        login = self.login("admin", "admin123456")
        access = login["access_token"]
        refresh = login["refresh_token"]
        me = curl_json([f"{self.api_base()}/auth/me", "-H", f"Authorization: Bearer {access}"])
        refreshed = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/refresh",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"refresh_token": refresh}),
            ]
        )
        old_status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/refresh",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"refresh_token": refresh}),
            ]
        )
        logout_status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/logout",
                "-H",
                f"Authorization: Bearer {refreshed['access_token']}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"refresh_token": refreshed["refresh_token"]}),
            ]
        )
        refresh_after_logout, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/refresh",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"refresh_token": refreshed["refresh_token"]}),
            ]
        )
        me_after_logout, _ = curl_status_json(
            [f"{self.api_base()}/auth/me", "-H", f"Authorization: Bearer {refreshed['access_token']}"]
        )
        assert_eq(me["data"]["username"], "admin", "auth/me username")
        assert_eq(old_status, 401, "old refresh token should be rejected")
        assert_eq(logout_status, 200, "logout status")
        assert_eq(refresh_after_logout, 401, "refresh token should be revoked on logout")
        assert_eq(me_after_logout, 401, "access token should be revoked on logout")
        self.results["auth"] = {"status": "passed"}

    def setup_tenants_and_users(self):
        info("==> tenant and user setup")
        admin_login = self.login("admin", "admin123456")
        self.platform_token = admin_login["access_token"]
        headers = ["-H", f"Authorization: Bearer {self.platform_token}", "-H", "Content-Type: application/json"]

        tenant_a = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants",
                *headers,
                "-d",
                json.dumps({"name": "Acceptance Tenant A", "code": "acc_tenant_a", "description": "acceptance", "icon": "team"}),
            ]
        )["data"]
        tenant_b = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants",
                *headers,
                "-d",
                json.dumps({"name": "Acceptance Tenant B", "code": "acc_tenant_b", "description": "acceptance", "icon": "team"}),
            ]
        )["data"]
        roles = curl_json([f"{self.api_base()}/platform/tenant-roles", "-H", f"Authorization: Bearer {self.platform_token}"])["data"]
        admin_role = next(r for r in roles if r["name"] == "admin")
        viewer_role = next(r for r in roles if r["name"] == "viewer")

        invite_a = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants/{tenant_a['id']}/invitations",
                *headers,
                "-d",
                json.dumps({"email": "tenanta-admin@example.com", "role_id": admin_role["id"], "send_email": False}),
            ]
        )["data"]
        invite_b = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants/{tenant_b['id']}/invitations",
                *headers,
                "-d",
                json.dumps({"email": "tenantb-viewer@example.com", "role_id": viewer_role["id"], "send_email": False}),
            ]
        )["data"]

        token_a = invite_a["invitation_url"].split("token=")[1]
        token_b = invite_b["invitation_url"].split("token=")[1]
        reg_a = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/register",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"token": token_a, "username": "tenantadmin", "password": "Tenant123456!", "display_name": "Tenant Admin"}),
            ]
        )["data"]["user"]
        reg_b = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/auth/register",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"token": token_b, "username": "tenantviewer", "password": "Tenant123456!", "display_name": "Tenant Viewer"}),
            ]
        )["data"]["user"]

        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants/{tenant_b['id']}/members",
                *headers,
                "-d",
                json.dumps({"user_id": reg_a["id"], "role_id": admin_role["id"]}),
            ]
        )

        tenant_admin_login = self.login("tenantadmin", "Tenant123456!")
        tenant_viewer_login = self.login("tenantviewer", "Tenant123456!")
        relogin = self.login("tenantadmin", "Tenant123456!")
        assert_eq(
            tenant_admin_login["current_tenant_id"],
            relogin["current_tenant_id"],
            "default tenant id should be stable across login",
        )

        self.tenant_a = tenant_a
        self.tenant_b = tenant_b
        self.tenant_admin = reg_a
        self.tenant_viewer = reg_b
        self.system_tenant_roles = {
            "admin": admin_role,
            "viewer": viewer_role,
        }
        self.tenant_admin_token = tenant_admin_login["access_token"]
        self.tenant_viewer_token = tenant_viewer_login["access_token"]
        self.default_tenant_id = tenant_admin_login["current_tenant_id"]
        self.other_tenant_id = tenant_b["id"] if self.default_tenant_id == tenant_a["id"] else tenant_a["id"]
        self.results["tenant_setup"] = {"status": "passed"}

    def run_platform_tenants(self):
        info("==> platform tenant management")
        token = self.platform_token
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)

        temp = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "Acceptance Platform Temp",
                        "code": "acc_platform_temp",
                        "description": "platform acceptance temp",
                        "icon": "lab",
                    }
                ),
            ]
        )["data"]

        listed = curl_json([f"{self.api_base()}/platform/tenants?keyword=acc_platform_temp", *view_headers])
        listed_items = self.list_items(listed)
        assert_true(any(item["id"] == temp["id"] for item in listed_items), "temp tenant should be listed")

        detail = curl_json([f"{self.api_base()}/platform/tenants/{temp['id']}", *view_headers])
        assert_eq(detail["data"]["code"], "acc_platform_temp", "tenant detail code")

        updated = curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/tenants/{temp['id']}",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "Acceptance Platform Temp Updated",
                        "description": "updated temp tenant",
                        "icon": "updated",
                    }
                ),
            ]
        )
        assert_eq(updated["data"]["name"], "Acceptance Platform Temp Updated", "tenant update name")

        stats = curl_json([f"{self.api_base()}/platform/tenants/stats", *view_headers])
        stats_tenants = stats["data"]["tenants"]
        assert_true(any(item["id"] == temp["id"] for item in stats_tenants), "tenant stats should include temp tenant")
        assert_true(stats["data"]["summary"]["total_tenants"] >= 3, "tenant summary should include created tenants")

        trends = curl_json([f"{self.api_base()}/platform/tenants/trends?days=7", *view_headers])
        assert_eq(len(trends["data"]["dates"]), 7, "tenant trends dates length")
        assert_eq(len(trends["data"]["operations"]), 7, "tenant trends operations length")
        assert_eq(len(trends["data"]["task_executions"]), 7, "tenant trends task executions length")

        viewer_role = self.system_tenant_roles["viewer"]
        admin_role = self.system_tenant_roles["admin"]
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants/{temp['id']}/members",
                *headers,
                "-d",
                json.dumps({"user_id": self.tenant_viewer["id"], "role_id": viewer_role["id"]}),
            ]
        )
        members = curl_json([f"{self.api_base()}/platform/tenants/{temp['id']}/members", *view_headers])["data"]
        member = next((item for item in members if item["user_id"] == self.tenant_viewer["id"]), None)
        assert_true(member is not None, "added member should be listed")
        assert_eq(member["role_id"], viewer_role["id"], "member viewer role")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/tenants/{temp['id']}/members/{self.tenant_viewer['id']}/role",
                *headers,
                "-d",
                json.dumps({"role_id": admin_role["id"]}),
            ]
        )
        members_after_admin = curl_json([f"{self.api_base()}/platform/tenants/{temp['id']}/members", *view_headers])["data"]
        member = next((item for item in members_after_admin if item["user_id"] == self.tenant_viewer["id"]), None)
        assert_eq(member["role_id"], admin_role["id"], "member admin role")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/tenants/{temp['id']}/members/{self.tenant_viewer['id']}/role",
                *headers,
                "-d",
                json.dumps({"role_id": viewer_role["id"]}),
            ]
        )
        curl_json(
            [
                "-X",
                "DELETE",
                f"{self.api_base()}/platform/tenants/{temp['id']}/members/{self.tenant_viewer['id']}",
                *view_headers,
            ]
        )
        members_after_remove = curl_json([f"{self.api_base()}/platform/tenants/{temp['id']}/members", *view_headers])["data"]
        assert_true(
            all(item["user_id"] != self.tenant_viewer["id"] for item in members_after_remove),
            "removed member should not remain",
        )

        invite = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants/{temp['id']}/invitations",
                *headers,
                "-d",
                json.dumps(
                    {
                        "email": "temp-invite@example.com",
                        "role_id": viewer_role["id"],
                        "send_email": False,
                    }
                ),
            ]
        )["data"]["invitation"]
        invitations = curl_json([f"{self.api_base()}/platform/tenants/{temp['id']}/invitations?status=pending", *view_headers])
        invitation_items = self.list_items(invitations)
        assert_true(any(item["id"] == invite["id"] for item in invitation_items), "pending invitation should be listed")

        curl_json(
            [
                "-X",
                "DELETE",
                f"{self.api_base()}/platform/tenants/{temp['id']}/invitations/{invite['id']}",
                *view_headers,
            ]
        )
        invitations_after_cancel = curl_json([f"{self.api_base()}/platform/tenants/{temp['id']}/invitations", *view_headers])
        cancelled_inv = next((item for item in self.list_items(invitations_after_cancel) if item["id"] == invite["id"]), None)
        assert_true(cancelled_inv is not None, "cancelled invitation should still be queryable")
        assert_eq(cancelled_inv["status"], "cancelled", "invitation status after cancel")

        delete_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/platform/tenants/{temp['id']}", *view_headers])
        assert_eq(delete_status, 200, "delete temp tenant status")
        deleted_status, _ = curl_status_json([f"{self.api_base()}/platform/tenants/{temp['id']}", *view_headers])
        assert_eq(deleted_status, 404, "deleted tenant should not be found")

        self.results["platform_tenants"] = {"status": "passed"}

    def run_settings_secrets_dictionaries(self):
        info("==> settings / secrets / dictionaries")
        platform_view_headers = self.auth_args(self.platform_token)
        platform_headers = self.auth_args(self.platform_token, json_content=True)
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        settings = curl_json([f"{self.api_base()}/platform/settings", *platform_view_headers])
        modules = {item["module"] for item in settings["data"]}
        assert_true("site" in modules, "platform settings should include site module")
        assert_true("email" in modules, "platform settings should include email module")

        site_settings = curl_json([f"{self.api_base()}/platform/settings?module=site", *platform_view_headers])
        email_settings = curl_json([f"{self.api_base()}/platform/settings?module=email", *platform_view_headers])
        assert_eq(len(site_settings["data"]), 1, "site module query should return one group")
        assert_eq(site_settings["data"][0]["module"], "site", "site settings module")
        assert_eq(len(email_settings["data"]), 1, "email module query should return one group")
        assert_eq(email_settings["data"][0]["module"], "email", "email settings module")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/settings/site.base_url",
                *platform_headers,
                "-d",
                json.dumps({"value": "https://acc.example.com"}),
            ]
        )
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/settings/email.invitation_expire_days",
                *platform_headers,
                "-d",
                json.dumps({"value": "14"}),
            ]
        )
        invalid_status, _ = curl_status_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/settings/email.invitation_expire_days",
                *platform_headers,
                "-d",
                json.dumps({"value": "abc"}),
            ]
        )
        assert_eq(invalid_status, 400, "invalid int platform setting should fail")
        site_settings_after = curl_json([f"{self.api_base()}/platform/settings?module=site", *platform_view_headers])
        email_settings_after = curl_json([f"{self.api_base()}/platform/settings?module=email", *platform_view_headers])
        site_group = site_settings_after["data"][0]["settings"]
        email_group = email_settings_after["data"][0]["settings"]
        assert_true(any(item["key"] == "site.base_url" and item["value"] == "https://acc.example.com" for item in site_group), "site.base_url should update")
        assert_true(any(item["key"] == "email.invitation_expire_days" and item["value"] == "14" for item in email_group), "email invitation days should update")

        dict_types = curl_json([f"{self.api_base()}/common/dictionaries/types", *tenant_view_headers])
        assert_true(len(dict_types["data"]) >= 1, "dictionary types should be non-empty")
        dict_payload = {
            "dict_type": "acc_test_type",
            "dict_key": "case_a",
            "label": "Case A",
            "label_en": "Case A",
            "sort_order": 1,
            "is_system": False,
            "is_active": True,
        }
        created_dict = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/dictionaries",
                *platform_headers,
                "-d",
                json.dumps(dict_payload),
            ]
        )["data"]
        common_dicts = curl_json([f"{self.api_base()}/common/dictionaries?types=acc_test_type&active_only=true", *tenant_view_headers])
        assert_true("acc_test_type" in common_dicts["data"], "custom dictionary type should be visible")
        assert_eq(common_dicts["data"]["acc_test_type"][0]["dict_key"], "case_a", "dictionary key should match")
        updated_dict = curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/dictionaries/{created_dict['id']}",
                *platform_headers,
                "-d",
                json.dumps(
                    {
                        "label": "Case A Updated",
                        "label_en": "Case A Updated",
                        "sort_order": 2,
                        "is_active": False,
                    }
                ),
            ]
        )["data"]
        assert_eq(updated_dict["label"], "Case A Updated", "dictionary label should update")
        common_inactive = curl_json([f"{self.api_base()}/common/dictionaries?types=acc_test_type&active_only=false", *tenant_view_headers])
        assert_true(
            any(item["id"] == created_dict["id"] and item["is_active"] is False for item in common_inactive["data"]["acc_test_type"]),
            "inactive dictionary should be visible when active_only=false",
        )
        delete_dict_status, _ = curl_status_json(
            ["-X", "DELETE", f"{self.api_base()}/platform/dictionaries/{created_dict['id']}", *platform_view_headers]
        )
        assert_eq(delete_dict_status, 200, "delete custom dictionary status")
        common_after_delete = curl_json([f"{self.api_base()}/common/dictionaries?types=acc_test_type&active_only=false", *tenant_view_headers])
        assert_true(
            "acc_test_type" not in common_after_delete["data"] or len(common_after_delete["data"]["acc_test_type"]) == 0,
            "deleted dictionary should not remain",
        )

        secrets_config = {
            "url": f"http://127.0.0.1:{self.aux_port}/secret",
            "method": "GET",
            "query_key": "hostname",
            "timeout": 5,
            "response_data_path": "data",
            "field_mapping": {"username": "username", "password": "password"},
            "auth": {"type": "none"},
        }
        source_a = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-a",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": secrets_config,
                        "is_default": True,
                        "priority": 1,
                    }
                ),
            ]
        )["data"]
        source_b = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-b",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": secrets_config,
                        "is_default": False,
                        "priority": 5,
                    }
                ),
            ]
        )["data"]
        secrets_list = curl_json([f"{self.api_base()}/tenant/secrets-sources?type=webhook&status=active", *tenant_view_headers])["data"]
        assert_true(any(item["id"] == source_a["id"] for item in secrets_list), "secret source A should be listed")
        assert_true(any(item["id"] == source_b["id"] for item in secrets_list), "secret source B should be listed")
        secrets_stats = curl_json([f"{self.api_base()}/tenant/secrets-sources/stats", *tenant_view_headers])
        assert_true(secrets_stats["data"]["total"] >= 2, "secret stats should include created sources")
        source_detail = curl_json([f"{self.api_base()}/tenant/secrets-sources/{source_a['id']}", *tenant_view_headers])["data"]
        assert_eq(source_detail["id"], source_a["id"], "secret source detail id")

        test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source_a['id']}/test", *tenant_view_headers])
        assert_eq(test_status, 200, "secret source test should pass")
        test_query_single = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources/{source_a['id']}/test-query",
                *tenant_headers,
                "-d",
                json.dumps({"hostname": "host-a"}),
            ]
        )["data"]
        assert_eq(test_query_single["success_count"], 1, "single secret test-query success count")
        test_query_multi = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources/{source_a['id']}/test-query",
                *tenant_headers,
                "-d",
                json.dumps({"hosts": [{"hostname": "host-a"}, {"hostname": "host-b"}]}),
            ]
        )["data"]
        assert_eq(test_query_multi["success_count"], 2, "multi secret test-query success count")

        query_explicit = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets/query",
                *tenant_headers,
                "-d",
                json.dumps({"source_id": source_a["id"], "hostname": "host-a"}),
            ]
        )["data"]
        assert_eq(query_explicit["username"], "ops-a", "explicit secret query username")
        query_default = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets/query",
                *tenant_headers,
                "-d",
                json.dumps({"hostname": "host-b"}),
            ]
        )["data"]
        assert_eq(query_default["username"], "ops-b", "default secret query username")

        updated_source_b = curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/secrets-sources/{source_b['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"priority": 0, "is_default": True}),
            ]
        )["data"]
        assert_eq(updated_source_b["is_default"], True, "updated source should become default")
        defaults = curl_json([f"{self.api_base()}/tenant/secrets-sources?is_default=true", *tenant_view_headers])["data"]
        assert_eq(len(defaults), 1, "there should be exactly one default secret source")
        assert_eq(defaults[0]["id"], source_b["id"], "source B should be the only default")

        invalid_status_update, _ = curl_status_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/secrets-sources/{source_b['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"status": "banana"}),
            ]
        )
        assert_eq(invalid_status_update, 400, "invalid secret source status should be rejected")

        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source_b['id']}/disable", *tenant_view_headers])
        disabled_source = curl_json([f"{self.api_base()}/tenant/secrets-sources/{source_b['id']}", *tenant_view_headers])["data"]
        assert_eq(disabled_source["status"], "inactive", "secret source should disable")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source_b['id']}/enable", *tenant_view_headers])
        enabled_source = curl_json([f"{self.api_base()}/tenant/secrets-sources/{source_b['id']}", *tenant_view_headers])["data"]
        assert_eq(enabled_source["status"], "active", "secret source should enable")

        delete_source_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source_a['id']}", *tenant_view_headers])
        assert_eq(delete_source_status, 200, "delete secret source A status")
        secrets_after_delete = curl_json([f"{self.api_base()}/tenant/secrets-sources", *tenant_view_headers])["data"]
        assert_true(all(item["id"] != source_a["id"] for item in secrets_after_delete), "deleted secret source should not remain")

        self.default_secret_source = enabled_source
        self.results["settings_secrets_dictionaries"] = {"status": "passed"}

    def run_common_isolation(self):
        info("==> common tenant isolation")
        base = self.tenant_admin_token
        headers = ["-H", f"Authorization: Bearer {base}", "-H", "Content-Type: application/json"]
        other_headers = ["-H", f"Authorization: Bearer {base}", "-H", "Content-Type: application/json", "-H", f"X-Tenant-ID: {self.other_tenant_id}"]

        curl_json(["-X", "PUT", f"{self.api_base()}/common/user/preferences", *headers, "-d", json.dumps({"preferences": {"theme": "A", "layout": "alpha"}})])
        curl_json(["-X", "PUT", f"{self.api_base()}/common/user/preferences", *other_headers, "-d", json.dumps({"preferences": {"theme": "B", "layout": "beta"}})])
        pref_a = curl_json([f"{self.api_base()}/common/user/preferences", "-H", f"Authorization: Bearer {base}"])
        pref_b = curl_json([f"{self.api_base()}/common/user/preferences", "-H", f"Authorization: Bearer {base}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])
        assert_eq(pref_a["data"]["preferences"]["theme"], "A", "tenant A preference")
        assert_eq(pref_b["data"]["preferences"]["theme"], "B", "tenant B preference")

        curl_json(["-X", "POST", f"{self.api_base()}/common/user/favorites", *headers, "-d", json.dumps({"menu_key": "dash-A", "name": "Dash A", "path": "/a"})])
        curl_json(["-X", "POST", f"{self.api_base()}/common/user/favorites", *other_headers, "-d", json.dumps({"menu_key": "dash-B", "name": "Dash B", "path": "/b"})])
        fav_a = curl_json([f"{self.api_base()}/common/user/favorites", "-H", f"Authorization: Bearer {base}"])
        fav_b = curl_json([f"{self.api_base()}/common/user/favorites", "-H", f"Authorization: Bearer {base}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])
        assert_eq([x["menu_key"] for x in fav_a["data"]], ["dash-A"], "tenant A favorites")
        assert_eq([x["menu_key"] for x in fav_b["data"]], ["dash-B"], "tenant B favorites")

        curl_json(["-X", "POST", f"{self.api_base()}/common/user/recents", *headers, "-d", json.dumps({"menu_key": "recent-A", "name": "Recent A", "path": "/ra"})])
        curl_json(["-X", "POST", f"{self.api_base()}/common/user/recents", *other_headers, "-d", json.dumps({"menu_key": "recent-B", "name": "Recent B", "path": "/rb"})])
        rec_a = curl_json([f"{self.api_base()}/common/user/recents", "-H", f"Authorization: Bearer {base}"])
        rec_b = curl_json([f"{self.api_base()}/common/user/recents", "-H", f"Authorization: Bearer {base}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])
        assert_eq([x["menu_key"] for x in rec_a["data"]], ["recent-A"], "tenant A recents")
        assert_eq([x["menu_key"] for x in rec_b["data"]], ["recent-B"], "tenant B recents")
        self.results["common_isolation"] = {"status": "passed"}

    def run_profile_rbac_misc(self):
        info("==> profile / permissions / users / roles")
        suffix = str(time.time_ns())[-8:]
        platform_view_headers = self.auth_args(self.platform_token)
        platform_headers = self.auth_args(self.platform_token, json_content=True)
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        platform_profile = curl_json([f"{self.api_base()}/auth/profile", *platform_view_headers])["data"]
        tenant_profile = curl_json([f"{self.api_base()}/auth/profile", *tenant_view_headers])["data"]
        assert_eq(platform_profile["username"], "admin", "platform profile username")
        assert_eq(tenant_profile["username"], "tenantadmin", "tenant profile username")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/auth/profile",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "display_name": "Tenant Admin Acceptance",
                        "email": "tenanta-admin@example.com",
                        "phone": "13800001111",
                    }
                ),
            ]
        )
        updated_tenant_profile = curl_json([f"{self.api_base()}/auth/profile", *tenant_view_headers])["data"]
        assert_eq(updated_tenant_profile["phone"], "13800001111", "tenant profile phone should update")

        platform_login_history = curl_json([f"{self.api_base()}/auth/profile/login-history?limit=5", *platform_view_headers])["data"]["items"]
        tenant_login_history = curl_json([f"{self.api_base()}/auth/profile/login-history?limit=5", *tenant_view_headers])["data"]["items"]
        tenant_activities = curl_json([f"{self.api_base()}/auth/profile/activities?limit=10", *tenant_view_headers])["data"]["items"]
        assert_true(len(platform_login_history) >= 1, "platform login history should be non-empty")
        assert_true(len(tenant_login_history) >= 1, "tenant login history should be non-empty")
        assert_true(len(tenant_activities) >= 1, "tenant profile activities should be non-empty")

        tenant_memberships = curl_json([f"{self.api_base()}/common/user/tenants", *tenant_view_headers])["data"]
        tenant_ids = {item["id"] for item in tenant_memberships}
        assert_true(self.tenant_a["id"] in tenant_ids, "common user tenants should include tenant A")
        assert_true(self.tenant_b["id"] in tenant_ids, "common user tenants should include tenant B")
        platform_tenants = curl_json([f"{self.api_base()}/common/user/tenants", *platform_view_headers])["data"]
        platform_tenant_ids = {item["id"] for item in platform_tenants}
        assert_true(self.tenant_a["id"] in platform_tenant_ids, "platform user tenants should include tenant A")

        platform_permissions = curl_json([f"{self.api_base()}/platform/permissions?module=platform", *platform_view_headers])["data"]
        platform_permission_tree = curl_json([f"{self.api_base()}/platform/permissions/tree", *platform_view_headers])["data"]
        tenant_permissions = curl_json([f"{self.api_base()}/tenant/permissions", *tenant_view_headers])["data"]
        tenant_permission_tree = curl_json([f"{self.api_base()}/tenant/permissions/tree", *tenant_view_headers])["data"]
        assert_true(any(item["code"] == "platform:users:list" for item in platform_permissions), "platform permissions should include users:list")
        assert_true("platform" in platform_permission_tree, "platform permission tree should include platform module")
        assert_true(any(item["code"] == "user:list" for item in tenant_permissions), "tenant permissions should include user:list")
        assert_true("user" in tenant_permission_tree, "tenant permission tree should include user module")

        platform_roles = curl_json([f"{self.api_base()}/platform/roles", *platform_view_headers])["data"]
        platform_role_names = {item["name"] for item in platform_roles}
        assert_true("platform_admin" in platform_role_names, "platform roles should include platform_admin")
        platform_simple_users = curl_json([f"{self.api_base()}/platform/users/simple?name=adm", *platform_view_headers])["data"]
        assert_true(any(item["username"] == "admin" for item in platform_simple_users), "platform simple users should include admin")

        role_permissions = [item["id"] for item in platform_permissions if item["code"] in ("platform:users:list", "platform:roles:list")]
        platform_role = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/roles",
                *platform_headers,
                "-d",
                json.dumps(
                    {
                        "name": f"acc_platform_role_{suffix}",
                        "display_name": f"Acceptance Platform Role {suffix}",
                        "description": "acceptance platform role",
                    }
                ),
            ]
        )["data"]
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/roles/{platform_role['id']}/permissions",
                *platform_headers,
                "-d",
                json.dumps({"permission_ids": role_permissions}),
            ]
        )
        platform_role_detail = curl_json([f"{self.api_base()}/platform/roles/{platform_role['id']}", *platform_view_headers])["data"]
        assert_eq(platform_role_detail["id"], platform_role["id"], "platform role detail id")
        empty_platform_role_users = curl_json([f"{self.api_base()}/platform/roles/{platform_role['id']}/users?page=1&page_size=20", *platform_view_headers])
        assert_eq(empty_platform_role_users["total"], 0, "new platform role should have no users")

        create_platform_user_status, create_platform_user_body = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/users",
                *platform_headers,
                "-d",
                json.dumps(
                    {
                        "username": f"accplat{suffix}",
                        "email": f"accplat{suffix}@example.com",
                        "password": "Tenant123456!",
                        "display_name": "Acceptance Platform User",
                        "role_id": platform_role["id"],
                    }
                ),
            ]
        )
        assert_eq(create_platform_user_status, 201, f"platform user create should succeed: {create_platform_user_body}")
        platform_user = create_platform_user_body["data"]
        listed_platform_users = curl_json([f"{self.api_base()}/platform/users?username=accplat{suffix}&page=1&page_size=20", *platform_view_headers])
        assert_true(any(item["id"] == platform_user["id"] for item in self.list_items(listed_platform_users)), "platform user should be listed")
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/users/{platform_user['id']}",
                *platform_headers,
                "-d",
                json.dumps({"display_name": "Acceptance Platform User Updated", "phone": "13900002222"}),
            ]
        )
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/users/{platform_user['id']}/reset-password",
                *platform_headers,
                "-d",
                json.dumps({"new_password": "Reset123456!"}),
            ]
        )
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/users/{platform_user['id']}/roles",
                *platform_headers,
                "-d",
                json.dumps({"role_ids": [platform_role["id"]]}),
            ]
        )
        platform_user_detail = curl_json([f"{self.api_base()}/platform/users/{platform_user['id']}", *platform_view_headers])["data"]
        assert_eq(platform_user_detail["phone"], "13900002222", "platform user phone should update")
        role_users = curl_json([f"{self.api_base()}/platform/roles/{platform_role['id']}/users?page=1&page_size=20", *platform_view_headers])
        assert_true(any(item["id"] == platform_user["id"] for item in self.list_items(role_users)), "platform role users should include created user")
        relogin_platform = self.login(f"accplat{suffix}", "Reset123456!")
        assert_eq(relogin_platform["user"]["username"], f"accplat{suffix}", "reset platform password should work")
        curl_json(["-X", "DELETE", f"{self.api_base()}/platform/users/{platform_user['id']}", *platform_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/platform/roles/{platform_role['id']}", *platform_view_headers])

        tenant_roles = curl_json([f"{self.api_base()}/tenant/roles", *tenant_view_headers])["data"]
        tenant_role_names = {item["name"] for item in tenant_roles}
        assert_true("admin" in tenant_role_names, "tenant roles should include admin")
        tenant_simple_users = curl_json([f"{self.api_base()}/tenant/users/simple?name=tenant&status=active", *tenant_view_headers])["data"]
        assert_true(any(item["username"] == "tenantadmin" for item in tenant_simple_users), "tenant simple users should include tenantadmin")

        tenant_role_permissions = [item["id"] for item in tenant_permissions if item["code"] in ("user:list", "role:list")]
        tenant_role = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/roles",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": f"acc_tenant_role_{suffix}",
                        "display_name": f"Acceptance Tenant Role {suffix}",
                        "description": "acceptance tenant role",
                    }
                ),
            ]
        )["data"]
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/roles/{tenant_role['id']}/permissions",
                *tenant_headers,
                "-d",
                json.dumps({"permission_ids": tenant_role_permissions}),
            ]
        )
        tenant_role_detail = curl_json([f"{self.api_base()}/tenant/roles/{tenant_role['id']}", *tenant_view_headers])["data"]
        assert_eq(tenant_role_detail["id"], tenant_role["id"], "tenant role detail id")
        empty_tenant_role_users = curl_json([f"{self.api_base()}/tenant/roles/{tenant_role['id']}/users?page=1&page_size=20", *tenant_view_headers])
        assert_eq(empty_tenant_role_users["total"], 0, "new tenant role should have no users")

        tenant_user = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/users",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "username": f"acctenant{suffix}",
                        "email": f"acctenant{suffix}@example.com",
                        "password": "Tenant123456!",
                        "display_name": "Acceptance Tenant User",
                    }
                ),
            ]
        )["data"]
        tenant_users = curl_json([f"{self.api_base()}/tenant/users?page=1&page_size=100", *tenant_view_headers])
        assert_true(any(item["id"] == tenant_user["id"] for item in self.list_items(tenant_users)), "tenant user should be listed")
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/users/{tenant_user['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"display_name": "Acceptance Tenant User Updated", "phone": "13700003333"}),
            ]
        )
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/users/{tenant_user['id']}/roles",
                *tenant_headers,
                "-d",
                json.dumps({"role_ids": [tenant_role["id"]]}),
            ]
        )
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/users/{tenant_user['id']}/reset-password",
                *tenant_headers,
                "-d",
                json.dumps({"new_password": "Reset123456!"}),
            ]
        )
        tenant_user_detail = curl_json([f"{self.api_base()}/tenant/users/{tenant_user['id']}", *tenant_view_headers])["data"]
        assert_eq(tenant_user_detail["phone"], "13700003333", "tenant user phone should update")
        tenant_role_users = curl_json([f"{self.api_base()}/tenant/roles/{tenant_role['id']}/users?page=1&page_size=20", *tenant_view_headers])
        assert_true(any(item["id"] == tenant_user["id"] for item in self.list_items(tenant_role_users)), "tenant role users should include created user")
        relogin_tenant = self.login(f"acctenant{suffix}", "Reset123456!")
        assert_eq(relogin_tenant["user"]["username"], f"acctenant{suffix}", "reset tenant password should work")
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/users/{tenant_user['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/roles/{tenant_role['id']}", *tenant_view_headers])

        self.results["profile_rbac_misc"] = {"status": "passed"}

    def run_workbench_site_messages(self):
        info("==> workbench / site messages")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)
        other_tenant_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.other_tenant_id)
        platform_view_headers = self.auth_args(self.platform_token)
        platform_headers = self.auth_args(self.platform_token, json_content=True)

        overview = curl_json([f"{self.api_base()}/common/workbench/overview", *tenant_view_headers])["data"]
        assert_true("system_health" in overview, "workbench overview should include system_health")
        assert_true("resource_overview" in overview, "workbench overview should include resource_overview")

        temp_role = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/roles",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": f"acc_workbench_role_{str(time.time_ns())[-6:]}",
                        "display_name": "Acceptance Workbench Role",
                        "description": "workbench activity mapping acceptance",
                    }
                ),
            ]
        )["data"]
        activities = curl_json([f"{self.api_base()}/common/workbench/activities?limit=20", *tenant_view_headers])["data"]["items"]
        assert_true(len(activities) >= 1, "workbench activities should be non-empty")
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/roles/{temp_role['id']}", *tenant_view_headers])

        workbench_schedule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-schedules",
                "-H",
                f"Authorization: Bearer {self.tenant_admin_token}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps(
                    {
                        "name": "acc-workbench-cron",
                        "task_id": self.local_task["id"],
                        "schedule_type": "cron",
                        "schedule_expr": "*/20 * * * *",
                        "description": "workbench calendar schedule",
                        "enabled": True,
                    }
                ),
            ]
        )["data"]
        now = time.localtime()
        schedule_calendar = curl_json(
            [
                f"{self.api_base()}/common/workbench/schedule-calendar?year={now.tm_year}&month={now.tm_mon}",
                *tenant_view_headers,
            ]
        )["data"]["dates"]
        assert_true(
            any(any(item["schedule_id"] == workbench_schedule["id"] for item in items) for items in schedule_calendar.values()),
            "workbench schedule calendar should include created schedule",
        )

        favorites_default = curl_json([f"{self.api_base()}/common/workbench/favorites", *tenant_view_headers])["data"]["items"]
        favorites_other = curl_json([f"{self.api_base()}/common/workbench/favorites", *other_tenant_headers])["data"]["items"]
        assert_true(any(item["key"] == "dash-A" for item in favorites_default), "default tenant favorites should be visible in workbench")
        assert_true(any(item["key"] == "dash-B" for item in favorites_other), "other tenant favorites should be isolated in workbench")

        categories = curl_json([f"{self.api_base()}/common/site-messages/categories", *tenant_view_headers])["data"]
        assert_true(any(item["value"] == "announcement" for item in categories), "site message categories should include announcement")

        site_settings_before = curl_json([f"{self.api_base()}/platform/site-messages/settings", *platform_view_headers])["data"]
        assert_true(site_settings_before["retention_days"] >= 1, "site message retention should be available")
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/platform/site-messages/settings",
                *platform_headers,
                "-d",
                json.dumps({"retention_days": 120}),
            ]
        )
        site_settings_after = curl_json([f"{self.api_base()}/platform/site-messages/settings", *platform_view_headers])["data"]
        assert_eq(site_settings_after["retention_days"], 120, "site message retention should update")

        unread_default_before = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", *tenant_view_headers])["data"]["unread_count"]
        unread_other_before = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", *other_tenant_headers])["data"]["unread_count"]

        for payload in [
            {"category": "announcement", "title": "WB Global", "content": "global announcement"},
            {"category": "announcement", "title": "WB Default Tenant", "content": "default tenant announcement", "target_tenant_ids": [self.default_tenant_id]},
            {"category": "announcement", "title": "WB Other Tenant", "content": "other tenant announcement", "target_tenant_ids": [self.other_tenant_id]},
        ]:
            status, _ = curl_status_json(
                [
                    "-X",
                    "POST",
                    f"{self.api_base()}/platform/site-messages",
                    *platform_headers,
                    "-d",
                    json.dumps(payload),
                ]
            )
            assert_true(status in (200, 201), "site message create should succeed")

        default_messages = curl_json([f"{self.api_base()}/tenant/site-messages?page=1&page_size=50&keyword=WB", *tenant_view_headers])
        other_messages = curl_json([f"{self.api_base()}/tenant/site-messages?page=1&page_size=50&keyword=WB", *other_tenant_headers])
        default_titles = [item["title"] for item in self.list_items(default_messages)]
        other_titles = [item["title"] for item in self.list_items(other_messages)]
        assert_true("WB Global" in default_titles, "default tenant should see global site message")
        assert_true("WB Default Tenant" in default_titles, "default tenant should see targeted site message")
        assert_true("WB Other Tenant" not in default_titles, "default tenant should not see other-tenant site message")
        assert_true("WB Global" in other_titles, "other tenant should see global site message")
        assert_true("WB Other Tenant" in other_titles, "other tenant should see targeted site message")
        assert_true("WB Default Tenant" not in other_titles, "other tenant should not see default-tenant site message")

        unread_default_after = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", *tenant_view_headers])["data"]["unread_count"]
        unread_other_after = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", *other_tenant_headers])["data"]["unread_count"]
        assert_true(unread_default_after >= unread_default_before + 2, "default tenant unread count should increase")
        assert_true(unread_other_after >= unread_other_before + 2, "other tenant unread count should increase")

        announcements_default = curl_json([f"{self.api_base()}/common/workbench/announcements?limit=10", *tenant_view_headers])["data"]["items"]
        announcements_other = curl_json([f"{self.api_base()}/common/workbench/announcements?limit=10", *other_tenant_headers])["data"]["items"]
        default_announcement_titles = [item["title"] for item in announcements_default]
        other_announcement_titles = [item["title"] for item in announcements_other]
        assert_true("WB Global" in default_announcement_titles, "workbench announcements should include global message")
        assert_true("WB Default Tenant" in default_announcement_titles, "workbench announcements should include tenant-targeted message")
        assert_true("WB Other Tenant" not in default_announcement_titles, "workbench announcements should exclude other tenant message")
        assert_true("WB Global" in other_announcement_titles, "other workbench announcements should include global message")
        assert_true("WB Other Tenant" in other_announcement_titles, "other workbench announcements should include target message")
        assert_true("WB Default Tenant" not in other_announcement_titles, "other workbench announcements should exclude default tenant message")

        unread_announcements = curl_json(
            [f"{self.api_base()}/tenant/site-messages?page=1&page_size=50&category=announcement&is_read=false&keyword=WB", *tenant_view_headers]
        )
        assert_true(len(self.list_items(unread_announcements)) >= 2, "announcement filter should work before read-all")
        marked = curl_json(["-X", "PUT", f"{self.api_base()}/tenant/site-messages/read-all", *tenant_view_headers, "-H", "Content-Type: application/json", "-d", "{}"])
        assert_true(marked["data"]["marked_count"] >= 1, "read-all should mark messages")
        unread_default_final = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", *tenant_view_headers])["data"]["unread_count"]
        assert_eq(unread_default_final, 0, "default tenant unread count should become zero after read-all")

        platform_status, _ = curl_status_json([f"{self.api_base()}/common/workbench/overview", *platform_view_headers])
        assert_true(platform_status in (400, 403), "platform user without impersonation should not access tenant workbench")

        self.results["workbench_site_messages"] = {"status": "passed"}

    def run_dashboard_overview_stats(self):
        info("==> dashboard overview / stats / conflicts")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        overview = curl_json(
            [
                f"{self.api_base()}/tenant/dashboard/overview?sections=incidents,cmdb,healing,execution,plugins,notifications,git,playbooks,secrets,users",
                *tenant_view_headers,
            ]
        )["data"]
        for key in ["incidents", "cmdb", "healing", "execution", "plugins", "notifications", "git", "playbooks", "secrets", "users"]:
            assert_true(key in overview, f"dashboard overview should include {key}")

        assert_true(overview["incidents"]["total"] >= 1, "dashboard incidents total should be non-zero")
        assert_true(overview["cmdb"]["total"] >= 1, "dashboard cmdb total should be non-zero")
        assert_true(overview["healing"]["flows_total"] >= 1, "dashboard healing flows should be non-zero")
        assert_true(overview["execution"]["tasks_total"] >= 1, "dashboard execution tasks should be non-zero")
        assert_true(overview["plugins"]["total"] >= 1, "dashboard plugin total should be non-zero")
        assert_true(overview["notifications"]["logs_total"] >= 1, "dashboard notification logs should be non-zero")
        assert_true(overview["git"]["repos_total"] >= 1, "dashboard git repos should be non-zero")
        assert_true(overview["playbooks"]["total"] >= 1, "dashboard playbooks total should be non-zero")
        assert_true(overview["secrets"]["total"] >= 1, "dashboard secrets total should be non-zero")

        tenant_users = curl_json([f"{self.api_base()}/tenant/users?page=1&page_size=100", *tenant_view_headers])
        assert_eq(overview["users"]["total"], tenant_users["total"], "dashboard users section should respect tenant membership")
        assert_true(overview["users"]["roles_total"] >= 2, "dashboard users roles_total should include tenant-visible roles")

        plugin_stats = curl_json([f"{self.api_base()}/tenant/plugins/stats", *tenant_view_headers])["data"]
        git_stats = curl_json([f"{self.api_base()}/tenant/git-repos/stats", *tenant_view_headers])["data"]
        playbook_stats = curl_json([f"{self.api_base()}/tenant/playbooks/stats", *tenant_view_headers])["data"]
        secrets_stats = curl_json([f"{self.api_base()}/tenant/secrets-sources/stats", *tenant_view_headers])["data"]
        assert_true(plugin_stats["total"] >= 1, "plugin stats should be non-zero")
        assert_true(git_stats["total"] >= 1, "git stats should be non-zero")
        assert_true(playbook_stats["total"] >= 1, "playbook stats should be non-zero")
        assert_true(secrets_stats["total"] >= 1, "secrets stats should be non-zero")

        protected_source = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-protected",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                        "is_default": False,
                        "priority": 9,
                    }
                ),
            ]
        )["data"]

        protected_task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-protected-secret",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "local",
                        "description": "task using protected secret",
                        "secrets_source_ids": [protected_source["id"]],
                    }
                ),
            ]
        )["data"]

        conflict_status, conflict_body = curl_status_json(
            ["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{protected_source['id']}", *tenant_view_headers]
        )
        assert_eq(conflict_status, 409, "deleting referenced secret source should conflict")
        assert_true("无法删除" in conflict_body["message"], "secret delete conflict message")

        protected_schedule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-schedules",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-protected-schedule",
                        "task_id": protected_task["id"],
                        "schedule_type": "cron",
                        "schedule_expr": "*/30 * * * *",
                        "description": "schedule using protected secret",
                        "enabled": False,
                        "secrets_source_ids": [protected_source["id"]],
                    }
                ),
            ]
        )["data"]

        conflict_status2, conflict_body2 = curl_status_json(
            ["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{protected_source['id']}", *tenant_view_headers]
        )
        assert_eq(conflict_status2, 409, "deleting secret source referenced by schedule should still conflict")
        assert_true("无法删除" in conflict_body2["message"], "secret delete conflict message after schedule")

        delete_schedule_status, _ = curl_status_json(
            ["-X", "DELETE", f"{self.api_base()}/tenant/execution-schedules/{protected_schedule['id']}", *tenant_view_headers]
        )
        assert_eq(delete_schedule_status, 200, "deleting protected schedule should succeed")
        delete_task_status, _ = curl_status_json(
            ["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{protected_task['id']}", *tenant_view_headers]
        )
        assert_eq(delete_task_status, 200, "deleting protected task should succeed")
        delete_source_status, _ = curl_status_json(
            ["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{protected_source['id']}", *tenant_view_headers]
        )
        assert_eq(delete_source_status, 200, "deleting unlinked secret source should succeed")

        self.protected_secret_source = protected_source
        self.protected_secret_task = protected_task
        self.protected_secret_schedule = protected_schedule
        self.results["dashboard_overview_stats"] = {"status": "passed"}

    def run_secrets_reference_updates(self):
        info("==> secrets reference lifecycle")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        source = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-unlink",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                        "priority": 11,
                    }
                ),
            ]
        )["data"]

        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-secret-unlink",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "local",
                        "description": "task linked to secret source",
                        "secrets_source_ids": [source["id"]],
                    }
                ),
            ]
        )["data"]

        schedule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-schedules",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-schedule-secret-unlink",
                        "task_id": task["id"],
                        "schedule_type": "cron",
                        "schedule_expr": "*/45 * * * *",
                        "description": "schedule linked to secret source",
                        "enabled": False,
                        "secrets_source_ids": [source["id"]],
                    }
                ),
            ]
        )["data"]

        conflict1, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source['id']}", *tenant_view_headers])
        assert_eq(conflict1, 409, "referenced secret delete should conflict")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"secrets_source_ids": []}),
            ]
        )
        conflict2, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source['id']}", *tenant_view_headers])
        assert_eq(conflict2, 409, "secret delete should still conflict while schedule references it")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/execution-schedules/{schedule['id']}",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": schedule["name"],
                        "schedule_type": schedule["schedule_type"],
                        "schedule_expr": schedule["schedule_expr"],
                        "description": schedule["description"],
                        "max_failures": schedule["max_failures"],
                        "secrets_source_ids": [],
                    }
                ),
            ]
        )
        deleted, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source['id']}", *tenant_view_headers])
        assert_eq(deleted, 200, "secret delete should succeed after unlinking task and schedule")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-schedules/{schedule['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{task['id']}", *tenant_view_headers])

        self.results["secrets_reference_updates"] = {"status": "passed"}

    def run_secrets_update_constraints(self):
        info("==> secrets update constraints")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        source = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-update-guard",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                        "priority": 13,
                    }
                ),
            ]
        )["data"]

        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-secret-update-guard",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "local",
                        "description": "task linked to guarded secret",
                        "secrets_source_ids": [source["id"]],
                    }
                ),
            ]
        )["data"]

        schedule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-schedules",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-schedule-secret-update-guard",
                        "task_id": task["id"],
                        "schedule_type": "cron",
                        "schedule_expr": "*/50 * * * *",
                        "description": "schedule linked to guarded secret",
                        "enabled": False,
                        "secrets_source_ids": [source["id"]],
                    }
                ),
            ]
        )["data"]

        config_conflict_status, config_conflict_body = curl_status_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/secrets-sources/{source['id']}",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret-alt",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        }
                    }
                ),
            ]
        )
        assert_eq(config_conflict_status, 409, "updating referenced secret config should conflict")
        assert_true("无法更新" in config_conflict_body["message"], "referenced secret config conflict message")

        updated = curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/secrets-sources/{source['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"priority": 0, "is_default": True}),
            ]
        )["data"]
        assert_eq(updated["priority"], 0, "referenced secret priority should still update")
        assert_eq(updated["is_default"], True, "referenced secret can become default")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-schedules/{schedule['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{task['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source['id']}", *tenant_view_headers])

        self.results["secrets_update_constraints"] = {"status": "passed"}

    def run_blacklist_security(self):
        info("==> command blacklist / exemptions")
        tenant_b_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"], json_content=True)
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])

        blacklists_schema = curl_json([f"{self.api_base()}/tenant/command-blacklist/search-schema", *tenant_b_view_headers])
        exemptions_schema = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions/search-schema", *tenant_b_view_headers])
        assert_true(len(blacklists_schema["fields"]) >= 1, "command blacklist search schema should be non-empty")
        assert_true(len(exemptions_schema["fields"]) >= 1, "blacklist exemption search schema should be non-empty")

        repo = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/git-repos",
                *tenant_b_headers,
                "-d",
                json.dumps({"name": "acc-blacklist-repo", "url": f"file://{self.repo_dir}", "default_branch": "main", "auth_type": "none", "auth_config": {}, "sync_enabled": False}),
            ]
        )["data"]
        playbook = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/playbooks",
                *tenant_b_headers,
                "-d",
                json.dumps({"repository_id": repo["id"], "name": "acc-blacklist-playbook", "file_path": "local.yml", "description": "blacklist playbook", "config_mode": "auto"}),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{playbook['id']}/scan", *tenant_b_view_headers])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{playbook['id']}/ready", *tenant_b_view_headers])
        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_b_headers,
                "-d",
                json.dumps({"name": "acc-blacklist-task", "playbook_id": playbook["id"], "target_hosts": "localhost", "executor_type": "local", "description": "blacklist task"}),
            ]
        )["data"]

        rule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-blacklist-rule",
                        "pattern": "rm -rf",
                        "match_type": "contains",
                        "severity": "critical",
                        "category": "system",
                        "description": "dangerous rm rule",
                        "is_active": True,
                    }
                ),
            ]
        )["data"]
        listed_rules = curl_json([f"{self.api_base()}/tenant/command-blacklist?page=1&page_size=20&name=acc-blacklist", *tenant_b_view_headers])
        assert_true(any(item["id"] == rule["id"] for item in self.list_items(listed_rules)), "custom blacklist rule should be listed")
        rule_detail = curl_json([f"{self.api_base()}/tenant/command-blacklist/{rule['id']}", *tenant_b_view_headers])["data"]
        assert_eq(rule_detail["id"], rule["id"], "blacklist rule detail id")

        all_rules = curl_json([f"{self.api_base()}/tenant/command-blacklist?page=1&page_size=50", *tenant_b_view_headers])
        system_rule = next(item for item in self.list_items(all_rules) if item.get("is_system"))
        system_toggled = curl_json(["-X", "POST", f"{self.api_base()}/tenant/command-blacklist/{system_rule['id']}/toggle", *tenant_b_view_headers])["data"]
        system_detail = curl_json([f"{self.api_base()}/tenant/command-blacklist/{system_rule['id']}", *tenant_b_view_headers])["data"]
        assert_eq(system_detail["is_active"], system_toggled["is_active"], "system blacklist detail should reflect tenant override")

        simulation = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist/simulate",
                *tenant_b_headers,
                "-d",
                json.dumps({"pattern": "rm -rf", "match_type": "contains", "content": "echo ok\nrm -rf /\n"}),
            ]
        )["data"]
        assert_eq(simulation["match_count"], 1, "blacklist simulate should match one line")

        updated_rule = curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/command-blacklist/{rule['id']}",
                *tenant_b_headers,
                "-d",
                json.dumps({"description": "dangerous rm rule updated", "severity": "high"}),
            ]
        )["data"]
        assert_eq(updated_rule["severity"], "high", "blacklist rule severity should update")

        toggled_rule = curl_json(["-X", "POST", f"{self.api_base()}/tenant/command-blacklist/{rule['id']}/toggle", *tenant_b_view_headers])["data"]
        assert_eq(toggled_rule["is_active"], False, "blacklist toggle should disable rule")
        batch_toggle = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist/batch-toggle",
                *tenant_b_headers,
                "-d",
                json.dumps({"ids": [rule["id"]], "is_active": True}),
            ]
        )
        assert_eq(batch_toggle["count"], 1, "blacklist batch-toggle should affect one rule")
        rule_after_batch = curl_json([f"{self.api_base()}/tenant/command-blacklist/{rule['id']}", *tenant_b_view_headers])["data"]
        assert_eq(rule_after_batch["is_active"], True, "blacklist batch-toggle should re-enable rule")

        blocking_rule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-blacklist-acceptance-ok",
                        "pattern": "acceptance-ok",
                        "match_type": "contains",
                        "severity": "critical",
                        "category": "system",
                        "description": "block acceptance echo playbook",
                        "is_active": True,
                    }
                ),
            ]
        )["data"]
        self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["is_active"] is True
                else None
            )(curl_json([f"{self.api_base()}/tenant/command-blacklist/{blocking_rule['id']}", *tenant_b_view_headers])),
            timeout=10,
            interval=1,
            description="blocking blacklist rule visible",
        )
        time.sleep(1)

        blocked_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_b_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        blocked_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{blocked_run['id']}", *tenant_b_view_headers])),
            timeout=20,
            interval=1,
            description="custom blacklist blocked execution",
        )
        assert_eq(blocked_final["status"], "failed", "custom blacklist rule should block execution")

        requester = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/users",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "username": "blacklistreq",
                        "email": "blacklistreq@example.com",
                        "password": "Tenant123456!",
                        "display_name": "Blacklist Requester",
                    }
                ),
            ]
        )["data"]
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/users/{requester['id']}/roles",
                *tenant_b_headers,
                "-d",
                json.dumps({"role_ids": [self.system_tenant_roles["admin"]["id"]]}),
            ]
        )
        requester_login = self.login("blacklistreq", "Tenant123456!")
        requester_headers = self.auth_args(requester_login["access_token"], tenant_id=self.tenant_b["id"], json_content=True)
        requester_view_headers = self.auth_args(requester_login["access_token"], tenant_id=self.tenant_b["id"])

        exemption = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions",
                *requester_headers,
                "-d",
                json.dumps(
                    {
                        "task_id": task["id"],
                        "task_name": task["name"],
                        "rule_id": blocking_rule["id"],
                        "rule_name": blocking_rule["name"],
                        "rule_severity": blocking_rule["severity"],
                        "rule_pattern": blocking_rule["pattern"],
                        "reason": "temporary exception for safe maintenance",
                        "validity_days": 7,
                    }
                ),
            ]
        )["id"]
        dup_status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions",
                *requester_headers,
                "-d",
                json.dumps(
                    {
                        "task_id": task["id"],
                        "task_name": task["name"],
                        "rule_id": blocking_rule["id"],
                        "rule_name": blocking_rule["name"],
                        "rule_severity": blocking_rule["severity"],
                        "rule_pattern": blocking_rule["pattern"],
                        "reason": "duplicate pending exception",
                        "validity_days": 7,
                    }
                ),
            ]
        )
        assert_eq(dup_status, 400, "duplicate pending exemption should fail")
        pending = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions/pending?page=1&page_size=20", *tenant_b_view_headers])
        assert_true(any(item["id"] == exemption for item in self.list_items(pending)), "pending blacklist exemption should be listed")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/blacklist-exemptions/{exemption}/approve", *tenant_b_headers, "-d", "{}"])
        approved = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions/{exemption}", *tenant_b_view_headers])
        assert_eq(approved["status"], "approved", "blacklist exemption should approve")

        exemption2 = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions",
                *requester_headers,
                "-d",
                json.dumps(
                    {
                        "task_id": task["id"],
                        "task_name": task["name"],
                        "rule_id": rule["id"],
                        "rule_name": rule["name"],
                        "rule_severity": rule_after_batch["severity"],
                        "rule_pattern": rule_after_batch["pattern"],
                        "reason": "second exception to reject",
                        "validity_days": 7,
                    }
                ),
            ]
        )["id"]
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions/{exemption2}/reject",
                *tenant_b_headers,
                "-d",
                json.dumps({"reject_reason": "not justified"}),
            ]
        )
        rejected = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions/{exemption2}", *tenant_b_view_headers])
        assert_eq(rejected["status"], "rejected", "blacklist exemption should reject")

        listed_exemptions = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions?page=1&page_size=20&search=exception", *tenant_b_view_headers])
        assert_true(len(self.list_items(listed_exemptions)) >= 2, "blacklist exemptions list should be queryable")

        danger_playbook = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/playbooks",
                *tenant_b_headers,
                "-d",
                json.dumps({"repository_id": repo["id"], "name": "acc-blacklist-danger", "file_path": "danger.yml", "description": "blacklist system rule playbook", "config_mode": "auto"}),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{danger_playbook['id']}/scan", *tenant_b_view_headers])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{danger_playbook['id']}/ready", *tenant_b_view_headers])
        danger_task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_b_headers,
                "-d",
                json.dumps({"name": "acc-blacklist-system-task", "playbook_id": danger_playbook["id"], "target_hosts": "localhost", "executor_type": "local", "description": "blacklist system task"}),
            ]
        )["data"]
        system_rules = curl_json([f"{self.api_base()}/tenant/command-blacklist?page=1&page_size=100&name=删除数据库", *tenant_b_view_headers])
        system_rule = next(item for item in self.list_items(system_rules) if item["name"] == "删除数据库")
        assert_true(system_rule["is_system"], "seeded blacklist rule should be a system rule")
        toggled_system = curl_json(["-X", "POST", f"{self.api_base()}/tenant/command-blacklist/{system_rule['id']}/toggle", *tenant_b_view_headers])["data"]
        assert_eq(toggled_system["is_active"], True, "system blacklist toggle should enable tenant override")
        other_tenant_rules = curl_json([f"{self.api_base()}/tenant/command-blacklist?page=1&page_size=100&name=删除数据库", *self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_a["id"])])
        other_tenant_system_rule = next(item for item in self.list_items(other_tenant_rules) if item["name"] == "删除数据库")
        assert_eq(other_tenant_system_rule["is_active"], False, "system blacklist override should not leak to other tenant")
        blocked_system_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{danger_task['id']}/execute",
                *tenant_b_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        blocked_system_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{blocked_system_run['id']}", *tenant_b_view_headers])),
            timeout=20,
            interval=1,
            description="system blacklist blocked execution",
        )
        assert_eq(blocked_system_final["status"], "failed", "enabled system blacklist override should block execution")
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist/batch-toggle",
                *tenant_b_headers,
                "-d",
                json.dumps({"ids": [system_rule["id"]], "is_active": False}),
            ]
        )
        system_rule_after_disable = curl_json([f"{self.api_base()}/tenant/command-blacklist/{system_rule['id']}", *tenant_b_view_headers])["data"]
        assert_eq(system_rule_after_disable["is_active"], False, "system blacklist batch-toggle should disable tenant override")

        delete_rule_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/command-blacklist/{rule['id']}", *tenant_b_view_headers])
        assert_eq(delete_rule_status, 200, "custom blacklist rule delete should succeed")
        delete_block_rule_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/command-blacklist/{blocking_rule['id']}", *tenant_b_view_headers])
        assert_eq(delete_block_rule_status, 200, "blocking blacklist rule delete should succeed")
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{danger_task['id']}", *tenant_b_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/playbooks/{danger_playbook['id']}", *tenant_b_view_headers])

        self.results["blacklist_security"] = {"status": "passed"}

    def run_blacklist_exemption_execution(self):
        info("==> blacklist exemption execution")
        tenant_b_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"], json_content=True)
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])

        repo = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/git-repos",
                *tenant_b_headers,
                "-d",
                json.dumps({"name": "acc-blacklist-exempt-repo", "url": f"file://{self.repo_dir}", "default_branch": "main", "auth_type": "none", "auth_config": {}, "sync_enabled": False}),
            ]
        )["data"]
        playbook = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/playbooks",
                *tenant_b_headers,
                "-d",
                json.dumps({"repository_id": repo["id"], "name": "acc-blacklist-exempt-playbook", "file_path": "local.yml", "description": "blacklist exemption playbook", "config_mode": "auto"}),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{playbook['id']}/scan", *tenant_b_view_headers])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{playbook['id']}/ready", *tenant_b_view_headers])
        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_b_headers,
                "-d",
                json.dumps({"name": "acc-blacklist-exempt-task", "playbook_id": playbook["id"], "target_hosts": "localhost", "executor_type": "local", "description": "blacklist exemption task"}),
            ]
        )["data"]

        blocking_rule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-blacklist-exempt-block",
                        "pattern": "acceptance-ok",
                        "match_type": "contains",
                        "severity": "critical",
                        "category": "system",
                        "description": "block acceptance echo playbook for exemption scenario",
                        "is_active": True,
                    }
                ),
            ]
        )["data"]

        blocked_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_b_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        blocked_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{blocked_run['id']}", *tenant_b_view_headers])),
            timeout=20,
            interval=1,
            description="blacklist exemption initial blocked run",
        )
        assert_eq(blocked_final["status"], "failed", "blocking rule should fail initial execution")

        requester = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/users",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "username": "blackexempt",
                        "email": "blackexempt@example.com",
                        "password": "Tenant123456!",
                        "display_name": "Blacklist Exemptor",
                    }
                ),
            ]
        )["data"]
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/users/{requester['id']}/roles",
                *tenant_b_headers,
                "-d",
                json.dumps({"role_ids": [self.system_tenant_roles["admin"]["id"]]}),
            ]
        )
        requester_login = self.login("blackexempt", "Tenant123456!")
        requester_headers = self.auth_args(requester_login["access_token"], tenant_id=self.tenant_b["id"], json_content=True)

        invalid_task_status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions",
                *requester_headers,
                "-d",
                json.dumps(
                    {
                        "task_id": str(uuid.uuid4()),
                        "rule_id": blocking_rule["id"],
                        "reason": "should fail for unknown task",
                        "validity_days": 7,
                    }
                ),
            ]
        )
        assert_eq(invalid_task_status, 400, "unknown task exemption should fail")
        invalid_rule_status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions",
                *requester_headers,
                "-d",
                json.dumps(
                    {
                        "task_id": task["id"],
                        "rule_id": str(uuid.uuid4()),
                        "reason": "should fail for unknown rule",
                        "validity_days": 7,
                    }
                ),
            ]
        )
        assert_eq(invalid_rule_status, 400, "unknown rule exemption should fail")

        exemption = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/blacklist-exemptions",
                *requester_headers,
                "-d",
                json.dumps(
                    {
                        "task_id": task["id"],
                        "task_name": task["name"],
                        "rule_id": blocking_rule["id"],
                        "rule_name": blocking_rule["name"],
                        "rule_severity": blocking_rule["severity"],
                        "rule_pattern": blocking_rule["pattern"],
                        "reason": "allow this maintenance task temporarily",
                        "validity_days": 7,
                    }
                ),
            ]
        )["id"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/blacklist-exemptions/{exemption}/approve", *tenant_b_headers, "-d", "{}"])
        approved = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions/{exemption}", *tenant_b_view_headers])
        assert_eq(approved["status"], "approved", "exemption should approve")

        allowed_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_b_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        allowed_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{allowed_run['id']}", *tenant_b_view_headers])),
            timeout=20,
            interval=1,
            description="blacklist exemption allowed run",
        )
        assert_eq(allowed_final["status"], "success", "approved exemption should allow execution")

        twin_rule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/command-blacklist",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-blacklist-exempt-twin",
                        "pattern": blocking_rule["pattern"],
                        "match_type": blocking_rule["match_type"],
                        "severity": "critical",
                        "category": "system",
                        "description": "same pattern but different rule id",
                        "is_active": True,
                    }
                ),
            ]
        )["data"]
        twin_blocked_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_b_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        twin_blocked_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{twin_blocked_run['id']}", *tenant_b_view_headers])),
            timeout=20,
            interval=1,
            description="blacklist exemption twin rule block",
        )
        assert_eq(twin_blocked_final["status"], "failed", "exemption should not bypass a different rule with same pattern")
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/command-blacklist/{twin_rule['id']}", *tenant_b_view_headers])

        self.psql(
            "UPDATE blacklist_exemptions "
            f"SET expires_at = NOW() - INTERVAL '1 minute', updated_at = NOW() "
            f"WHERE id = '{exemption}'"
        )
        expired = self.wait_until(
            lambda: (
                lambda payload: payload if payload.get("status") == "expired" else None
            )(curl_json([f"{self.api_base()}/tenant/blacklist-exemptions/{exemption}", *tenant_b_view_headers])),
            timeout=10,
            interval=1,
            description="blacklist exemption expiry status",
        )
        assert_eq(expired["status"], "expired", "expired exemption should be normalized in detail")
        expired_list = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions?status=expired&page=1&page_size=20", *tenant_b_view_headers])
        assert_true(any(item["id"] == exemption for item in self.list_items(expired_list)), "expired exemption should be listed as expired")
        approved_list = curl_json([f"{self.api_base()}/tenant/blacklist-exemptions?status=approved&page=1&page_size=20", *tenant_b_view_headers])
        assert_true(all(item["id"] != exemption for item in self.list_items(approved_list)), "expired exemption should not remain in approved list")

        blocked_again_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_b_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        blocked_again_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{blocked_again_run['id']}", *tenant_b_view_headers])),
            timeout=20,
            interval=1,
            description="blacklist exemption blocked after expiry",
        )
        assert_eq(blocked_again_final["status"], "failed", "expired exemption should block execution again")

        self.results["blacklist_exemption_execution"] = {"status": "passed"}

    def run_audit_action_assertions(self):
        info("==> audit action assertions")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])
        platform_view_headers = self.auth_args(self.platform_token)
        platform_headers = self.auth_args(self.platform_token, json_content=True)

        temp_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-audit-delete-channel",
                        "type": "webhook",
                        "description": "temporary delete-audit channel",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                    }
                ),
            ]
        )["data"]
        delete_channel_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{temp_channel['id']}", *tenant_view_headers])
        assert_eq(delete_channel_status, 200, "temporary tenant channel delete should succeed")

        temp_tenant = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/tenants",
                *platform_headers,
                "-d",
                json.dumps({"name": "Acceptance Audit Temp", "code": f"acc_audit_{int(time.time())}", "description": "audit temp", "icon": "team"}),
            ]
        )["data"]
        delete_tenant_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/platform/tenants/{temp_tenant['id']}", *platform_view_headers])
        assert_eq(delete_tenant_status, 200, "temporary platform tenant delete should succeed")

        tenant_delete_logs = curl_json([f"{self.api_base()}/tenant/audit-logs?action=delete&page=1&page_size=20", *tenant_view_headers])
        assert_true(tenant_delete_logs["data"] is None or isinstance(tenant_delete_logs["data"], list), "tenant delete action filter payload")

        platform_delete_logs = curl_json([f"{self.api_base()}/platform/audit-logs?action=delete&page=1&page_size=20", *platform_view_headers])
        assert_true(platform_delete_logs["data"] is None or isinstance(platform_delete_logs["data"], list), "platform delete action filter payload")
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])
        tenant_reset_logs = curl_json([f"{self.api_base()}/tenant/audit-logs?action=reset_password&page=1&page_size=20", *tenant_b_view_headers])
        assert_true(tenant_reset_logs["data"] is None or isinstance(tenant_reset_logs["data"], list), "tenant reset_password action filter payload")

        tenant_high = curl_json([f"{self.api_base()}/tenant/audit-logs/high-risk?page=1&page_size=20", *tenant_b_view_headers])
        platform_high = curl_json([f"{self.api_base()}/platform/audit-logs/high-risk?page=1&page_size=20", *platform_view_headers])
        for payload in [tenant_high, platform_high]:
            items = self.list_items(payload) if payload["data"] is not None else []
            for item in items:
                assert_true(bool(item.get("risk_reason")), "high-risk audit items should include risk_reason")

        self.results["audit_action_assertions"] = {"status": "passed"}

    def run_filters_pagination(self):
        info("==> filters / pagination / empty states")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])
        platform_view_headers = self.auth_args(self.platform_token)

        tasks_page1 = curl_json([f"{self.api_base()}/tenant/execution-tasks?page=1&page_size=1&sort_by=created_at&sort_order=asc", *tenant_view_headers])
        tasks_page2 = curl_json([f"{self.api_base()}/tenant/execution-tasks?page=2&page_size=1&sort_by=created_at&sort_order=asc", *tenant_view_headers])
        assert_true(tasks_page1["total"] >= 2, "execution task total should support pagination")
        assert_eq(len(self.list_items(tasks_page1)), 1, "execution tasks page 1 size")
        assert_eq(len(self.list_items(tasks_page2)), 1, "execution tasks page 2 size")
        assert_true(self.list_items(tasks_page1)[0]["id"] != self.list_items(tasks_page2)[0]["id"], "execution task pages should differ")

        docker_tasks = curl_json([f"{self.api_base()}/tenant/execution-tasks?executor_type=docker&page=1&page_size=20", *tenant_view_headers])
        assert_true(any(item["id"] == self.docker_task["id"] for item in self.list_items(docker_tasks)), "docker task filter should work")
        empty_tasks = curl_json([f"{self.api_base()}/tenant/execution-tasks?name=not-exist&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_tasks)), 0, "execution tasks empty filter should return empty list")

        cancelled_runs = curl_json([f"{self.api_base()}/tenant/execution-runs?status=cancelled&task_id={self.long_task['id']}&page=1&page_size=20", *tenant_view_headers])
        assert_true(any(item["id"] == self.cancelled_run["id"] for item in self.list_items(cancelled_runs)), "cancelled run filter should work")
        empty_runs = curl_json([f"{self.api_base()}/tenant/execution-runs?run_id=00000000&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_runs)), 0, "execution runs empty filter should return empty list")

        incident_page = curl_json([f"{self.api_base()}/tenant/incidents?plugin_id={self.plugin_itsm['id']}&page=1&page_size=1", *tenant_view_headers])
        assert_eq(len(self.list_items(incident_page)), 1, "incidents page size should work")
        empty_incidents = curl_json([f"{self.api_base()}/tenant/incidents?external_id=no-such-incident&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_incidents)), 0, "incident empty filter should return empty list")

        cmdb_prod = curl_json([f"{self.api_base()}/tenant/cmdb?plugin_id={self.plugin_cmdb['id']}&environment=production&page=1&page_size=20", *tenant_view_headers])
        assert_true(len(self.list_items(cmdb_prod)) >= 1, "cmdb environment filter should return data")
        empty_cmdb = curl_json([f"{self.api_base()}/tenant/cmdb?environment=nonexistent&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_cmdb)), 0, "cmdb empty filter should return empty list")

        sent_notifications = curl_json([f"{self.api_base()}/tenant/notifications?status=sent&subject=Execution&page=1&page_size=20", *tenant_view_headers])
        assert_true(len(self.list_items(sent_notifications)) >= 1, "notification subject/status filter should work")
        empty_notifications = curl_json([f"{self.api_base()}/tenant/notifications?subject=no-such-subject&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_notifications)), 0, "notification empty filter should return empty list")

        git_filtered = curl_json([f"{self.api_base()}/tenant/git-repos?name=acc-repo-host&page=1&page_size=20", *tenant_view_headers])
        assert_true(len(self.list_items(git_filtered)) >= 1, "git repo name filter should work")
        empty_git = curl_json([f"{self.api_base()}/tenant/git-repos?name=no-such-repo&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_git)), 0, "git empty filter should return empty list")

        playbooks_ready = curl_json([f"{self.api_base()}/tenant/playbooks?status=ready&page=1&page_size=20", *tenant_view_headers])
        assert_true(len(self.list_items(playbooks_ready)) >= 1, "playbook status filter should work")
        empty_playbooks = curl_json([f"{self.api_base()}/tenant/playbooks?name=no-such-playbook&page=1&page_size=20", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_playbooks)), 0, "playbook empty filter should return empty list")

        messages_page = curl_json([f"{self.api_base()}/tenant/site-messages?page=1&page_size=1&category=announcement", *tenant_b_view_headers])
        assert_eq(len(self.list_items(messages_page)), 1, "site messages page size should work")
        empty_messages = curl_json([f"{self.api_base()}/tenant/site-messages?page=1&page_size=20&keyword=no-such-message", *tenant_view_headers])
        assert_eq(len(self.list_items(empty_messages)), 0, "site messages empty filter should return empty list")

        tenant_audit_create = curl_json([f"{self.api_base()}/tenant/audit-logs?action=create&page=1&page_size=20", *tenant_view_headers])
        assert_true(isinstance(self.list_items(tenant_audit_create), list), "tenant audit action filter should be queryable")
        tenant_audit_search = curl_json([f"{self.api_base()}/tenant/audit-logs?search=channels&page=1&page_size=20", *tenant_view_headers])
        assert_true(isinstance(self.list_items(tenant_audit_search), list), "tenant audit search filter should be queryable")
        platform_audit_operation = curl_json([f"{self.api_base()}/platform/audit-logs?category=operation&page=1&page_size=20", *platform_view_headers])
        assert_true(isinstance(self.list_items(platform_audit_operation), list), "platform audit category filter should be queryable")
        platform_audit_search = curl_json([f"{self.api_base()}/platform/audit-logs?search=Acceptance&page=1&page_size=20", *platform_view_headers])
        assert_true(isinstance(self.list_items(platform_audit_search), list), "platform audit search filter should be queryable")

        self.results["filters_pagination"] = {"status": "passed"}

    def run_dashboard(self):
        info("==> dashboard isolation")
        token = self.tenant_admin_token
        headers = ["-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json"]
        other_headers = ["-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json", "-H", f"X-Tenant-ID: {self.other_tenant_id}"]

        curl_json(["-X", "PUT", f"{self.api_base()}/tenant/dashboard/config", *headers, "-d", json.dumps({"activeWorkspaceId": "wa", "workspaces": [{"id": "wa", "name": "A", "widgets": [], "layouts": []}]} )])
        curl_json(["-X", "PUT", f"{self.api_base()}/tenant/dashboard/config", *other_headers, "-d", json.dumps({"activeWorkspaceId": "wb", "workspaces": [{"id": "wb", "name": "B", "widgets": [], "layouts": []}]} )])
        cfg_a = curl_json([f"{self.api_base()}/tenant/dashboard/config", "-H", f"Authorization: Bearer {token}"])
        cfg_b = curl_json([f"{self.api_base()}/tenant/dashboard/config", "-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])
        assert_eq(cfg_a["data"]["config"]["activeWorkspaceId"], "wa", "dashboard config tenant A")
        assert_eq(cfg_b["data"]["config"]["activeWorkspaceId"], "wb", "dashboard config tenant B")

        ws_a = curl_json(["-X", "POST", f"{self.api_base()}/tenant/dashboard/workspaces", *headers, "-d", json.dumps({"name": "Workspace A", "description": "tenant A ws", "config": {"widgets": [], "layouts": []}})])
        ws_b = curl_json(["-X", "POST", f"{self.api_base()}/tenant/dashboard/workspaces", *other_headers, "-d", json.dumps({"name": "Workspace B", "description": "tenant B ws", "config": {"widgets": [], "layouts": []}})])
        list_a = curl_json([f"{self.api_base()}/tenant/dashboard/workspaces", "-H", f"Authorization: Bearer {token}"])
        list_b = curl_json([f"{self.api_base()}/tenant/dashboard/workspaces", "-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])
        assert_eq([x["name"] for x in list_a["data"]], [ws_a["data"]["name"]], "dashboard workspaces tenant A")
        assert_eq([x["name"] for x in list_b["data"]], [ws_b["data"]["name"]], "dashboard workspaces tenant B")
        self.results["dashboard"] = {"status": "passed"}

    def _assert_git_contract_endpoints(self, token):
        view_headers = self.auth_args(token)
        existing_logs = curl_json([f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}/logs?page=1&page_size=20", *view_headers])
        existing_items = self.list_items(existing_logs)
        known_log_ids = {item["id"] for item in existing_items}
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}/sync", *view_headers])
        logs_payload = self.wait_until(
            lambda: (
                lambda payload: payload
                if any(item["id"] not in known_log_ids for item in self.list_items(payload))
                else None
            )(curl_json([f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}/logs?page=1&page_size=20", *view_headers])),
            timeout=20,
            interval=1,
            description="git sync logs",
        )
        new_logs = [item for item in self.list_items(logs_payload) if item["id"] not in known_log_ids]
        assert_true(len(new_logs) >= 1, "git repo logs should include a newly created sync record")
        assert_true(new_logs[0].get("status") in ("pending", "running", "success", "failed"), "git sync log should expose status")
        reset_error = curl_json(["-X", "POST", f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}/reset-status?status=error", *view_headers])
        assert_eq(reset_error["message"], "状态已重置为 error", "git reset-status should acknowledge error")
        repo_after_error = curl_json([f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}", *view_headers])["data"]
        assert_eq(repo_after_error["status"], "error", "git repo status should become error after reset-status")
        reset_pending = curl_json(["-X", "POST", f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}/reset-status?status=pending", *view_headers])
        assert_eq(reset_pending["message"], "状态已重置为 pending", "git reset-status should acknowledge pending")
        repo_after_pending = curl_json([f"{self.api_base()}/tenant/git-repos/{self.git_repo['id']}", *view_headers])["data"]
        assert_eq(repo_after_pending["status"], "pending", "git repo status should become pending after reset-status")
        scan_logs = curl_json([f"{self.api_base()}/tenant/playbooks/{self.local_playbook['id']}/scan-logs?page=1&page_size=20", *view_headers])
        assert_true(len(self.list_items(scan_logs)) >= 1, "playbook scan logs should be queryable")

    def _assert_batch_confirm_review(self, token):
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)
        playbook_detail = curl_json([f"{self.api_base()}/tenant/playbooks/{self.local_playbook['id']}", *view_headers])["data"]
        variables = list(playbook_detail.get("variables") or [])
        variables.append({"name": f"acceptance_contract_{int(time.time())}", "type": "string", "required": False})
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/playbooks/{self.local_playbook['id']}/variables",
                *headers,
                "-d",
                json.dumps({"variables": variables}),
            ]
        )
        review_task = self.wait_until(
            lambda: (
                lambda payload: payload["data"] if payload["data"]["needs_review"] else None
            )(curl_json([f"{self.api_base()}/tenant/execution-tasks/{self.local_task['id']}", *view_headers])),
            timeout=20,
            interval=1,
            description="execution task review flag",
        )
        assert_true(len(review_task.get("changed_variables") or []) >= 1, "task should expose changed variables after playbook update")
        batch_payload = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/batch-confirm-review",
                *headers,
                "-d",
                json.dumps({"playbook_id": self.local_playbook["id"]}),
            ]
        )["data"]
        assert_true(batch_payload["confirmed_count"] >= 1, "batch confirm review should confirm at least one task")
        cleared_task = self.wait_until(
            lambda: (
                lambda payload: payload["data"] if not payload["data"]["needs_review"] else None
            )(curl_json([f"{self.api_base()}/tenant/execution-tasks/{self.local_task['id']}", *view_headers])),
            timeout=20,
            interval=1,
            description="execution task review cleared",
        )
        assert_eq(len(cleared_task.get("changed_variables") or []), 0, "changed variables should be cleared after batch confirm review")

    def _assert_healing_retry_contract(self, token):
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)
        suffix = str(time.time_ns())[-8:]
        plugin = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/plugins",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": f"acc-retry-plugin-{suffix}",
                        "type": "itsm",
                        "config": {"url": f"http://127.0.0.1:{self.itsm_port}/api/now/table/incident", "auth_type": "none", "response_data_path": "result"},
                        "field_mapping": {
                            "incident_mapping": {
                                "external_id": "sys_id",
                                "title": "short_description",
                                "description": "description",
                                "priority": "priority",
                                "status": "state",
                                "category": "category",
                                "affected_ci": "cmdb_ci",
                                "affected_service": "business_service",
                                "assignee": "assigned_to",
                                "reporter": "opened_by",
                            }
                        },
                        "sync_enabled": False,
                    }
                ),
            ]
        )["data"]
        flow = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/flows",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": f"acc-retry-flow-{suffix}",
                        "description": "retry contract flow",
                        "nodes": [
                            {"id": "start1", "type": "start", "name": "开始", "position": {"x": 0, "y": 0}, "config": {}},
                            {
                                "id": "exec_fail",
                                "type": "execution",
                                "name": "失败执行",
                                "position": {"x": 220, "y": 0},
                                "config": {"task_template_id": str(uuid.uuid4())},
                            },
                        ],
                        "edges": [{"source": "start1", "target": "exec_fail"}],
                        "is_active": True,
                    }
                ),
            ]
        )["data"]
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/rules",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": f"acc-retry-rule-{suffix}",
                        "description": "retry contract rule",
                        "priority": 999,
                        "trigger_mode": "manual",
                        "match_mode": "all",
                        "flow_id": flow["id"],
                        "is_active": True,
                        "conditions": [{"type": "condition", "field": "title", "operator": "contains", "value": "E2E-HEALING"}],
                    }
                ),
            ]
        )
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{plugin['id']}/sync", *view_headers])
        pending_incident = self.wait_until(
            lambda: (
                lambda payload: self.list_items(payload)[0] if self.list_items(payload) else None
            )(curl_json([f"{self.api_base()}/tenant/healing/pending/trigger?page=1&page_size=20", *view_headers])),
            timeout=30,
            interval=2,
            description="retry pending incident",
        )
        failed_instance = curl_json(["-X", "POST", f"{self.api_base()}/tenant/incidents/{pending_incident['id']}/trigger", *view_headers])["data"]
        failed_terminal = self.wait_until(
            lambda: (
                lambda payload: payload["data"] if payload["data"]["status"] in ("failed", "completed", "cancelled") else None
            )(curl_json([f"{self.api_base()}/tenant/healing/instances/{failed_instance['id']}", *view_headers])),
            timeout=30,
            interval=1,
            description="retry failed instance",
        )
        assert_eq(failed_terminal["status"], "failed", "retry smoke should first create a failed instance")
        previous_updated_at = failed_terminal.get("updated_at")
        retry_payload = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/instances/{failed_instance['id']}/retry",
                *headers,
                "-d",
                "{}",
            ]
        )
        assert_eq(retry_payload["message"], "流程实例正在重试", "healing retry should acknowledge async retry")
        retried_terminal = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] == "failed" and payload["data"].get("updated_at") != previous_updated_at
                else None
            )(curl_json([f"{self.api_base()}/tenant/healing/instances/{failed_instance['id']}", *view_headers])),
            timeout=30,
            interval=1,
            description="retry should re-run failed instance",
        )
        assert_eq(retried_terminal["status"], "failed", "retry contract should re-run and return terminal failed state for invalid task id")

    def _assert_dashboard_contract_endpoints(self, token):
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)
        suffix = str(time.time_ns())[-6:]
        role = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/roles",
                *headers,
                "-d",
                json.dumps({"name": f"acc_dashboard_contract_{suffix}", "display_name": f"Acceptance Dashboard Role {suffix}", "description": "dashboard contract role"}),
            ]
        )["data"]
        workspace = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/dashboard/workspaces",
                *headers,
                "-d",
                json.dumps({"name": f"Contract WS {suffix}", "description": "interface smoke", "config": {"widgets": [], "layouts": []}}),
            ]
        )["data"]
        updated = curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/dashboard/workspaces/{workspace['id']}",
                *headers,
                "-d",
                json.dumps({"name": f"Contract WS {suffix} Updated", "description": "updated", "config": {"widgets": [], "layouts": []}}),
            ]
        )["data"]
        assert_eq(updated["name"], f"Contract WS {suffix} Updated", "dashboard workspace update should persist")
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/dashboard/roles/{role['id']}/workspaces",
                *headers,
                "-d",
                json.dumps({"workspace_ids": [workspace["id"]]}),
            ]
        )
        role_workspaces = curl_json([f"{self.api_base()}/tenant/dashboard/roles/{role['id']}/workspaces", *view_headers])["data"]["workspace_ids"]
        assert_true(workspace["id"] in role_workspaces, "dashboard role-workspaces should include assigned workspace")
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/dashboard/workspaces/{workspace['id']}", *headers])
        remaining = curl_json([f"{self.api_base()}/tenant/dashboard/workspaces", *view_headers])["data"]
        assert_true(all(item["id"] != workspace["id"] for item in remaining), "dashboard workspace delete should remove workspace from list")

    def run_interface_contract_smoke(self):
        info("==> interface contract smoke")
        token = self.tenant_admin_token
        self._assert_git_contract_endpoints(token)
        self._assert_batch_confirm_review(token)
        self._assert_healing_retry_contract(token)
        self._assert_dashboard_contract_endpoints(token)
        self.results["interface_contract_smoke"] = {"status": "passed"}

    def run_tenant_boundaries(self):
        info("==> tenant user/role boundaries")
        token = self.tenant_admin_token
        a_headers = ["-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.tenant_a['id']}"]
        b_headers = ["-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.tenant_b['id']}"]
        role = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/roles",
                *b_headers,
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"name": "ops_b_custom", "display_name": "Ops B Custom", "description": "tenant B custom role"}),
            ]
        )
        role_id = role["data"]["id"]
        user_cross, _ = curl_status_json([f"{self.api_base()}/tenant/users/{self.tenant_viewer['id']}", *a_headers])
        user_ok, _ = curl_status_json([f"{self.api_base()}/tenant/users/{self.tenant_viewer['id']}", *b_headers])
        role_cross, _ = curl_status_json([f"{self.api_base()}/tenant/roles/{role_id}", *a_headers])
        role_ok, _ = curl_status_json([f"{self.api_base()}/tenant/roles/{role_id}", *b_headers])
        assert_eq(user_cross, 404, "cross-tenant user lookup should fail")
        assert_eq(user_ok, 200, "tenant B user lookup should succeed")
        assert_eq(role_cross, 404, "cross-tenant role lookup should fail")
        assert_eq(role_ok, 200, "tenant B role lookup should succeed")
        self.results["tenant_boundaries"] = {"status": "passed"}

    def run_search_and_site_messages(self):
        info("==> search and site message isolation")
        token = self.tenant_admin_token
        headers = ["-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json"]
        other_headers = ["-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json", "-H", f"X-Tenant-ID: {self.other_tenant_id}"]
        default_plugin_name = "plugin-default-tenant"
        other_plugin_name = "plugin-other-tenant"
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins", *headers, "-d", json.dumps({"name": default_plugin_name, "type": "itsm", "config": {"url": "http://itsm-a.local"}})])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins", *other_headers, "-d", json.dumps({"name": other_plugin_name, "type": "itsm", "config": {"url": "http://itsm-b.local"}})])
        self.plugin_name_by_tenant = {
            self.default_tenant_id: default_plugin_name,
            self.other_tenant_id: other_plugin_name,
        }
        search_default = curl_json([f"{self.api_base()}/common/search?q=plugin-", "-H", f"Authorization: Bearer {token}"])
        search_b = curl_json([f"{self.api_base()}/common/search?q=plugin-", "-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])

        def titles(payload):
            result = []
            for category in payload.get("data", {}).get("results", []):
                for item in category.get("items", []):
                    result.append(item.get("title"))
            return result

        assert_eq(titles(search_default), [default_plugin_name], "default tenant search")
        assert_eq(titles(search_b), [other_plugin_name], "explicit tenant search")

        create = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/site-messages",
                "-H",
                f"Authorization: Bearer {self.platform_token}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps(
                    {
                        "category": "announcement",
                        "title": "Tenant split msg",
                        "content": "acceptance targeted message",
                        "target_tenant_ids": [self.default_tenant_id, self.other_tenant_id],
                    }
                ),
            ]
        )
        assert_eq(create["code"], 0, "create targeted site message")
        count_a_before = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", "-H", f"Authorization: Bearer {token}"])["data"]["unread_count"]
        count_b_before = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", "-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])["data"]["unread_count"]
        list_a = curl_json([f"{self.api_base()}/tenant/site-messages?page=1&page_size=20", "-H", f"Authorization: Bearer {token}"])
        list_b = curl_json([f"{self.api_base()}/tenant/site-messages?page=1&page_size=20", "-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])
        a_msg = next(x for x in list_a["data"] if x["title"] == "Tenant split msg")
        next(x for x in list_b["data"] if x["title"] == "Tenant split msg")
        curl_json(["-X", "PUT", f"{self.api_base()}/tenant/site-messages/read", "-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json", "-d", json.dumps({"ids": [a_msg["id"]]})])
        count_a_after = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", "-H", f"Authorization: Bearer {token}"])["data"]["unread_count"]
        count_b_after = curl_json([f"{self.api_base()}/tenant/site-messages/unread-count", "-H", f"Authorization: Bearer {token}", "-H", f"X-Tenant-ID: {self.other_tenant_id}"])["data"]["unread_count"]
        assert_true(count_a_after == count_a_before - 1, "tenant A unread count should decrease")
        assert_eq(count_b_after, count_b_before, "tenant B unread count should stay unchanged")
        self.results["search_site_messages"] = {"status": "passed"}

    def run_impersonation(self):
        info("==> impersonation")
        tenant_token = self.tenant_admin_token
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/settings/impersonation-approvers",
                "-H",
                f"Authorization: Bearer {tenant_token}",
                "-H",
                f"X-Tenant-ID: {self.tenant_a['id']}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"user_ids": [self.tenant_admin["id"]]}),
            ]
        )
        req = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/impersonation/requests",
                "-H",
                f"Authorization: Bearer {self.platform_token}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"tenant_id": self.tenant_a["id"], "reason": "acceptance", "duration_minutes": 30}),
            ]
        )["data"]
        pending = curl_json([f"{self.api_base()}/tenant/impersonation/pending", "-H", f"Authorization: Bearer {tenant_token}", "-H", f"X-Tenant-ID: {self.tenant_a['id']}"])
        apps = pending["data"]
        assert_true(any(x["id"] == req["id"] for x in apps), "pending impersonation request should be visible")
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/impersonation/{req['id']}/approve",
                "-H",
                f"Authorization: Bearer {tenant_token}",
                "-H",
                f"X-Tenant-ID: {self.tenant_a['id']}",
                "-H",
                "Content-Type: application/json",
                "-d",
                "{}",
            ]
        )
        enter = curl_json(["-X", "POST", f"{self.api_base()}/platform/impersonation/requests/{req['id']}/enter", "-H", f"Authorization: Bearer {self.platform_token}"])
        assert_eq(enter["data"]["status"], "active", "impersonation session should become active")
        tenant_a_plugin = self.plugin_name_by_tenant[self.tenant_a["id"]]
        search = curl_json(
            [
                f"{self.api_base()}/common/search?q={tenant_a_plugin}",
                "-H",
                f"Authorization: Bearer {self.platform_token}",
                "-H",
                "X-Impersonation: true",
                "-H",
                f"X-Impersonation-Request-ID: {req['id']}",
                "-H",
                f"X-Tenant-ID: {self.tenant_a['id']}",
            ]
        )
        assert_true(search["data"]["total_count"] >= 1, "impersonated platform user should see tenant data")
        curl_json(["-X", "POST", f"{self.api_base()}/platform/impersonation/requests/{req['id']}/exit", "-H", f"Authorization: Bearer {self.platform_token}"])
        req2 = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/platform/impersonation/requests",
                "-H",
                f"Authorization: Bearer {self.platform_token}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"tenant_id": self.tenant_a["id"], "reason": "cancel-before-approve", "duration_minutes": 15}),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/platform/impersonation/requests/{req2['id']}/cancel", "-H", f"Authorization: Bearer {self.platform_token}"])
        status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/impersonation/{req2['id']}/approve",
                "-H",
                f"Authorization: Bearer {tenant_token}",
                "-H",
                f"X-Tenant-ID: {self.tenant_a['id']}",
                "-H",
                "Content-Type: application/json",
                "-d",
                "{}",
            ]
        )
        assert_eq(status, 400, "cancelled impersonation request should not be approvable")
        self.results["impersonation"] = {"status": "passed"}

    def run_healing_flow(self):
        info("==> healing approval flow")
        token = self.tenant_admin_token
        headers = ["-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json"]
        flow_nodes = [
            {"id": "start1", "type": "start", "name": "开始", "position": {"x": 0, "y": 0}, "config": {}},
            {"id": "approval1", "type": "approval", "name": "审批", "position": {"x": 200, "y": 0}, "config": {"title": "Acceptance Approval", "description": "approval for acceptance", "timeout_hours": 1}},
            {"id": "end_ok", "type": "end", "name": "结束", "position": {"x": 420, "y": -80}, "config": {}},
            {"id": "end_reject", "type": "end", "name": "拒绝结束", "position": {"x": 420, "y": 80}, "config": {}},
        ]
        flow_edges = [
            {"source": "start1", "target": "approval1"},
            {"source": "approval1", "target": "end_ok", "sourceHandle": "approved"},
            {"source": "approval1", "target": "end_reject", "sourceHandle": "rejected"},
        ]
        flow = curl_json(["-X", "POST", f"{self.api_base()}/tenant/healing/flows", *headers, "-d", json.dumps({"name": "acc-flow-approval", "description": "acceptance approval flow", "nodes": flow_nodes, "edges": flow_edges, "is_active": True})])["data"]
        rule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/rules",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-rule-manual",
                        "description": "manual acceptance rule",
                        "priority": 10,
                        "trigger_mode": "manual",
                        "match_mode": "all",
                        "flow_id": flow["id"],
                        "is_active": True,
                        "conditions": [{"type": "condition", "field": "title", "operator": "contains", "value": "E2E-HEALING"}],
                    }
                ),
            ]
        )["data"]
        plugin = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/plugins",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-itsm-mock",
                        "type": "itsm",
                        "config": {"url": f"http://127.0.0.1:{self.itsm_port}/api/now/table/incident", "auth_type": "none", "response_data_path": "result"},
                        "field_mapping": {
                            "incident_mapping": {
                                "external_id": "sys_id",
                                "title": "short_description",
                                "description": "description",
                                "priority": "priority",
                                "status": "state",
                                "category": "category",
                                "affected_ci": "cmdb_ci",
                                "affected_service": "business_service",
                                "assignee": "assigned_to",
                                "reporter": "opened_by",
                            }
                        },
                        "sync_enabled": False,
                    }
                ),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{plugin['id']}/sync", "-H", f"Authorization: Bearer {token}"])
        pending_items = self.wait_pending_trigger_items(token)
        success_incident = self.pick_pending_incident(pending_items)
        incident_id = success_incident["id"]
        instance = curl_json(["-X", "POST", f"{self.api_base()}/tenant/incidents/{incident_id}/trigger", "-H", f"Authorization: Bearer {token}"])["data"]
        approvals = []
        for _ in range(12):
            time.sleep(1)
            resp = curl_json([f"{self.api_base()}/tenant/healing/approvals", "-H", f"Authorization: Bearer {token}"])
            approvals = resp["data"] if isinstance(resp["data"], list) else resp["data"]["items"]
            approvals = [a for a in approvals if a.get("flow_instance_id") == instance["id"] and a.get("status") == "pending"]
            if approvals:
                break
        assert_true(approvals, "approval task should be created")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/healing/approvals/{approvals[0]['id']}/approve", "-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json", "-d", json.dumps({"comment": "approved"})])
        final = self.wait_until(
            lambda: (
                lambda payload: payload
                if payload["data"]["status"] in ("completed", "failed", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/healing/instances/{instance['id']}", "-H", f"Authorization: Bearer {token}"])),
            timeout=30,
            interval=1,
            description="approved healing flow terminal state",
        )
        assert_eq(final["data"]["status"], "completed", "approved healing flow should complete")

        # cancel path
        pending_items = self.wait_pending_trigger_items(token)
        cancel_incident = self.pick_pending_incident(pending_items, exclude_ids={incident_id}, allow_fail=True)
        incident_id = cancel_incident["id"]
        instance2 = curl_json(["-X", "POST", f"{self.api_base()}/tenant/incidents/{incident_id}/trigger", "-H", f"Authorization: Bearer {token}"])["data"]
        approval2 = None
        for _ in range(12):
            time.sleep(1)
            resp = curl_json([f"{self.api_base()}/tenant/healing/approvals", "-H", f"Authorization: Bearer {token}"])
            approvals = resp["data"] if isinstance(resp["data"], list) else resp["data"]["items"]
            matches = [a for a in approvals if a.get("flow_instance_id") == instance2["id"] and a.get("status") == "pending"]
            if matches:
                approval2 = matches[0]
                break
        assert_true(approval2 is not None, "second approval task should be created")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/healing/instances/{instance2['id']}/cancel", "-H", f"Authorization: Bearer {token}"])
        inst2 = curl_json([f"{self.api_base()}/tenant/healing/instances/{instance2['id']}", "-H", f"Authorization: Bearer {token}"])
        assert_eq(inst2["data"]["status"], "cancelled", "cancelled flow should stay cancelled")
        approval2_detail = curl_json([f"{self.api_base()}/tenant/healing/approvals/{approval2['id']}", "-H", f"Authorization: Bearer {token}"])
        assert_eq(approval2_detail["data"]["status"], "cancelled", "approval task should be cancelled with flow")
        status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/approvals/{approval2['id']}/approve",
                "-H",
                f"Authorization: Bearer {token}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps({"comment": "should fail"}),
            ]
        )
        assert_true(status in (400, 409), "approval after cancel should be rejected")
        self.healing_flow = flow
        self.healing_rule = rule
        self.healing_plugin = plugin
        self.healing_completed_instance = final["data"]
        self.healing_cancelled_instance = inst2["data"]
        self.healing_approved_task = approvals[0]
        self.healing_cancelled_task = approval2_detail["data"]
        self.results["healing"] = {"status": "passed"}

    def run_healing_queries(self):
        info("==> healing query surfaces")
        token = self.tenant_admin_token
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)

        node_schema = curl_json([f"{self.api_base()}/tenant/healing/flows/node-schema", *view_headers])
        flow_schema = curl_json([f"{self.api_base()}/tenant/healing/flows/search-schema", *view_headers])
        rule_schema = curl_json([f"{self.api_base()}/tenant/healing/rules/search-schema", *view_headers])
        instance_schema = curl_json([f"{self.api_base()}/tenant/healing/instances/search-schema", *view_headers])
        assert_true("nodes" in node_schema["data"], "healing node schema should expose nodes")
        assert_true(len(flow_schema["data"]["fields"]) >= 1, "flow search schema should return fields")
        assert_true(len(rule_schema["data"]["fields"]) >= 1, "rule search schema should return fields")
        assert_true(len(instance_schema["data"]["fields"]) >= 1, "instance search schema should return fields")

        flows = curl_json([f"{self.api_base()}/tenant/healing/flows?name={self.healing_flow['name']}", *view_headers])
        flow_items = self.list_items(flows)
        assert_true(any(item["id"] == self.healing_flow["id"] for item in flow_items), "healing flow should be listed")
        flow_detail = curl_json([f"{self.api_base()}/tenant/healing/flows/{self.healing_flow['id']}", *view_headers])
        flow_stats = curl_json([f"{self.api_base()}/tenant/healing/flows/stats", *view_headers])
        assert_eq(flow_detail["data"]["id"], self.healing_flow["id"], "healing flow detail")
        assert_true(isinstance(flow_stats["data"], dict), "healing flow stats payload")

        dry_run_body = {
            "mock_incident": {
                "title": "E2E-HEALING DRY RUN",
                "description": "dry run validation",
                "severity": "high",
                "priority": "2",
            },
            "mock_approvals": {"approval1": "approved"},
        }
        dry_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/flows/{self.healing_flow['id']}/dry-run",
                *headers,
                "-d",
                json.dumps(dry_run_body),
            ]
        )
        assert_true(dry_run["data"]["success"], "healing dry run should succeed")
        assert_true(
            any(node["node_id"] == "approval1" for node in dry_run["data"]["nodes"]),
            "healing dry run should include approval node",
        )
        dry_run_stream = subprocess.run(
            [
                "curl",
                "-sN",
                "--max-time",
                "8",
                "-X",
                "POST",
                f"{self.api_base()}/tenant/healing/flows/{self.healing_flow['id']}/dry-run-stream",
                "-H",
                f"Authorization: Bearer {token}",
                "-H",
                "Content-Type: application/json",
                "-d",
                json.dumps(dry_run_body),
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
        )
        assert_true("event: flow_complete" in dry_run_stream.stdout, "dry-run stream should complete")

        rules = curl_json(
            [f"{self.api_base()}/tenant/healing/rules?flow_id={self.healing_flow['id']}&name={self.healing_rule['name']}", *view_headers]
        )
        rule_items = self.list_items(rules)
        assert_true(any(item["id"] == self.healing_rule["id"] for item in rule_items), "healing rule should be listed")
        rule_detail = curl_json([f"{self.api_base()}/tenant/healing/rules/{self.healing_rule['id']}", *view_headers])
        rule_stats = curl_json([f"{self.api_base()}/tenant/healing/rules/stats", *view_headers])
        assert_eq(rule_detail["data"]["id"], self.healing_rule["id"], "healing rule detail")
        assert_true(isinstance(rule_stats["data"], dict), "healing rule stats payload")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/healing/rules/{self.healing_rule['id']}/deactivate", *view_headers])
        rule_after_deactivate = curl_json([f"{self.api_base()}/tenant/healing/rules/{self.healing_rule['id']}", *view_headers])
        assert_eq(rule_after_deactivate["data"]["is_active"], False, "rule should deactivate")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/healing/rules/{self.healing_rule['id']}/activate", *view_headers])
        rule_after_activate = curl_json([f"{self.api_base()}/tenant/healing/rules/{self.healing_rule['id']}", *view_headers])
        assert_eq(rule_after_activate["data"]["is_active"], True, "rule should activate")

        instances = curl_json([f"{self.api_base()}/tenant/healing/instances?flow_id={self.healing_flow['id']}", *view_headers])
        instance_items = self.list_items(instances)
        instance_ids = {item["id"] for item in instance_items}
        assert_true(self.healing_completed_instance["id"] in instance_ids, "completed instance should be listed")
        assert_true(self.healing_cancelled_instance["id"] in instance_ids, "cancelled instance should be listed")
        instance_stats = curl_json([f"{self.api_base()}/tenant/healing/instances/stats", *view_headers])
        instance_detail = curl_json([f"{self.api_base()}/tenant/healing/instances/{self.healing_completed_instance['id']}", *view_headers])
        assert_eq(instance_detail["data"]["status"], "completed", "completed instance detail")
        assert_true(isinstance(instance_stats["data"], dict), "instance stats payload")
        event_stream = subprocess.run(
            [
                "curl",
                "-sN",
                "--max-time",
                "5",
                f"{self.api_base()}/tenant/healing/instances/{self.healing_cancelled_instance['id']}/events",
                "-H",
                f"Authorization: Bearer {token}",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
        )
        assert_true("flow_complete" in event_stream.stdout, "instance events should finish for cancelled instance")

        approvals = curl_json([f"{self.api_base()}/tenant/healing/approvals?page=1&page_size=50", *view_headers])
        approval_items = self.list_items(approvals)
        assert_true(any(item["id"] == self.healing_approved_task["id"] for item in approval_items), "approved task should be listed")
        assert_true(any(item["id"] == self.healing_cancelled_task["id"] for item in approval_items), "cancelled task should be listed")
        filtered_approvals = curl_json([f"{self.api_base()}/tenant/healing/approvals?flow_instance_id={self.healing_completed_instance['id']}", *view_headers])
        assert_true(isinstance(self.list_items(filtered_approvals), list), "filtered approvals should be queryable")
        pending_approvals = curl_json([f"{self.api_base()}/tenant/healing/approvals/pending", *view_headers])
        assert_true(isinstance(self.list_items(pending_approvals), list), "pending approvals should be queryable")
        approval_detail = curl_json([f"{self.api_base()}/tenant/healing/approvals/{self.healing_cancelled_task['id']}", *view_headers])
        assert_eq(approval_detail["data"]["status"], "cancelled", "cancelled approval detail")

        pending = curl_json([f"{self.api_base()}/tenant/healing/pending/trigger", *view_headers])
        pending_items = self.list_items(pending)
        assert_true(isinstance(pending_items, list), "pending trigger list should be queryable")
        if pending_items:
            dismissed_target = pending_items[0]
            curl_json(["-X", "POST", f"{self.api_base()}/tenant/incidents/{dismissed_target['id']}/dismiss", *view_headers])
            dismissed = curl_json([f"{self.api_base()}/tenant/healing/pending/dismissed", *view_headers])
            dismissed_items = self.list_items(dismissed)
            assert_true(
                any(item["id"] == dismissed_target["id"] for item in dismissed_items),
                "dismissed trigger incident should be queryable",
            )

        self.results["healing_queries"] = {"status": "passed"}

    def run_git_playbook_execution(self):
        info("==> git / playbook / execution / schedule")
        token = self.tenant_admin_token
        headers = ["-H", f"Authorization: Bearer {token}", "-H", "Content-Type: application/json"]
        validate = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/git-repos/validate",
                *headers,
                "-d",
                json.dumps({"url": f"file://{self.repo_dir}", "auth_type": "none", "auth_config": {}}),
            ]
        )["data"]
        assert_true("main" in validate["branches"], "git validate should return main branch")
        assert_eq(validate["default_branch"], "main", "git validate default branch")
        repo = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/git-repos",
                *headers,
                "-d",
                json.dumps({"name": "acc-repo-host", "url": f"file://{self.repo_dir}", "default_branch": "main", "auth_type": "none", "auth_config": {}, "sync_enabled": False}),
            ]
        )["data"]
        commits = curl_json([f"{self.api_base()}/tenant/git-repos/{repo['id']}/commits", "-H", f"Authorization: Bearer {token}"])
        files = curl_json([f"{self.api_base()}/tenant/git-repos/{repo['id']}/files?path=", "-H", f"Authorization: Bearer {token}"])
        assert_true(len(commits["data"]) >= 1, "git commits should be listed")
        assert_true(any(x["name"] == "local.yml" for x in files["data"]["files"]), "git files should include local.yml")

        playbook = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/playbooks",
                *headers,
                "-d",
                json.dumps({"repository_id": repo["id"], "name": "acc-playbook-local", "file_path": "local.yml", "description": "local host execution playbook", "config_mode": "auto"}),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{playbook['id']}/scan", "-H", f"Authorization: Bearer {token}"])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{playbook['id']}/ready", "-H", f"Authorization: Bearer {token}"])

        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *headers,
                "-d",
                json.dumps({"name": "acc-task-local", "playbook_id": playbook["id"], "target_hosts": "localhost", "executor_type": "local", "description": "host local execution acceptance"}),
            ]
        )["data"]
        run_resp = curl_json(["-X", "POST", f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute", *headers, "-d", json.dumps({"target_hosts": "localhost"})])["data"]
        final = None
        for _ in range(20):
            time.sleep(1)
            final = curl_json([f"{self.api_base()}/tenant/execution-runs/{run_resp['id']}", "-H", f"Authorization: Bearer {token}"])
            if final["data"]["status"] in ("success", "failed", "partial", "cancelled"):
                break
        assert_eq(final["data"]["status"], "success", "local execution should succeed")

        # long run + cancel + stream done
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/git-repos/{repo['id']}/sync", "-H", f"Authorization: Bearer {token}"])
        time.sleep(2)
        long_playbook = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/playbooks",
                *headers,
                "-d",
                json.dumps({"repository_id": repo["id"], "name": "acc-playbook-long", "file_path": "long.yml", "description": "long host execution playbook", "config_mode": "auto"}),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{long_playbook['id']}/scan", "-H", f"Authorization: Bearer {token}"])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/playbooks/{long_playbook['id']}/ready", "-H", f"Authorization: Bearer {token}"])
        long_task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *headers,
                "-d",
                json.dumps({"name": "acc-task-long", "playbook_id": long_playbook["id"], "target_hosts": "localhost", "executor_type": "local", "description": "long execution"}),
            ]
        )["data"]
        long_run = curl_json(["-X", "POST", f"{self.api_base()}/tenant/execution-tasks/{long_task['id']}/execute", *headers, "-d", json.dumps({"target_hosts": "localhost"})])["data"]
        for _ in range(10):
            time.sleep(1)
            current = curl_json([f"{self.api_base()}/tenant/execution-runs/{long_run['id']}", "-H", f"Authorization: Bearer {token}"])
            if current["data"]["status"] == "running":
                break
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/execution-runs/{long_run['id']}/cancel", "-H", f"Authorization: Bearer {token}"])
        final_cancel = None
        for _ in range(15):
            time.sleep(1)
            final_cancel = curl_json([f"{self.api_base()}/tenant/execution-runs/{long_run['id']}", "-H", f"Authorization: Bearer {token}"])
            if final_cancel["data"]["status"] == "cancelled":
                break
        assert_eq(final_cancel["data"]["status"], "cancelled", "cancelled execution should stay cancelled")
        stream_status, stream_body = curl_status_json([f"{self.api_base()}/tenant/execution-runs/{long_run['id']}/stream", "-H", f"Authorization: Bearer {token}"])
        assert_eq(stream_status, 200, "execution stream status")
        assert_true("event: done" in stream_body, "execution stream should finish with done")

        # once schedule should not complete before terminal run
        scheduled_at = time.gmtime(time.time() + 20)
        scheduled_at_str = time.strftime("%Y-%m-%dT%H:%M:%SZ", scheduled_at)
        schedule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-schedules",
                *headers,
                "-d",
                json.dumps({"name": "acc-once-long", "task_id": long_task["id"], "schedule_type": "once", "scheduled_at": scheduled_at_str, "description": "once long acceptance", "enabled": True}),
            ]
        )["data"]
        snapshots = []
        terminal_run_state = self.wait_until(
            lambda: (
                lambda current, run_items: (
                    snapshots.append((current["status"], run_items[0]["status"] if run_items else None)),
                    (current["status"], run_items[0]["status"])
                    if run_items and run_items[0]["status"] in ("success", "failed", "partial", "cancelled") and current["status"] == "completed"
                    else None
                )[1]
            )(
                curl_json([f"{self.api_base()}/tenant/execution-schedules/{schedule['id']}", "-H", f"Authorization: Bearer {token}"])["data"],
                (
                    lambda payload: payload["data"] if isinstance(payload["data"], list) else payload["data"]["items"]
                )(curl_json([f"{self.api_base()}/tenant/execution-runs?triggered_by=scheduler:once&task_id={long_task['id']}", "-H", f"Authorization: Bearer {token}"])),
            ),
            timeout=60,
            interval=1,
            description="terminal once schedule run",
        )
        bad = [s for s in snapshots if s[1] in (None, "pending", "running") and s[0] == "completed"]
        assert_true(not bad, "once schedule should not be completed before run terminal state")
        observed_scheduler_runs = [run_status for _, run_status in snapshots if run_status is not None]
        assert_true(observed_scheduler_runs, "once schedule should create at least one scheduler-triggered run")
        assert_true(terminal_run_state[1] in ("success", "failed", "partial", "cancelled"), "once schedule should eventually produce a terminal scheduler-triggered run")
        assert_true(terminal_run_state[0] == "completed", "once schedule should eventually become completed")
        self.git_repo = repo
        self.local_playbook = playbook
        self.long_playbook = long_playbook
        self.local_task = task
        self.long_task = long_task
        self.local_run = final["data"]
        self.cancelled_run = final_cancel["data"]
        self.once_schedule = schedule
        self.results["git_execution"] = {"status": "passed"}

    def run_plugin_cmdb(self):
        info("==> plugin / incidents / cmdb")
        token = self.tenant_admin_token
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)

        plugin_schema = curl_json([f"{self.api_base()}/tenant/plugins/search-schema", *view_headers])
        incident_schema = curl_json([f"{self.api_base()}/tenant/incidents/search-schema", *view_headers])
        cmdb_schema = curl_json([f"{self.api_base()}/tenant/cmdb/search-schema", *view_headers])
        assert_true(len(plugin_schema["data"]["fields"]) >= 1, "plugin search schema should return fields")
        assert_true(len(incident_schema["data"]["fields"]) >= 1, "incident search schema should return fields")
        assert_true(len(cmdb_schema["data"]["fields"]) >= 1, "cmdb search schema should return fields")

        itsm_plugin = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/plugins",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-plugin-itsm",
                        "type": "itsm",
                        "description": "acceptance itsm plugin",
                        "config": {
                            "url": f"http://127.0.0.1:{self.itsm_port}/api/now/table/incident",
                            "auth_type": "none",
                            "response_data_path": "result",
                        },
                        "field_mapping": {
                            "incident_mapping": {
                                "external_id": "sys_id",
                                "title": "short_description",
                                "description": "description",
                                "priority": "priority",
                                "status": "state",
                                "category": "category",
                                "affected_ci": "cmdb_ci",
                                "affected_service": "business_service",
                                "assignee": "assigned_to",
                                "reporter": "opened_by",
                            }
                        },
                        "sync_enabled": False,
                    }
                ),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{itsm_plugin['id']}/test", *view_headers])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{itsm_plugin['id']}/activate", *view_headers])
        itsm_detail = curl_json([f"{self.api_base()}/tenant/plugins/{itsm_plugin['id']}", *view_headers])
        assert_eq(itsm_detail["data"]["status"], "active", "itsm plugin should activate")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{itsm_plugin['id']}/sync", *view_headers])

        def wait_sync(plugin_id):
            def _done():
                logs = curl_json([f"{self.api_base()}/tenant/plugins/{plugin_id}/logs?page=1&page_size=20", *view_headers])
                items = self.list_items(logs)
                if items and items[0].get("completed_at"):
                    return items[0]
                return None

            return self.wait_until(_done, timeout=30, interval=1, description=f"plugin {plugin_id} sync")

        itsm_log = wait_sync(itsm_plugin["id"])
        assert_eq(itsm_log["status"], "success", "itsm plugin sync should succeed")

        cmdb_plugin = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/plugins",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-plugin-cmdb",
                        "type": "cmdb",
                        "description": "acceptance cmdb plugin",
                        "config": {
                            "url": f"http://127.0.0.1:{self.cmdb_port}/api/now/table/cmdb_ci",
                            "auth_type": "none",
                            "response_data_path": "result",
                        },
                        "field_mapping": {
                            "cmdb_mapping": {
                                "external_id": "sys_id",
                                "name": "name",
                                "type": "category",
                                "status": "status",
                                "ip_address": "ip_address",
                                "hostname": "hostname",
                                "os": "os",
                                "os_version": "os_version",
                                "cpu": "cpu",
                                "memory": "memory",
                                "disk": "disk",
                                "location": "location",
                                "owner": "owner",
                                "environment": "environment",
                                "department": "department",
                            }
                        },
                        "sync_enabled": False,
                    }
                ),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{cmdb_plugin['id']}/test", *view_headers])
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{cmdb_plugin['id']}/activate", *view_headers])
        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/plugins/{cmdb_plugin['id']}",
                *headers,
                "-d",
                json.dumps({"description": "acceptance cmdb plugin updated"}),
            ]
        )
        cmdb_detail = curl_json([f"{self.api_base()}/tenant/plugins/{cmdb_plugin['id']}", *view_headers])
        assert_eq(cmdb_detail["data"]["status"], "active", "cmdb plugin should activate")
        assert_eq(cmdb_detail["data"]["description"], "acceptance cmdb plugin updated", "cmdb plugin should update")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{cmdb_plugin['id']}/sync", *view_headers])
        cmdb_log = wait_sync(cmdb_plugin["id"])
        assert_eq(cmdb_log["status"], "success", "cmdb plugin sync should succeed")

        plugins = curl_json([f"{self.api_base()}/tenant/plugins?page=1&page_size=50", *view_headers])
        plugin_items = self.list_items(plugins)
        plugin_names = {item["name"] for item in plugin_items}
        assert_true("acc-plugin-itsm" in plugin_names, "itsm plugin should be listed")
        assert_true("acc-plugin-cmdb" in plugin_names, "cmdb plugin should be listed")
        plugin_stats = curl_json([f"{self.api_base()}/tenant/plugins/stats", *view_headers])
        assert_true(plugin_stats["data"]["total"] >= 2, "plugin stats should include new plugins")

        incidents = curl_json([f"{self.api_base()}/tenant/incidents?plugin_id={itsm_plugin['id']}&page=1&page_size=20", *view_headers])
        incident_items = self.list_items(incidents)
        assert_true(len(incident_items) >= 1, "incident list should contain synced incidents")
        incident = incident_items[0]
        incident_detail = curl_json([f"{self.api_base()}/tenant/incidents/{incident['id']}", *view_headers])
        incident_stats = curl_json([f"{self.api_base()}/tenant/incidents/stats", *view_headers])
        assert_eq(incident_detail["data"]["id"], incident["id"], "incident detail id")
        assert_true(isinstance(incident_stats["data"], dict), "incident stats payload")
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/incidents/{incident['id']}/reset-scan",
                *view_headers,
            ]
        )
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/incidents/batch-reset-scan",
                *headers,
                "-d",
                json.dumps({"ids": [incident["id"]]}),
            ]
        )

        cmdb_items = curl_json([f"{self.api_base()}/tenant/cmdb?plugin_id={cmdb_plugin['id']}&page=1&page_size=10", *view_headers])
        cmdb_list = self.list_items(cmdb_items)
        assert_true(len(cmdb_list) >= 2, "cmdb list should contain synced items")
        cmdb_ids_payload = curl_json([f"{self.api_base()}/tenant/cmdb/ids?plugin_id={cmdb_plugin['id']}", *view_headers])
        cmdb_id_items = cmdb_ids_payload["data"]["items"]
        assert_true(len(cmdb_id_items) >= 2, "cmdb ids should be queryable")
        cmdb_item = cmdb_list[0]
        cmdb_detail = curl_json([f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}", *view_headers])
        cmdb_stats = curl_json([f"{self.api_base()}/tenant/cmdb/stats", *view_headers])
        assert_eq(cmdb_detail["data"]["id"], cmdb_item["id"], "cmdb detail id")
        assert_true(isinstance(cmdb_stats["data"], dict), "cmdb stats payload")
        single_connection = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}/test-connection",
                *headers,
                "-d",
                json.dumps({"secrets_source_id": self.default_secret_source["id"]}),
            ]
        )["data"]
        assert_true("success" in single_connection, "single cmdb connection test should return success flag")
        assert_true("message" in single_connection, "single cmdb connection test should return message")
        batch_connection = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/cmdb/batch-test-connection",
                *headers,
                "-d",
                json.dumps({"cmdb_ids": [item["id"] for item in cmdb_list[:2]], "secrets_source_id": self.default_secret_source["id"]}),
            ]
        )["data"]
        assert_eq(batch_connection["total"], 2, "batch cmdb connection test total")
        assert_eq(len(batch_connection["results"]), 2, "batch cmdb connection test should return two results")
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}/maintenance",
                *headers,
                "-d",
                json.dumps({"reason": "acceptance maintenance"}),
            ]
        )
        maintained = curl_json([f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}", *view_headers])
        assert_eq(maintained["data"]["status"], "maintenance", "cmdb item should enter maintenance")
        maintenance_logs = curl_json([f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}/maintenance-logs?page=1&page_size=20", *view_headers])
        assert_true(len(maintenance_logs["data"]["data"]) >= 1, "cmdb maintenance logs should exist")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}/resume", *view_headers])
        resumed = curl_json([f"{self.api_base()}/tenant/cmdb/{cmdb_item['id']}", *view_headers])
        assert_true(resumed["data"]["status"] != "maintenance", "cmdb item should resume from maintenance")

        batch_ids = [item["id"] for item in cmdb_list[:2]]
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/cmdb/batch/maintenance",
                *headers,
                "-d",
                json.dumps({"ids": batch_ids, "reason": "batch maintenance"}),
            ]
        )
        curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/cmdb/batch/resume",
                *headers,
                "-d",
                json.dumps({"ids": batch_ids}),
            ]
        )
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/plugins/{itsm_plugin['id']}/deactivate", *view_headers])
        deactivated = curl_json([f"{self.api_base()}/tenant/plugins/{itsm_plugin['id']}", *view_headers])
        assert_eq(deactivated["data"]["status"], "inactive", "plugin should deactivate")

        self.plugin_itsm = itsm_plugin
        self.plugin_cmdb = cmdb_plugin
        self.sample_incident = incident_detail["data"]
        self.sample_cmdb_item = resumed["data"]
        self.results["plugin_cmdb"] = {"status": "passed"}

    def run_execution_queries(self):
        info("==> execution / schedule query surfaces")
        token = self.tenant_admin_token
        view_headers = self.auth_args(token)
        headers = self.auth_args(token, json_content=True)

        self.ensure_executor_image()
        docker_task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-docker",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "docker",
                        "description": "docker execution acceptance",
                    }
                ),
            ]
        )["data"]
        docker_run = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{docker_task['id']}/execute",
                *headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        docker_final = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{docker_run['id']}", *view_headers])),
            timeout=60,
            interval=2,
            description="docker execution run",
        )
        assert_eq(docker_final["status"], "success", "docker execution should succeed")

        task_schema = curl_json([f"{self.api_base()}/tenant/execution-tasks/search-schema", *view_headers])
        run_schema = curl_json([f"{self.api_base()}/tenant/execution-runs/search-schema", *view_headers])
        assert_true(len(task_schema["data"]["fields"]) >= 1, "task search schema should return fields")
        assert_true(len(run_schema["data"]["fields"]) >= 1, "run search schema should return fields")

        tasks = curl_json([f"{self.api_base()}/tenant/execution-tasks?has_runs=true&page=1&page_size=50", *view_headers])
        task_items = self.list_items(tasks)
        task_ids = {item["id"] for item in task_items}
        assert_true(self.local_task["id"] in task_ids, "local task should be listed with runs")
        task_detail = curl_json([f"{self.api_base()}/tenant/execution-tasks/{self.local_task['id']}", *view_headers])
        task_runs = curl_json([f"{self.api_base()}/tenant/execution-tasks/{self.local_task['id']}/runs?page=1&page_size=20", *view_headers])
        task_stats = curl_json([f"{self.api_base()}/tenant/execution-tasks/stats", *view_headers])
        assert_eq(task_detail["data"]["id"], self.local_task["id"], "task detail id")
        assert_true(any(item["id"] == self.local_run["id"] for item in self.list_items(task_runs)), "task runs should include local run")
        assert_true(isinstance(task_stats["data"], dict), "task stats payload")

        runs = curl_json([f"{self.api_base()}/tenant/execution-runs?task_id={self.local_task['id']}&page=1&page_size=50", *view_headers])
        run_items = self.list_items(runs)
        assert_true(any(item["id"] == self.local_run["id"] for item in run_items), "run list should include local run")
        run_detail = curl_json([f"{self.api_base()}/tenant/execution-runs/{self.local_run['id']}", *view_headers])
        run_logs = curl_json([f"{self.api_base()}/tenant/execution-runs/{self.local_run['id']}/logs", *view_headers])
        run_stats = curl_json([f"{self.api_base()}/tenant/execution-runs/stats", *view_headers])
        run_trend = curl_json([f"{self.api_base()}/tenant/execution-runs/trend?days=7", *view_headers])
        trigger_distribution = curl_json([f"{self.api_base()}/tenant/execution-runs/trigger-distribution", *view_headers])
        top_failed = curl_json([f"{self.api_base()}/tenant/execution-runs/top-failed?limit=5", *view_headers])
        top_active = curl_json([f"{self.api_base()}/tenant/execution-runs/top-active?limit=5", *view_headers])
        assert_eq(run_detail["data"]["id"], self.local_run["id"], "run detail id")
        assert_true(isinstance(run_logs["data"], list), "run logs should be a list")
        assert_true(isinstance(run_stats["data"], dict), "run stats payload")
        assert_eq(run_trend["data"]["days"], 7, "run trend days")
        assert_true(isinstance(trigger_distribution["data"], list), "trigger distribution payload")
        assert_true(top_failed["data"] is None or isinstance(top_failed["data"], list), "top failed payload")
        assert_true(top_active["data"] is None or isinstance(top_active["data"], list), "top active payload")

        cron_schedule = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-schedules",
                *headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-cron-query",
                        "task_id": self.local_task["id"],
                        "schedule_type": "cron",
                        "schedule_expr": "*/15 * * * *",
                        "description": "cron query schedule",
                        "enabled": False,
                    }
                ),
            ]
        )["data"]
        schedules = curl_json([f"{self.api_base()}/tenant/execution-schedules?task_id={self.local_task['id']}", *view_headers])
        schedule_items = self.list_items(schedules)
        assert_true(any(item["id"] == cron_schedule["id"] for item in schedule_items), "cron schedule should be listed")
        schedule_detail = curl_json([f"{self.api_base()}/tenant/execution-schedules/{cron_schedule['id']}", *view_headers])
        schedule_stats = curl_json([f"{self.api_base()}/tenant/execution-schedules/stats", *view_headers])
        today = time.strftime("%Y-%m-%d", time.localtime())
        schedule_timeline = curl_json([f"{self.api_base()}/tenant/execution-schedules/timeline?date={today}", *view_headers])
        assert_eq(schedule_detail["data"]["id"], cron_schedule["id"], "schedule detail id")
        assert_true(isinstance(schedule_stats["data"], dict), "schedule stats payload")
        assert_true(isinstance(schedule_timeline["data"], list), "schedule timeline payload")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/execution-schedules/{cron_schedule['id']}/enable", *view_headers])
        enabled_schedule = curl_json([f"{self.api_base()}/tenant/execution-schedules/{cron_schedule['id']}", *view_headers])
        assert_eq(enabled_schedule["data"]["enabled"], True, "schedule should enable")
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/execution-schedules/{cron_schedule['id']}/disable", *view_headers])
        disabled_schedule = curl_json([f"{self.api_base()}/tenant/execution-schedules/{cron_schedule['id']}", *view_headers])
        assert_eq(disabled_schedule["data"]["enabled"], False, "schedule should disable")

        self.docker_task = docker_task
        self.docker_run = docker_final
        self.query_schedule = cron_schedule
        self.results["execution_queries"] = {"status": "passed"}

    def run_notifications_audit(self):
        info("==> notifications / audit")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)
        platform_view_headers = self.auth_args(self.platform_token)

        initial_hits = len(self.webhook_hits())
        channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-channel",
                        "type": "webhook",
                        "description": "acceptance webhook channel",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/notify",
                            "method": "POST",
                            "timeout_seconds": 5,
                        },
                        "recipients": ["ops@example.com"],
                        "is_default": True,
                    }
                ),
            ]
        )["data"]
        channels = curl_json([f"{self.api_base()}/tenant/channels?type=webhook&page=1&page_size=20", *tenant_view_headers])
        assert_true(any(item["id"] == channel["id"] for item in self.list_items(channels)), "notification channel should be listed")
        channel_detail = curl_json([f"{self.api_base()}/tenant/channels/{channel['id']}", *tenant_view_headers])["data"]
        assert_eq(channel_detail["id"], channel["id"], "channel detail id")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/channels/{channel['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"name": "acc-webhook-channel-updated", "description": "updated channel"}),
            ]
        )
        updated_channel = curl_json([f"{self.api_base()}/tenant/channels/{channel['id']}", *tenant_view_headers])["data"]
        assert_eq(updated_channel["name"], "acc-webhook-channel-updated", "channel name should update")

        test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/channels/{channel['id']}/test", *tenant_view_headers])
        assert_eq(test_status, 200, "channel test should pass")
        self.wait_until(lambda: len(self.webhook_hits()) > initial_hits, timeout=10, interval=1, description="channel test webhook hit")

        dingtalk_hits_before = len(self.provider_hits("/dingtalk"))
        dingtalk_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-dingtalk-channel",
                        "type": "dingtalk",
                        "description": "acceptance dingtalk channel",
                        "config": {
                            "webhook_url": f"http://127.0.0.1:{self.aux_port}/dingtalk",
                            "secret": "acceptance-dingtalk-secret",
                            "at_mobiles": ["13800138000"],
                            "at_all": False,
                        },
                        "recipients": [],
                    }
                ),
            ]
        )["data"]
        dingtalk_test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/channels/{dingtalk_channel['id']}/test", *tenant_view_headers])
        assert_eq(dingtalk_test_status, 200, "dingtalk channel test should pass")
        self.wait_until(
            lambda: len(self.provider_hits("/dingtalk")) > dingtalk_hits_before,
            timeout=10,
            interval=1,
            description="dingtalk test hit",
        )

        template = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/templates",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-template-webhook",
                        "description": "acceptance template",
                        "event_type": "acceptance.event",
                        "supported_channels": ["webhook"],
                        "subject_template": "Execution {{task.name}} {{execution.status}}",
                        "body_template": "Body for {{task.name}} on {{date}}",
                        "format": "markdown",
                    }
                ),
            ]
        )["data"]
        templates = curl_json([f"{self.api_base()}/tenant/templates?supported_channel=webhook&page=1&page_size=20", *tenant_view_headers])
        assert_true(any(item["id"] == template["id"] for item in self.list_items(templates)), "template should be listed")
        template_detail = curl_json([f"{self.api_base()}/tenant/templates/{template['id']}", *tenant_view_headers])["data"]
        assert_eq(template_detail["id"], template["id"], "template detail id")

        preview = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/templates/{template['id']}/preview",
                *tenant_headers,
                "-d",
                json.dumps({"variables": {"task": {"name": "acceptance-task"}, "execution": {"status": "success"}, "date": "2026-03-22"}}),
            ]
        )["data"]
        assert_eq(preview["subject"], "Execution acceptance-task success", "template preview subject")

        template_vars = curl_json([f"{self.api_base()}/tenant/template-variables", *tenant_view_headers])["data"]["variables"]
        assert_true(len(template_vars) >= 1, "template variables catalog should be non-empty")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/templates/{template['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"body_template": "Updated body for {{task.name}}", "is_active": True}),
            ]
        )
        updated_template = curl_json([f"{self.api_base()}/tenant/templates/{template['id']}", *tenant_view_headers])["data"]
        assert_eq(updated_template["body_template"], "Updated body for {{task.name}}", "template body should update")

        hits_before_send = len(self.webhook_hits())
        sent = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "template_id": template["id"],
                        "channel_ids": [channel["id"]],
                        "variables": {
                            "task": {"name": "acceptance-task"},
                            "execution": {"status": "success"},
                            "date": "2026-03-22",
                        },
                    }
                ),
            ]
        )["data"]
        assert_eq(len(sent["notification_ids"]), 1, "notification send should create one log")
        self.wait_until(lambda: len(self.webhook_hits()) > hits_before_send, timeout=10, interval=1, description="notification webhook hit")
        last_hit = self.webhook_hits()[-1]["body"]
        assert_eq(last_hit["subject"], "Execution acceptance-task success", "webhook subject should render from template")
        assert_true("acceptance-task" in last_hit["body"], "webhook body should render from template")

        dingtalk_hits_before_send = len(self.provider_hits("/dingtalk"))
        dingtalk_sent = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "channel_ids": [dingtalk_channel["id"]],
                        "subject": "Acceptance DingTalk",
                        "body": "DingTalk body",
                        "format": "markdown",
                    }
                ),
            ]
        )["data"]
        assert_eq(len(dingtalk_sent["notification_ids"]), 1, "dingtalk send should create one log")
        self.wait_until(
            lambda: len(self.provider_hits("/dingtalk")) > dingtalk_hits_before_send,
            timeout=10,
            interval=1,
            description="dingtalk send hit",
        )
        last_dingtalk_hit = self.provider_hits("/dingtalk")[-1]["body"]
        assert_eq(last_dingtalk_hit["msgtype"], "markdown", "dingtalk payload should be markdown")
        assert_eq(last_dingtalk_hit["markdown"]["title"], "Acceptance DingTalk", "dingtalk title should match")
        assert_true("DingTalk body" in last_dingtalk_hit["markdown"]["text"], "dingtalk body should match")

        notifications = curl_json(
            [
                f"{self.api_base()}/tenant/notifications?channel_id={channel['id']}&template_id={template['id']}&status=sent&page=1&page_size=20",
                *tenant_view_headers,
            ]
        )
        notification_items = self.list_items(notifications)
        assert_true(any(item["id"] == sent["notification_ids"][0] for item in notification_items), "notification log should be listed")
        notification_detail = curl_json([f"{self.api_base()}/tenant/notifications/{sent['notification_ids'][0]}", *tenant_view_headers])["data"]
        notification_stats = curl_json([f"{self.api_base()}/tenant/notifications/stats", *tenant_view_headers])["data"]
        assert_eq(notification_detail["status"], "sent", "notification detail status")
        assert_true(notification_stats["logs_total"] >= 1, "notification stats should be non-zero")
        dingtalk_notifications = curl_json(
            [f"{self.api_base()}/tenant/notifications?channel_id={dingtalk_channel['id']}&status=sent&page=1&page_size=20", *tenant_view_headers]
        )
        assert_true(any(item["id"] == dingtalk_sent["notification_ids"][0] for item in self.list_items(dingtalk_notifications)), "dingtalk notification log should be listed")

        smtp_hits_before = len(self.smtp_hits())
        email_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-email-channel",
                        "type": "email",
                        "description": "acceptance email channel",
                        "config": {
                            "smtp_host": "127.0.0.1",
                            "smtp_port": self.smtp_port,
                            "username": "acceptance",
                            "password": "acceptance",
                            "from_address": "noreply@acceptance.local",
                            "use_tls": False,
                        },
                        "recipients": ["ops@example.com"],
                    }
                ),
            ]
        )["data"]
        email_test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/channels/{email_channel['id']}/test", *tenant_view_headers])
        assert_eq(email_test_status, 200, "email channel test should pass")

        email_sent = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "channel_ids": [email_channel["id"]],
                        "subject": "Acceptance Email",
                        "body": "<b>Email body</b>",
                        "format": "html",
                    }
                ),
            ]
        )["data"]
        assert_eq(len(email_sent["notification_ids"]), 1, "email send should create one log")
        self.wait_until(
            lambda: len(self.smtp_hits()) > smtp_hits_before,
            timeout=10,
            interval=1,
            description="email smtp hit",
        )
        last_email = self.smtp_hits()[-1]["data"]
        assert_true("Subject: Acceptance Email" in last_email, "email subject should be present")
        assert_true("Content-Type: text/html; charset=UTF-8" in last_email, "email content type should be html")
        assert_true("<b>Email body</b>" in last_email, "email body should be present")
        email_notifications = curl_json(
            [f"{self.api_base()}/tenant/notifications?channel_id={email_channel['id']}&status=sent&page=1&page_size=20", *tenant_view_headers]
        )
        assert_true(any(item["id"] == email_sent["notification_ids"][0] for item in self.list_items(email_notifications)), "email notification log should be listed")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/templates/{template['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{channel['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{dingtalk_channel['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{email_channel['id']}", *tenant_view_headers])
        reset_status, _ = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/users/{self.tenant_viewer['id']}/reset-password",
                *self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"], json_content=True),
                "-d",
                json.dumps({"new_password": "TenantReset123!"}),
            ]
        )
        assert_eq(reset_status, 200, "tenant reset-password should succeed")
        time.sleep(1)

        tenant_audit_list = curl_json([f"{self.api_base()}/tenant/audit-logs?page=1&page_size=20&sort_by=created_at&sort_order=desc", *tenant_view_headers])
        tenant_audit_items = self.list_items(tenant_audit_list)
        assert_true(len(tenant_audit_items) >= 1, "tenant audit logs should be non-empty")
        tenant_audit_detail = curl_json([f"{self.api_base()}/tenant/audit-logs/{tenant_audit_items[0]['id']}", *tenant_view_headers])["data"]
        tenant_audit_stats = curl_json([f"{self.api_base()}/tenant/audit-logs/stats", *tenant_view_headers])["data"]
        tenant_audit_ranking = curl_json([f"{self.api_base()}/tenant/audit-logs/user-ranking?limit=10&days=7", *tenant_view_headers])["data"]
        tenant_audit_grouping = curl_json([f"{self.api_base()}/tenant/audit-logs/action-grouping?action=create&days=30", *tenant_view_headers])["data"]
        tenant_audit_resource = curl_json([f"{self.api_base()}/tenant/audit-logs/resource-stats?days=30", *tenant_view_headers])["data"]
        tenant_audit_trend = curl_json([f"{self.api_base()}/tenant/audit-logs/trend?days=30", *tenant_view_headers])["data"]
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])
        tenant_reset_logs = curl_json([f"{self.api_base()}/tenant/audit-logs?action=reset_password&page=1&page_size=20", *tenant_b_view_headers])
        tenant_audit_high = curl_json([f"{self.api_base()}/tenant/audit-logs/high-risk?page=1&page_size=20", *tenant_view_headers])
        export_status, export_body = curl_status_text([f"{self.api_base()}/tenant/audit-logs/export", *tenant_view_headers])
        assert_eq(tenant_audit_detail["id"], tenant_audit_items[0]["id"], "tenant audit detail id")
        assert_true(tenant_audit_stats["total_count"] >= 1, "tenant audit stats should be non-zero")
        assert_true(len(tenant_audit_ranking["rankings"]) >= 1, "tenant audit ranking should be non-empty")
        assert_true("items" in tenant_audit_grouping, "tenant audit grouping payload")
        assert_true("items" in tenant_audit_resource, "tenant audit resource stats payload")
        assert_eq(tenant_audit_trend["days"], 30, "tenant audit trend days")
        assert_true(tenant_reset_logs["data"] is None or isinstance(tenant_reset_logs["data"], list), "tenant reset-password audit payload")
        assert_true(tenant_audit_high["data"] is None or isinstance(tenant_audit_high["data"], list), "tenant high-risk payload")
        assert_eq(export_status, 200, "tenant audit export status")
        assert_true("时间,用户,操作" in export_body, "tenant audit export should contain CSV header")

        platform_audit_list = curl_json([f"{self.api_base()}/platform/audit-logs?page=1&page_size=20&sort_by=created_at&sort_order=desc", *platform_view_headers])
        platform_audit_items = self.list_items(platform_audit_list)
        assert_true(len(platform_audit_items) >= 1, "platform audit logs should be non-empty")
        platform_audit_detail = curl_json([f"{self.api_base()}/platform/audit-logs/{platform_audit_items[0]['id']}", *platform_view_headers])["data"]
        platform_audit_stats = curl_json([f"{self.api_base()}/platform/audit-logs/stats", *platform_view_headers])["data"]
        platform_audit_trend = curl_json([f"{self.api_base()}/platform/audit-logs/trend?days=7", *platform_view_headers])["data"]
        platform_audit_ranking = curl_json([f"{self.api_base()}/platform/audit-logs/user-ranking?limit=10&days=7", *platform_view_headers])["data"]
        platform_audit_high = curl_json([f"{self.api_base()}/platform/audit-logs/high-risk?page=1&page_size=20", *platform_view_headers])
        assert_eq(platform_audit_detail["id"], platform_audit_items[0]["id"], "platform audit detail id")
        assert_true(platform_audit_stats["total_count"] >= 1, "platform audit stats should be non-zero")
        assert_eq(platform_audit_trend["days"], 7, "platform audit trend days")
        assert_true(len(platform_audit_ranking["rankings"]) >= 1, "platform audit ranking should be non-empty")
        assert_true(platform_audit_high["data"] is None or isinstance(platform_audit_high["data"], list), "platform high-risk payload")

        self.results["notifications_audit"] = {"status": "passed"}

    def run_notification_failures(self):
        info("==> notification provider failures")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        failing_webhook = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-fail",
                        "type": "webhook",
                        "description": "failing webhook channel",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify-fail", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                    }
                ),
            ]
        )["data"]
        webhook_test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/channels/{failing_webhook['id']}/test", *tenant_view_headers])
        assert_eq(webhook_test_status, 400, "failing webhook test should fail")
        webhook_send = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [failing_webhook["id"]], "subject": "Webhook Fail", "body": "fail body", "format": "text"}),
            ]
        )["data"]
        webhook_fail_log = curl_json([f"{self.api_base()}/tenant/notifications/{webhook_send['notification_ids'][0]}", *tenant_view_headers])["data"]
        assert_eq(webhook_fail_log["status"], "failed", "failing webhook send should record failed log")

        failing_dingtalk = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-dingtalk-fail",
                        "type": "dingtalk",
                        "description": "failing dingtalk channel",
                        "config": {"webhook_url": f"http://127.0.0.1:{self.aux_port}/dingtalk-fail", "secret": "bad-secret"},
                        "recipients": [],
                    }
                ),
            ]
        )["data"]
        dingtalk_test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/channels/{failing_dingtalk['id']}/test", *tenant_view_headers])
        assert_eq(dingtalk_test_status, 400, "failing dingtalk test should fail")
        dingtalk_send = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [failing_dingtalk["id"]], "subject": "DingTalk Fail", "body": "fail body", "format": "markdown"}),
            ]
        )["data"]
        dingtalk_fail_log = curl_json([f"{self.api_base()}/tenant/notifications/{dingtalk_send['notification_ids'][0]}", *tenant_view_headers])["data"]
        assert_eq(dingtalk_fail_log["status"], "failed", "failing dingtalk send should record failed log")

        failing_email = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-email-fail",
                        "type": "email",
                        "description": "failing email channel",
                        "config": {
                            "smtp_host": "127.0.0.1",
                            "smtp_port": self.smtp_port + 1,
                            "username": "acceptance",
                            "password": "acceptance",
                            "from_address": "noreply@acceptance.local",
                            "use_tls": False,
                        },
                        "recipients": ["ops@example.com"],
                    }
                ),
            ]
        )["data"]
        email_test_status, _ = curl_status_json(["-X", "POST", f"{self.api_base()}/tenant/channels/{failing_email['id']}/test", *tenant_view_headers])
        assert_eq(email_test_status, 400, "failing email test should fail")
        email_send = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [failing_email["id"]], "subject": "Email Fail", "body": "fail body", "format": "text"}),
            ]
        )["data"]
        email_fail_log = curl_json([f"{self.api_base()}/tenant/notifications/{email_send['notification_ids'][0]}", *tenant_view_headers])["data"]
        assert_eq(email_fail_log["status"], "failed", "failing email send should record failed log")

        failed_notifications = curl_json([f"{self.api_base()}/tenant/notifications?status=failed&page=1&page_size=50", *tenant_view_headers])
        failed_ids = {item["id"] for item in self.list_items(failed_notifications)}
        assert_true(webhook_send["notification_ids"][0] in failed_ids, "webhook failed log should appear in failed filter")
        assert_true(dingtalk_send["notification_ids"][0] in failed_ids, "dingtalk failed log should appear in failed filter")
        assert_true(email_send["notification_ids"][0] in failed_ids, "email failed log should appear in failed filter")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{failing_webhook['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{failing_dingtalk['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{failing_email['id']}", *tenant_view_headers])

        self.results["notification_failures"] = {"status": "passed"}

    def run_notification_retry(self):
        info("==> notification retry")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        retry_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-retry",
                        "type": "webhook",
                        "description": "retry webhook channel",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify-fail", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                        "retry_config": {"max_retries": 2, "retry_intervals": [0, 0]},
                    }
                ),
            ]
        )["data"]

        first_send = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [retry_channel["id"]], "subject": "Retry Me", "body": "retry body", "format": "text"}),
            ]
        )["data"]
        first_log_id = first_send["notification_ids"][0]
        first_log = curl_json([f"{self.api_base()}/tenant/notifications/{first_log_id}", *tenant_view_headers])["data"]
        assert_eq(first_log["status"], "failed", "first retry log should start as failed")
        assert_true(first_log.get("next_retry_at") is not None, "failed retry log should have next_retry_at")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/channels/{retry_channel['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"config": {"url": f"http://127.0.0.1:{self.aux_port}/notify", "method": "POST", "timeout_seconds": 5}}),
            ]
        )

        retried_log = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] == "sent" and payload["data"]["retry_count"] >= 1
                else None
            )(curl_json([f"{self.api_base()}/tenant/notifications/{first_log_id}", *tenant_view_headers])),
            timeout=15,
            interval=1,
            description="notification retry success",
        )
        assert_eq(retried_log["status"], "sent", "retried notification should become sent")
        assert_true(retried_log["retry_count"] >= 1, "retried notification should increment retry_count")
        assert_true(retried_log.get("sent_at") is not None, "retried notification should have sent_at")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{retry_channel['id']}", *tenant_view_headers])

        self.results["notification_retry"] = {"status": "passed"}

    def run_notification_retry_tenant_scope(self):
        info("==> notification retry tenant scope")
        tenant_b_view_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"])
        tenant_b_headers = self.auth_args(self.tenant_admin_token, tenant_id=self.tenant_b["id"], json_content=True)

        retry_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_b_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-retry-tenant-b",
                        "type": "webhook",
                        "description": "retry webhook channel tenant b",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify-fail", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                        "retry_config": {"max_retries": 2, "retry_intervals": [0, 0]},
                    }
                ),
            ]
        )["data"]

        sent = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_b_headers,
                "-d",
                json.dumps({"channel_ids": [retry_channel["id"]], "subject": "Retry Tenant B", "body": "retry tenant body", "format": "text"}),
            ]
        )["data"]
        log_id = sent["notification_ids"][0]
        first_log = curl_json([f"{self.api_base()}/tenant/notifications/{log_id}", *tenant_b_view_headers])["data"]
        assert_eq(first_log["status"], "failed", "tenant-b retry log should start as failed")

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/channels/{retry_channel['id']}",
                *tenant_b_headers,
                "-d",
                json.dumps({"config": {"url": f"http://127.0.0.1:{self.aux_port}/notify", "method": "POST", "timeout_seconds": 5}}),
            ]
        )

        retried_log = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] == "sent" and payload["data"]["retry_count"] >= 1
                else None
            )(curl_json([f"{self.api_base()}/tenant/notifications/{log_id}", *tenant_b_view_headers])),
            timeout=15,
            interval=1,
            description="tenant-b notification retry success",
        )
        assert_eq(retried_log["status"], "sent", "tenant-b retried notification should become sent")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{retry_channel['id']}", *tenant_b_view_headers])

        self.results["notification_retry_tenant_scope"] = {"status": "passed"}

    def run_notification_retry_exhaustion(self):
        info("==> notification retry exhaustion")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        retry_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-retry-exhaust",
                        "type": "webhook",
                        "description": "retry exhaustion webhook channel",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify-fail", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                        "retry_config": {"max_retries": 1, "retry_intervals": [0]},
                    }
                ),
            ]
        )["data"]

        sent = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [retry_channel["id"]], "subject": "Retry Exhaust", "body": "still fail", "format": "text"}),
            ]
        )["data"]
        log_id = sent["notification_ids"][0]
        initial_log = curl_json([f"{self.api_base()}/tenant/notifications/{log_id}", *tenant_view_headers])["data"]
        assert_eq(initial_log["status"], "failed", "retry exhaustion log should start as failed")
        assert_true(initial_log.get("next_retry_at") is not None, "retry exhaustion log should schedule a retry")

        exhausted_log = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] == "failed" and payload["data"]["retry_count"] >= 1 and payload["data"].get("next_retry_at") is None
                else None
            )(curl_json([f"{self.api_base()}/tenant/notifications/{log_id}", *tenant_view_headers])),
            timeout=15,
            interval=1,
            description="notification retry exhaustion",
        )
        assert_eq(exhausted_log["retry_count"], 1, "retry exhaustion should stop after one retry")

        time.sleep(4)
        stable_log = curl_json([f"{self.api_base()}/tenant/notifications/{log_id}", *tenant_view_headers])["data"]
        assert_eq(stable_log["retry_count"], 1, "retry exhaustion should not continue retrying after max_retries")
        assert_eq(stable_log["status"], "failed", "retry exhaustion final status should stay failed")
        assert_true(stable_log.get("next_retry_at") is None, "retry exhaustion should clear next_retry_at")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{retry_channel['id']}", *tenant_view_headers])

        self.results["notification_retry_exhaustion"] = {"status": "passed"}

    def run_notification_rate_limit(self):
        info("==> notification rate limit")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        limited_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-limited",
                        "type": "webhook",
                        "description": "rate limited webhook channel",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                        "rate_limit_per_minute": 1,
                    }
                ),
            ]
        )["data"]

        first_send = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [limited_channel["id"]], "subject": "Rate Limit First", "body": "first", "format": "text"}),
            ]
        )["data"]
        second_send = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/notifications/send",
                *tenant_headers,
                "-d",
                json.dumps({"channel_ids": [limited_channel["id"]], "subject": "Rate Limit Second", "body": "second", "format": "text"}),
            ]
        )["data"]

        first_log = curl_json([f"{self.api_base()}/tenant/notifications/{first_send['notification_ids'][0]}", *tenant_view_headers])["data"]
        second_log = curl_json([f"{self.api_base()}/tenant/notifications/{second_send['notification_ids'][0]}", *tenant_view_headers])["data"]
        assert_eq(first_log["status"], "sent", "first rate-limited notification should send")
        assert_eq(second_log["status"], "failed", "second rate-limited notification should fail")
        assert_true("速率限制" in (second_log.get("error_message") or ""), "rate limit failure message should be recorded")

        failed_notifications = curl_json(
            [f"{self.api_base()}/tenant/notifications?channel_id={limited_channel['id']}&status=failed&page=1&page_size=20", *tenant_view_headers]
        )
        assert_true(any(item["id"] == second_log["id"] for item in self.list_items(failed_notifications)), "rate-limited failed log should be queryable")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{limited_channel['id']}", *tenant_view_headers])

        self.results["notification_rate_limit"] = {"status": "passed"}

    def run_secrets_default_fallback(self):
        info("==> secrets default fallback")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        source_primary = self.default_secret_source
        source_c = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-c",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                        "priority": 9,
                    }
                ),
            ]
        )["data"]

        curl_json(
            [
                "-X",
                "PUT",
                f"{self.api_base()}/tenant/secrets-sources/{source_primary['id']}",
                *tenant_headers,
                "-d",
                json.dumps({"is_default": False}),
            ]
        )
        defaults_after_demote = curl_json([f"{self.api_base()}/tenant/secrets-sources?is_default=true", *tenant_view_headers])["data"]
        assert_eq(len(defaults_after_demote), 0, "there should be no explicit default after demotion")

        query_primary_fallback = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets/query",
                *tenant_headers,
                "-d",
                json.dumps({"hostname": "host-a"}),
            ]
        )["data"]
        assert_eq(query_primary_fallback["username"], "ops-a", "highest-priority active source should be used as fallback")

        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source_primary['id']}/disable", *tenant_view_headers])
        query_secondary_fallback = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets/query",
                *tenant_headers,
                "-d",
                json.dumps({"hostname": "host-b"}),
            ]
        )["data"]
        assert_eq(query_secondary_fallback["username"], "ops-b", "next active source should be used after primary disable")

        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source_c['id']}/disable", *tenant_view_headers])
        no_active_status, no_active_body = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets/query",
                *tenant_headers,
                "-d",
                json.dumps({"hostname": "host-a"}),
            ]
        )
        assert_eq(no_active_status, 409, "querying without active sources should fail")
        assert_true("默认密钥源" in no_active_body["message"] or "密钥未找到" in no_active_body["message"], "no active source error message")

        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source_primary['id']}/enable", *tenant_view_headers])
        delete_c_status, _ = curl_status_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source_c['id']}", *tenant_view_headers])
        assert_eq(delete_c_status, 200, "secondary source cleanup should succeed")

        self.results["secrets_default_fallback"] = {"status": "passed"}

    def run_secrets_disabled_usage(self):
        info("==> secrets disabled usage")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        source = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-disabled-usage",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                        "priority": 12,
                    }
                ),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{source['id']}/disable", *tenant_view_headers])

        disabled_query_status, disabled_query_body = curl_status_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets/query",
                *tenant_headers,
                "-d",
                json.dumps({"source_id": source["id"], "hostname": "host-a"}),
            ]
        )
        assert_eq(disabled_query_status, 409, "explicit disabled source query should conflict")
        assert_true("未启用" in disabled_query_body["message"], "disabled source query message")

        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-disabled-secret",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "local",
                        "description": "task using disabled secret source",
                        "secrets_source_ids": [source["id"]],
                    }
                ),
            ]
        )["data"]
        run_resp = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]
        final_run = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{run_resp['id']}", *tenant_view_headers])),
            timeout=20,
            interval=1,
            description="disabled secret execution result",
        )
        assert_eq(final_run["status"], "failed", "execution using only disabled secret should fail")
        logs = curl_json([f"{self.api_base()}/tenant/execution-runs/{run_resp['id']}/logs", *tenant_view_headers])["data"]
        log_messages = " ".join(item.get("message", "") for item in logs)
        assert_true("没有可用的密钥源" in log_messages, "disabled secret execution should log no available secrets")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{task['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{source['id']}", *tenant_view_headers])

        self.results["secrets_disabled_usage"] = {"status": "passed"}

    def run_secrets_runtime_override(self):
        info("==> secrets runtime override")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        disabled_source = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-runtime-disabled",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                    }
                ),
            ]
        )["data"]
        curl_json(["-X", "POST", f"{self.api_base()}/tenant/secrets-sources/{disabled_source['id']}/disable", *tenant_view_headers])

        active_source = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/secrets-sources",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-secret-runtime-active",
                        "type": "webhook",
                        "auth_type": "password",
                        "config": {
                            "url": f"http://127.0.0.1:{self.aux_port}/secret",
                            "method": "GET",
                            "query_key": "hostname",
                            "timeout": 5,
                            "response_data_path": "data",
                            "field_mapping": {"username": "username", "password": "password"},
                            "auth": {"type": "none"},
                        },
                    }
                ),
            ]
        )["data"]

        task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-secret-runtime-override",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "local",
                        "description": "task with disabled default secret source",
                        "secrets_source_ids": [disabled_source["id"]],
                    }
                ),
            ]
        )["data"]

        run_resp = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{task['id']}/execute",
                *tenant_headers,
                "-d",
                json.dumps({"target_hosts": "localhost", "secrets_source_ids": [active_source["id"]]}),
            ]
        )["data"]
        final_run = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{run_resp['id']}", *tenant_view_headers])),
            timeout=20,
            interval=1,
            description="runtime override execution result",
        )
        assert_eq(final_run["status"], "success", "runtime secret source override should take precedence and allow execution")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{task['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{disabled_source['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/secrets-sources/{active_source['id']}", *tenant_view_headers])

        self.results["secrets_runtime_override"] = {"status": "passed"}

    def run_notification_variables(self):
        info("==> notification template variables")
        tenant_view_headers = self.auth_args(self.tenant_admin_token)
        tenant_headers = self.auth_args(self.tenant_admin_token, json_content=True)

        variable_hits_before = len(self.provider_hits("/notify-vars"))
        vars_channel = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/channels",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-webhook-vars",
                        "type": "webhook",
                        "description": "webhook channel for variable rendering",
                        "config": {"url": f"http://127.0.0.1:{self.aux_port}/notify-vars", "method": "POST", "timeout_seconds": 5},
                        "recipients": ["ops@example.com"],
                    }
                ),
            ]
        )["data"]

        start_template = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/templates",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-template-vars-start",
                        "description": "start template variables",
                        "event_type": "execution.start",
                        "supported_channels": ["webhook"],
                        "subject_template": "Start {{task.name}} {{execution.status}}",
                        "body_template": "run={{execution.run_id}} status={{execution.status}} trigger={{execution.trigger_type}} started={{execution.started_at}} hosts={{task.host_count}} system={{system.name}}",
                        "format": "text",
                    }
                ),
            ]
        )["data"]
        success_template = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/templates",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-template-vars-success",
                        "description": "success template variables",
                        "event_type": "execution.success",
                        "supported_channels": ["webhook"],
                        "subject_template": "Done {{task.name}} {{execution.status}}",
                        "body_template": "run={{execution.run_id}} code={{execution.exit_code}} ok={{stats.ok}} failed={{stats.failed}} duration={{execution.duration}} repo={{repository.playbook}} executor={{task.executor_type}} system={{system.name}}",
                        "format": "text",
                    }
                ),
            ]
        )["data"]

        vars_task = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks",
                *tenant_headers,
                "-d",
                json.dumps(
                    {
                        "name": "acc-task-vars",
                        "playbook_id": self.local_playbook["id"],
                        "target_hosts": "localhost",
                        "executor_type": "local",
                        "description": "task for notification variable rendering",
                        "notification_config": {
                            "enabled": True,
                            "on_start": {
                                "enabled": True,
                                "channel_ids": [vars_channel["id"]],
                                "template_id": start_template["id"],
                            },
                            "on_success": {
                                "enabled": True,
                                "channel_ids": [vars_channel["id"]],
                                "template_id": success_template["id"],
                            },
                        },
                    }
                ),
            ]
        )["data"]

        run_resp = curl_json(
            [
                "-X",
                "POST",
                f"{self.api_base()}/tenant/execution-tasks/{vars_task['id']}/execute",
                *tenant_headers,
                "-d",
                json.dumps({"target_hosts": "localhost"}),
            ]
        )["data"]

        final_run = self.wait_until(
            lambda: (
                lambda payload: payload["data"]
                if payload["data"]["status"] in ("success", "failed", "partial", "cancelled")
                else None
            )(curl_json([f"{self.api_base()}/tenant/execution-runs/{run_resp['id']}", *tenant_view_headers])),
            timeout=30,
            interval=1,
            description="variable notification execution run",
        )
        assert_eq(final_run["status"], "success", "variable notification task should succeed")

        self.wait_until(
            lambda: len(self.provider_hits("/notify-vars")) >= variable_hits_before + 2,
            timeout=10,
            interval=1,
            description="variable notification hits",
        )
        var_hits = self.provider_hits("/notify-vars")[-2:]
        start_hit = next(hit for hit in var_hits if hit["body"]["subject"].startswith("Start "))
        success_hit = next(hit for hit in var_hits if hit["body"]["subject"].startswith("Done "))

        start_body = start_hit["body"]
        success_body = success_hit["body"]
        assert_eq(start_body["subject"], "Start acc-task-vars running", "start subject should render variables")
        assert_true(f"run={run_resp['id']}" in start_body["body"], "start body should include run id")
        assert_true("status=running" in start_body["body"], "start body should include running status")
        assert_true("trigger=manual" in start_body["body"], "start body should include trigger type")
        assert_true("hosts=1" in start_body["body"], "start body should include host count")
        assert_true("system=Auto-Healing" in start_body["body"], "start body should include system name")
        assert_true("{{" not in start_body["body"], "start body should not contain unresolved variables")

        assert_eq(success_body["subject"], "Done acc-task-vars success", "success subject should render variables")
        assert_true(f"run={run_resp['id']}" in success_body["body"], "success body should include run id")
        assert_true("code=0" in success_body["body"], "success body should include exit code")
        assert_true("ok=" in success_body["body"], "success body should include stats.ok")
        assert_true("failed=0" in success_body["body"], "success body should include stats.failed")
        assert_true("repo=local.yml" in success_body["body"], "success body should include repository playbook path")
        assert_true("executor=local" in success_body["body"], "success body should include executor type")
        assert_true("system=Auto-Healing" in success_body["body"], "success body should include system name")
        assert_true("{{" not in success_body["body"], "success body should not contain unresolved variables")

        notification_logs = curl_json(
            [f"{self.api_base()}/tenant/notifications?execution_run_id={run_resp['id']}&status=sent&page=1&page_size=20", *tenant_view_headers]
        )
        logs = self.list_items(notification_logs)
        assert_true(len(logs) >= 2, "execution-linked notification logs should include start and success")

        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/execution-tasks/{vars_task['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/templates/{start_template['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/templates/{success_template['id']}", *tenant_view_headers])
        curl_json(["-X", "DELETE", f"{self.api_base()}/tenant/channels/{vars_channel['id']}", *tenant_view_headers])

        self.results["notification_variables"] = {"status": "passed"}

    def run_query_token_sse(self):
        info("==> query token / SSE")
        token = self.tenant_admin_token
        status, _ = curl_status_json([f"{self.api_base()}/common/search?q=test&token={token}"])
        assert_eq(status, 401, "query token should be rejected for normal API")
        proc = subprocess.run(
            [
                "curl",
                "-sN",
                "--max-time",
                "5",
                f"{self.api_base()}/tenant/site-messages/events?token={token}",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
        )
        assert_true("event: init" in proc.stdout, "SSE query token should still work")
        self.results["query_token_sse"] = {"status": "passed"}

    def write_report(self):
        suffix = "all"
        if self.selected_phases != EXPOSED_PHASES:
            suffix = "-".join(self.selected_phases)
        report_path = ROOT / "docs" / f"acceptance-automation-results-{suffix}-2026-03-22.json"
        report_path.write_text(json.dumps(self.results, ensure_ascii=False, indent=2) + "\n")
        info(f"wrote report: {report_path}")


def resolve_phases(selected):
    order = [
        "auth",
        "tenant_setup",
        "platform_tenants",
        "settings_secrets_dictionaries",
        "common",
        "profile_rbac_misc",
        "dashboard",
        "tenant_boundaries",
        "search_site_messages",
        "impersonation",
        "healing",
        "healing_queries",
        "git_execution",
        "plugin_cmdb",
        "execution_queries",
        "interface_contract_smoke",
        "workbench_site_messages",
        "notifications_audit",
        "notification_variables",
        "notification_failures",
        "notification_retry",
        "notification_retry_exhaustion",
        "notification_rate_limit",
        "notification_retry_tenant_scope",
        "secrets_default_fallback",
        "secrets_disabled_usage",
        "secrets_runtime_override",
        "secrets_reference_updates",
        "secrets_update_constraints",
        "blacklist_security",
        "blacklist_exemption_execution",
        "audit_action_assertions",
        "dashboard_overview_stats",
        "filters_pagination",
        "query_token_sse",
    ]
    needed = set()

    def add_phase(phase):
        if phase == "tenant_setup":
            needed.add("tenant_setup")
            return
        for dep in PHASE_DEPENDENCIES.get(phase, []):
            add_phase(dep)
        needed.add(phase)

    for phase in selected:
        add_phase(phase)

    return [phase for phase in order if phase in needed]


def main():
    parser = argparse.ArgumentParser(description="Run real acceptance scenarios against an isolated Auto-Healing environment")
    parser.add_argument(
        "--phase",
        action="append",
        choices=EXPOSED_PHASES,
        help="Run only the selected phase(s). Can be specified multiple times.",
    )
    parser.add_argument("--list-phases", action="store_true", help="List available phases and exit")
    args = parser.parse_args()

    if args.list_phases:
        for phase in EXPOSED_PHASES:
            print(phase)
        return

    selected_phases = args.phase or EXPOSED_PHASES[:]

    for cmd in ["curl", "jq", "docker", "git", "python3.11"]:
        require(cmd)

    runner = AcceptanceRunner(selected_phases=selected_phases)
    atexit.register(runner.cleanup)

    runner.build_binaries()
    runner.prepare_repo()
    runner.prepare_db()
    runner.start_mocks()
    runner.start_server()
    runner.init_admin()

    phase_runners = {
        "auth": runner.run_auth_scenarios,
        "tenant_setup": runner.setup_tenants_and_users,
        "platform_tenants": runner.run_platform_tenants,
        "settings_secrets_dictionaries": runner.run_settings_secrets_dictionaries,
        "common": runner.run_common_isolation,
        "profile_rbac_misc": runner.run_profile_rbac_misc,
        "workbench_site_messages": runner.run_workbench_site_messages,
        "dashboard_overview_stats": runner.run_dashboard_overview_stats,
        "dashboard": runner.run_dashboard,
        "tenant_boundaries": runner.run_tenant_boundaries,
        "search_site_messages": runner.run_search_and_site_messages,
        "impersonation": runner.run_impersonation,
        "healing": runner.run_healing_flow,
        "healing_queries": runner.run_healing_queries,
        "git_execution": runner.run_git_playbook_execution,
        "plugin_cmdb": runner.run_plugin_cmdb,
        "execution_queries": runner.run_execution_queries,
        "interface_contract_smoke": runner.run_interface_contract_smoke,
        "notifications_audit": runner.run_notifications_audit,
        "notification_variables": runner.run_notification_variables,
        "notification_failures": runner.run_notification_failures,
        "notification_retry": runner.run_notification_retry,
        "notification_retry_exhaustion": runner.run_notification_retry_exhaustion,
        "notification_rate_limit": runner.run_notification_rate_limit,
        "notification_retry_tenant_scope": runner.run_notification_retry_tenant_scope,
        "secrets_default_fallback": runner.run_secrets_default_fallback,
        "secrets_disabled_usage": runner.run_secrets_disabled_usage,
        "secrets_runtime_override": runner.run_secrets_runtime_override,
        "secrets_reference_updates": runner.run_secrets_reference_updates,
        "secrets_update_constraints": runner.run_secrets_update_constraints,
        "blacklist_security": runner.run_blacklist_security,
        "blacklist_exemption_execution": runner.run_blacklist_exemption_execution,
        "audit_action_assertions": runner.run_audit_action_assertions,
        "filters_pagination": runner.run_filters_pagination,
        "query_token_sse": runner.run_query_token_sse,
    }

    for phase in resolve_phases(selected_phases):
        phase_runners[phase]()
    runner.write_report()

    info("all acceptance scenarios passed")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        traceback.print_exc()
        print(f"ACCEPTANCE FAILED: {exc}", file=sys.stderr)
        sys.exit(1)
