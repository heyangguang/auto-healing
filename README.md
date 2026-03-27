<p align="center">
  <img src="docs/images/pangolin-logo-full.png" alt="Pangolin - Auto-Healing Platform" width="400" />
</p>

<h1 align="center">Pangolin — Auto-Healing Platform</h1>

<p align="center">
  <strong>Enterprise-grade Intelligent IT Operations Self-Healing Platform</strong>
</p>

<p align="center">
  <a href="https://github.com/heyangguang/auto-healing/releases"><img src="https://img.shields.io/github/v/release/heyangguang/auto-healing?style=flat-square&color=blue" alt="Release" /></a>
  <a href="https://github.com/heyangguang/auto-healing/blob/main/LICENSE"><img src="https://img.shields.io/github/license/heyangguang/auto-healing?style=flat-square" alt="License" /></a>
  <a href="https://goreportcard.com/report/github.com/heyangguang/auto-healing"><img src="https://goreportcard.com/badge/github.com/heyangguang/auto-healing?style=flat-square" alt="Go Report Card" /></a>
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/Ansible-2.14+-EE0000?style=flat-square&logo=ansible&logoColor=white" alt="Ansible" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=white" alt="React" />
</p>

<p align="center">
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-features">Features</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="#-deployment">Deployment</a> •
  <a href="#-documentation">Documentation</a> •
  <a href="#-contributing">Contributing</a>
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README_zh-CN.md">简体中文</a> | <a href="./README_ja.md">日本語</a>
</p>

---

## 🌟 What is Auto-Healing?

**Auto-Healing Platform (AHS)** is an open-source, enterprise-grade intelligent IT operations automation platform. It bridges the gap between **alert detection** and **automated remediation** by orchestrating ITSM tickets, CMDB assets, Ansible playbooks, and approval workflows into a seamless self-healing pipeline.

> **From "seeing the problem" to "resolving it automatically" — with full auditability.**

```
┌─────────────┐     ┌──────────────────┐     ┌────────────────────┐     ┌──────────────────┐
│  External    │────▶│  Alert Ingestion │────▶│  Smart Rule Engine │────▶│  Auto Remediation│
│  ITSM/Mon.  │     │  (Plugin System) │     │  (Healing Rules)   │     │  + Human Approval│
└─────────────┘     └──────────────────┘     └────────────────────┘     └──────────────────┘
                                                                                 │
                              ◀────────── Full Audit Trail & Real-time SSE ──────┘
```

### 💡 Why Auto-Healing?

| Challenge | Traditional Approach | With Auto-Healing |
|-----------|---------------------|-------------------|
| **Alert Fatigue** | Thousands of alerts, manual triage | Smart rule matching, auto-resolution |
| **Slow MTTR** | 30+ min average response time | **< 2 min** for automated remediation |
| **Repetitive Tasks** | Manual disk cleanup, service restart | Fully automated with DAG workflows |
| **No Audit Trail** | Operations depend on tribal knowledge | Immutable forensic-grade logs |
| **Tool Silos** | ITSM, CMDB, scripts are disconnected | Unified platform with plugin integration |

---

## ✨ Features

### 🔄 Self-Healing Engine
- **Visual DAG Workflow Editor** — Design complex remediation flows with drag-and-drop
- **9 Node Types** — Host extraction, CMDB validation, conditional branching, execution, approval, notification, variable setting, loops, and compute
- **Dual Trigger Modes** — Automatic (zero-touch) or manual (approval-gated)
- **Dry-Run Sandbox** — Simulate workflow execution without side effects
- **Real-time SSE Streaming** — Node status updates in < 200ms

### 🔌 Plugin Integration
- **Universal ITSM/CMDB Adapters** — ServiceNow, Jira, Zabbix, or custom systems
- **Field Mapping Engine** — Visual configuration of external-to-internal field mapping
- **Smart Filtering** — AND/OR logic groups for selective data sync
- **Bi-directional Writeback** — Update ticket status after remediation

### ⚡ Execution Center
- **3 Trigger Modes** — Manual launch pad, scheduled (cron), and healing-flow triggered
- **Ansible Engine** — Docker and Local dual-mode executors
- **3-State Validation** — Prevents false success (reachability verification)
- **Runtime Variable Override** — Dynamic host and variable injection per execution

### 🔐 Security & Access
- **RBAC** — Resource-level + operation-level permission control
- **JWT + SAML 2.0** — Dual-track authentication with enterprise SSO (ADFS, Azure AD)
- **JIT Provisioning** — Auto-create user accounts on first SSO login
- **Secrets Management** — Centralized credential vault (SSH, API keys, tokens)
- **Audit Logging** — Every operation tracked with operator attribution

### 📊 Additional Modules
- **CMDB Asset Management** — 3-state lifecycle (active/offline/maintenance), bulk maintenance windows
- **Git Repository Management** — Automated sync, branch locking, drift detection
- **Playbook Templates** — Recursive variable scanning, change drift tracking
- **Notification Engine** — Multi-channel (Email, DingTalk, Webhook) with 40+ template variables
- **Pending Center** — Unified decision console for approvals and manual triggers
- **Analytics Dashboard** — KPI cards, rule hit distribution, trend visualization

---

## 🏗 Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Frontend (React SPA)                        │
│   React 19 · Umi 4 · Ant Design 6 · ProComponents · React Flow    │
└─────────────────────────────┬───────────────────────────────────────┘
                              │ REST API
┌─────────────────────────────▼───────────────────────────────────────┐
│                         Backend (Go)                                │
│   Gin HTTP Router · Layered Architecture                            │
│   Handler → Service → Repository → Database                        │
│                                                                     │
│   ┌────────────────┐  ┌───────────────┐  ┌───────────────────────┐ │
│   │ Healing Engine │  │ Plugin Adapter│  │ Execution Engine      │ │
│   │ (DAG Executor) │  │ (ITSM/CMDB)  │  │ (Ansible Runner)      │ │
│   └────────────────┘  └───────────────┘  └───────────────────────┘ │
│   ┌────────────────┐  ┌───────────────┐  ┌───────────────────────┐ │
│   │ Auth (JWT+SAML)│  │ Notification  │  │ Background Scheduler  │ │
│   └────────────────┘  └───────────────┘  └───────────────────────┘ │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│                        Data Layer                                   │
│   PostgreSQL (JSONB · TIMESTAMPTZ · UUID PKs · Partitioned Index)  │
│   Redis (Cache & Message Queue)                                     │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────────────┐
│                       Execution Layer                               │
│   Ansible Engine (Local / Docker dual-mode)                         │
│   SSH Credential Injection · Jinja2 Variable Rendering             │
└─────────────────────────────────────────────────────────────────────┘
```

### Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Language** | Go 1.24+ | High-performance, low-memory backend |
| **Web Framework** | Gin | Industry-leading Go HTTP framework |
| **ORM** | GORM | Feature-rich ORM with JSONB support |
| **Database** | PostgreSQL 15+ | JSONB, UUID, TIMESTAMPTZ native support |
| **Cache** | Redis 7+ | Caching and message queue |
| **Auth** | JWT + SAML 2.0 | Dual-track with enterprise SSO |
| **Real-time** | Server-Sent Events | Lightweight unidirectional streaming |
| **Expression** | expr-lang/expr | High-perf Go expression evaluation |
| **Automation** | Ansible 2.14+ | Infrastructure automation engine |
| **Frontend** | React 19 + Umi 4 | Enterprise-grade React framework |
| **UI Library** | Ant Design 6 | Mature enterprise UI ecosystem |
| **Workflow UI** | React Flow (xyflow) | Professional DAG visual editor |

### Data Model

```
incidents ──────────▶ healing_rules ──────────▶ healing_flows
    │                                                │
    │                                          nodes (JSONB)
    │                                                │
    └──▶ flow_instances ◀────────────────────────────┘
              │
              ├──▶ approval_tasks
              └──▶ flow_execution_logs

plugins ──────▶ incidents
plugins ──────▶ cmdb_items

git_repositories ──────▶ playbook_templates ──────▶ execution_task_templates
                                                          │
                                               execution_runs ──▶ execution_logs
                                               execution_schedules
```

### Project Structure

```
auto-healing/
├── cmd/
│   ├── server/                  # Main application entry
│   └── init-admin/              # Admin initialization tool
├── internal/
│   ├── config/                  # Configuration management
│   ├── database/                # Database connection & migration
│   ├── engine/                  # Execution engine (Ansible)
│   │   ├── interface.go
│   │   └── provider/ansible/
│   ├── git/                     # Git operations
│   ├── handler/                 # HTTP handlers (DTO layer)
│   ├── middleware/              # HTTP middleware (Auth, CORS, Logging)
│   ├── model/                   # Data models
│   ├── notification/            # Notification providers (Email, DingTalk, Webhook)
│   ├── pkg/                     # Internal shared packages
│   ├── repository/              # Data access layer
│   ├── scheduler/               # Background job schedulers
│   ├── secrets/                 # Secret providers (File, Vault, Webhook)
│   └── service/                 # Business logic layer
│       ├── execution/
│       ├── git/
│       ├── healing/
│       ├── plugin/
│       └── secrets/
├── migrations/                  # SQL migration files (up/down)
├── configs/                     # Configuration templates
├── deployments/
│   ├── docker/                  # Docker Compose for infra
│   └── kubernetes/              # Kubernetes manifests
├── docker/
│   └── ansible-executor/        # Ansible executor Docker image
├── docs/                        # Documentation & screenshots
├── tools/                       # Dev tools & mock services
└── tests/                       # E2E tests
```

---

## 🚀 Quick Start

### Prerequisites

| Dependency | Version | Required |
|-----------|---------|----------|
| Go | 1.24+ | ✅ |
| PostgreSQL | 15+ | ✅ |
| Redis | 7+ | ✅ |
| Ansible | 2.14+ | ✅ (for execution) |
| Docker | 24+ | Optional |

### Option 1: Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/heyangguang/auto-healing.git
cd auto-healing

# Start infrastructure (PostgreSQL + Redis)
cd deployments/docker
docker compose up -d
cd ../..

# Build the server
go build -o bin/server ./cmd/server
go build -o bin/init-admin ./cmd/init-admin

# Initialize admin account
./bin/init-admin

# Start the server
./bin/server
```

### Option 2: Quick Start Script

```bash
git clone https://github.com/heyangguang/auto-healing.git
cd auto-healing

# Start everything with one command
./start-all.sh
```

### Verify Installation

```bash
# Health check
curl http://localhost:8080/health
# Expected: {"status":"ok"}

# Login
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}'
```

> **Default credentials:** `admin` / `admin123456`  
> ⚠️ **Change the default password immediately in production!**

---

## 📦 Pre-built Binaries

Download pre-built binaries from the [Releases](https://github.com/heyangguang/auto-healing/releases) page.

### Supported Platforms

| OS | Architecture | Binary Name |
|----|-------------|-------------|
| **Linux** | x86_64 (amd64) | `auto-healing-linux-amd64` |
| **Linux** | ARM64 (aarch64) | `auto-healing-linux-arm64` |
| **macOS** | Intel (amd64) | `auto-healing-darwin-amd64` |
| **macOS** | Apple Silicon (arm64) | `auto-healing-darwin-arm64` |
| **Windows** | x86_64 (amd64) | `auto-healing-windows-amd64.exe` |
| **Windows** | ARM64 | `auto-healing-windows-arm64.exe` |

### Install from Release

```bash
# Linux (amd64)
curl -LO https://github.com/heyangguang/auto-healing/releases/latest/download/auto-healing-linux-amd64.tar.gz
tar -xzf auto-healing-linux-amd64.tar.gz
chmod +x auto-healing-linux-amd64
./auto-healing-linux-amd64

# macOS (Apple Silicon)
curl -LO https://github.com/heyangguang/auto-healing/releases/latest/download/auto-healing-darwin-arm64.tar.gz
tar -xzf auto-healing-darwin-arm64.tar.gz
chmod +x auto-healing-darwin-arm64
./auto-healing-darwin-arm64
```

### Build from Source

```bash
# Build for current platform
go build -o bin/server ./cmd/server

# Cross-compile for all platforms
GOOS=linux   GOARCH=amd64 go build -o bin/auto-healing-linux-amd64      ./cmd/server
GOOS=linux   GOARCH=arm64 go build -o bin/auto-healing-linux-arm64      ./cmd/server
GOOS=darwin  GOARCH=amd64 go build -o bin/auto-healing-darwin-amd64     ./cmd/server
GOOS=darwin  GOARCH=arm64 go build -o bin/auto-healing-darwin-arm64     ./cmd/server
GOOS=windows GOARCH=amd64 go build -o bin/auto-healing-windows-amd64.exe ./cmd/server
GOOS=windows GOARCH=arm64 go build -o bin/auto-healing-windows-arm64.exe ./cmd/server
```

---

## 🐳 Docker Images

Official Docker images are available on GitHub Container Registry:

| Image | Description |
|-------|-------------|
| `ghcr.io/heyangguang/auto-healing` | **Server** — Main API & healing engine |
| `ghcr.io/heyangguang/auto-healing-executor` | **Executor** — Isolated Ansible execution environment |

### Quick Start with Docker

```bash
# Pull images
docker pull ghcr.io/heyangguang/auto-healing:latest
docker pull ghcr.io/heyangguang/auto-healing-executor:latest

# Run server (requires PostgreSQL & Redis)
docker run -d --name auto-healing \
  -p 8080:8080 \
  -e AH_DATABASE_HOST=your-postgres-host \
  -e AH_REDIS_HOST=your-redis-host \
  ghcr.io/heyangguang/auto-healing:latest
```

### What is the Executor?

The platform supports **two execution modes** for running Ansible Playbooks:

| Mode | How it Works | Best For |
|------|-------------|----------|
| **Local** | Runs Ansible directly on the server host | Simple setups, development |
| **Docker** | Runs Ansible inside `auto-healing-executor` container | Production (isolated, reproducible) |

The Executor image comes pre-installed with:
- `ansible-core 2.14.18` (supports Python 3.6+ targets)
- `paramiko`, `sshpass` for SSH connectivity
- `git`, `curl` for utilities

> 💡 Docker mode ensures each execution runs in a clean, isolated environment — preventing dependency conflicts and improving security.

---

## 🔧 Configuration

Create `configs/config.yaml` from the template:

```bash
cp configs/config.yaml configs/config.local.yaml
```

### Configuration Reference

```yaml
app:
  name: Auto-Healing
  version: 1.0.0
  url: http://localhost:8080
  env: production          # production | staging | development

server:
  host: 0.0.0.0
  port: 8080
  mode: release            # debug | release | test

database:
  host: localhost
  port: 5432
  user: postgres
  password: your-secure-password
  dbname: auto_healing
  ssl_mode: disable        # disable | require | verify-full
  max_open_conns: 25
  max_idle_conns: 5
  max_lifetime_minutes: 5

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

jwt:
  secret: CHANGE-THIS-TO-A-STRONG-SECRET-KEY
  access_token_ttl_minutes: 60
  refresh_token_ttl_hours: 168
  issuer: auto-healing

log:
  level: info              # debug | info | warn | error
  console:
    enabled: true
    format: text           # text | json
    color: true
  file:
    enabled: true
    path: ./logs
    filename: app.log
    format: json
    max_size: 100          # MB
    max_backups: 10
    max_age: 30            # days
    compress: true
  db_level: warn           # info | warn | error | off
```

### Environment Variables

All config values can be overridden via environment variables with the prefix `AH_`:

```bash
export AH_DATABASE_HOST=db.example.com
export AH_DATABASE_PASSWORD=secure-password
export AH_JWT_SECRET=my-production-secret
export AH_SERVER_PORT=9090
```

---

## 🚢 Deployment

### Docker Compose (Development / Small Teams)

```bash
cd deployments/docker
docker compose up -d
```

This starts:
- **PostgreSQL 15** (port 5432) with auto-migration
- **Redis 7** (port 6379) with AOF persistence

### Kubernetes (Production)

Kubernetes manifests are available in `deployments/kubernetes/`. Adapt them to your cluster:

```bash
kubectl apply -f deployments/kubernetes/
```

### System Requirements

| Scale | CPU | Memory | Disk | Database |
|-------|-----|--------|------|----------|
| **Small** (< 50 hosts) | 2 cores | 2 GB | 20 GB | PostgreSQL on same host |
| **Medium** (50-500 hosts) | 4 cores | 4 GB | 50 GB | Dedicated PostgreSQL |
| **Large** (500+ hosts) | 8+ cores | 8+ GB | 100+ GB | PostgreSQL HA cluster |

### Production Checklist

- [ ] Change default admin password
- [ ] Set a strong JWT secret
- [ ] Enable PostgreSQL SSL (`ssl_mode: require`)
- [ ] Configure log rotation
- [ ] Set up database backups
- [ ] Configure reverse proxy (Nginx/Caddy) with TLS
- [ ] Restrict network access to management ports
- [ ] Enable Redis authentication

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| [API Reference](api/openapi.yaml) | OpenAPI 3.0 specification |
| [API Testing Guide](docs/api-testing-guide.md) | cURL examples and test workflows |
| [Project Introduction](docs/auto_healing_project_intro.md) | Comprehensive product overview |
| [Internal Architecture](internal/README.md) | Internal directory structure |

### API Quick Reference

```bash
# Authenticate
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

# List plugins
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/plugins | jq

# List incidents
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/incidents | jq

# List healing rules
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/healing/rules | jq

# List CMDB items
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/cmdb/items | jq
```

---

## 🧪 Testing

### End-to-End Tests

```bash
# Start mock services
cd tools
python3 mock_itsm_healing.py &
python3 mock_secrets_healing.py &
python3 mock_notification.py &
cd ..

# Run E2E tests
cd tests/e2e
./test_complete_workflow_docker.sh   # Docker executor mode
./test_complete_workflow_local.sh    # Local executor mode
```

### Unit Tests

```bash
go test ./... -v
```

---

## 📸 Screenshots

<details>
<summary><b>Click to expand screenshots</b></summary>

### Healing Flow DAG Editor
<img src="docs/images/image-1.png" alt="Healing Flow Editor" width="800" />

### Analytics Dashboard
<img src="docs/images/image-8.png" alt="Dashboard" width="800" />

### Plugin Configuration
<img src="docs/images/image-10.png" alt="Plugin Configuration" width="800" />

### Execution Center
<img src="docs/images/image-15.png" alt="Execution Center" width="800" />

### CMDB Asset Management
<img src="docs/images/image-20.png" alt="CMDB Management" width="800" />

### Real-time Execution Logs
<img src="docs/images/image-25.png" alt="Execution Logs" width="800" />

</details>

---

## 🤝 Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

### Development Setup

```bash
# Clone
git clone https://github.com/heyangguang/auto-healing.git
cd auto-healing

# Install dependencies
go mod tidy

# Start infrastructure
cd deployments/docker && docker compose up -d && cd ../..

# Run in development mode
go run ./cmd/server
```

### Code Architecture Principles

- **Backend Authority** — Frontend is a high-fidelity display of backend state; no client-side state transitions
- **Layered Architecture** — `Handler (DTO) → Service (Model) → Repository → Database`
- **Provider Pattern** — All external integrations use `interface.go + provider/` structure
- **Protective Deletion** — Referenced resources cannot be deleted (foreign key reference counting)
- **No-Blind-Spot Forensics** — Every node execution produces at least one Info-level log

### Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add notification rate limiting
fix: resolve SSE connection leak on timeout
docs: update API testing guide
refactor: extract plugin adapter interface
```

---

## 📋 Roadmap

- [ ] 🧠 AI-powered root cause analysis
- [ ] 📱 Mobile companion app
- [ ] 🔗 Terraform / Pulumi integration
- [ ] 📊 Advanced analytics with ML-based anomaly detection
- [ ] 🌐 Multi-tenant SaaS mode
- [ ] 🔄 Bi-directional CMDB sync
- [ ] 📦 Helm Chart for Kubernetes deployment
- [ ] 🤖 ChatOps integration (Slack, Teams)

---

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE).

---

## 🙏 Acknowledgments

- [Gin](https://github.com/gin-gonic/gin) — HTTP web framework
- [GORM](https://gorm.io/) — ORM library for Go
- [Ansible](https://www.ansible.com/) — IT automation engine
- [React Flow](https://reactflow.dev/) — DAG visual editor
- [Ant Design](https://ant.design/) — Enterprise UI library
- [expr-lang](https://github.com/expr-lang/expr) — Expression evaluation engine

---

<p align="center">
  <strong>⭐ If you find this project useful, please give it a star!</strong>
</p>

<p align="center">
  Made with ❤️ by the <a href="https://github.com/heyangguang">Auto-Healing Team</a>
</p>
