# Development Guide

> Auto-Healing Platform 开发指南

## Prerequisites

| Dependency | Version | Purpose |
|-----------|---------|---------|
| Go | 1.24+ | Backend language |
| PostgreSQL | 15+ | Primary database |
| Redis | 7+ | Cache & message queue |
| Ansible | 2.14+ | Automation engine |
| Docker | 24+ | Infrastructure & executor |
| Node.js | 20+ | Frontend build (optional) |

## Quick Setup

```bash
# 1. Clone the repository
git clone https://github.com/heyangguang/auto-healing.git
cd auto-healing

# 2. Start infrastructure
make infra-up

# 3. Build and run
make build
./bin/init-admin
./bin/server
```

## Project Structure

```text
auto-healing/
├── cmd/
│   ├── server/                      # Main server entry point
│   └── init-admin/                  # Admin initialization tool
├── internal/
│   ├── app/httpapi/                 # Top-level route composition
│   ├── config/                      # Configuration loading
│   ├── database/                    # Database / Redis initialization
│   ├── middleware/                  # Shared HTTP middleware
│   ├── model/                       # GORM data models
│   ├── modules/                     # Domain modules
│   │   ├── access/
│   │   ├── automation/
│   │   ├── engagement/
│   │   ├── integrations/
│   │   ├── ops/
│   │   └── secrets/
│   ├── platform/                    # Shared runtime capabilities
│   │   ├── events/
│   │   ├── httpx/
│   │   ├── lifecycle/
│   │   ├── repository/
│   │   └── repositoryx/
│   ├── engine/                      # Execution engine
│   ├── scheduler/                   # Background schedulers
│   ├── notification/                # Notification engine + providers
│   ├── secrets/                     # Secrets providers
│   ├── git/                         # Git base capabilities
│   └── pkg/                         # Shared utility packages
├── migrations/                      # SQL migration files
├── configs/                         # Configuration templates
├── deployments/                     # Deployment configs
├── docs/                            # Documentation
├── docker/                          # Docker / executor images
└── web/                             # Frontend source
```

## Architecture

### Top-Level Organization

Backend code is organized by **business domain first**, not by a top-level global `handler / service / repository` split.

At the top level:

- `internal/modules/*` owns business code
- `internal/platform/*` owns shared runtime capabilities
- `internal/app/httpapi` owns route composition

Within a single domain module, code is still separated by responsibility:

```text
internal/modules/<domain>/
├── module.go
├── httpapi/
├── service/
└── repository/
```

### Domain Modules

| Module | Responsibility |
|-------|----------------|
| `access` | auth, tenant, user, role, permission, impersonation |
| `automation` | execution, healing, scheduling |
| `engagement` | dashboard, workbench, search, notification, site message, preference |
| `integrations` | plugin, git, cmdb, playbook |
| `ops` | audit, blacklist, dictionary, platform settings |
| `secrets` | secrets source admin and query |

### Shared Runtime

| Directory | Responsibility |
|-----------|----------------|
| `internal/platform/httpx/` | pagination, query parsing, validation formatting, SSE, resource error helpers |
| `internal/platform/lifecycle/` | lifecycle hooks and cleanup |
| `internal/platform/events/` | shared event bus |
| `internal/platform/repository/` | shared repositories such as `audit`, `cmdb`, `incident`, `settings` |
| `internal/platform/repositoryx/` | tenant context and scoped DB helpers |

### Route Composition

Routes are composed in:

- `internal/app/httpapi/router.go`
- `internal/app/httpapi/modules.go`

New APIs should be registered through the relevant domain registrar and module wiring, not through a global legacy `routes.go`.

### Key Principles

- **Domain-first organization** — new business code goes under `internal/modules/<domain>/...`
- **Shared runtime stays in platform** — cross-domain helpers belong in `internal/platform/*`
- **HTTP DTOs stay close to handlers** — request/response structs belong in each module’s `httpapi/`
- **Backend Authority** — frontend is display-only; business rules live in backend
- **Provider Pattern** — engines, notifications, schedulers, and secrets integrations keep explicit provider structures

### Provider Pattern

```go
type Provider interface {
    Execute(ctx context.Context, params Params) (Result, error)
}
```

Typical provider-oriented layout:

```text
module/
├── interface.go
└── provider/
    ├── impl_a.go
    └── impl_b.go
```

## Adding New Features

### New API Endpoint

1. Add or update model definitions in `internal/model/` when persistence shape changes
2. Implement repository logic in `internal/modules/<domain>/repository/`
3. Implement use-case logic in `internal/modules/<domain>/service/`
4. Implement handlers and DTOs in `internal/modules/<domain>/httpapi/`
5. Wire dependencies in `internal/modules/<domain>/module.go`
6. Register the module endpoint via `internal/app/httpapi/modules.go`
7. Add migration in `migrations/` if schema changes
8. Update `docs/openapi.yaml`

### New Shared HTTP / Runtime Helper

1. Put HTTP-centric helpers in `internal/platform/httpx/`
2. Put shared lifecycle/event helpers in `internal/platform/lifecycle/` or `internal/platform/events/`
3. Put cross-domain repository helpers in `internal/platform/repository/` or `internal/platform/repositoryx/`

### New Provider Implementation

1. Create `internal/<module>/provider/<impl>.go`
2. Implement the required interface
3. Register it in the module’s factory / registry

## Database Migrations

Migration files are in `migrations/` and executed sequentially on startup.

```bash
touch migrations/$(date +%Y%m%d%H%M%S)_add_new_table.sql
```

Rules:

- New environments: only modify migration files
- Existing environments: migration file + explicit rollout steps
- Use `TIMESTAMPTZ` for timestamps
- Use `UUID` for primary keys
- Use `JSONB` for dynamic data

## Configuration

Configuration lives in `configs/config.yaml` and can be overridden with environment variables prefixed `AH_`:

```bash
AH_DATABASE_HOST=db.prod.local
AH_DATABASE_PASSWORD=secure_password
AH_JWT_SECRET=your-256-bit-secret
AH_SERVER_PORT=8080
```

## Testing

```bash
# Run all tests
make test

# Run one domain service package
go test ./internal/modules/automation/service/healing/... -v

# Run with race detector
go test ./... -race
```

## Code Style

- Run `golangci-lint` before committing: `make lint`
- Prefer domain modules over new top-level horizontal layers
- Keep shared helpers in `internal/platform/*`, not in ad-hoc utility packages
- Use `json:"-"` to hide fields from JSON output
- Use pointer types (`*Type`) for nullable fields
- Use explicit JSON tags for API fields

## Commit Convention

```text
feat: add new feature
fix: bug fix
docs: documentation changes
refactor: code refactoring
test: add or update tests
chore: build process or auxiliary tool changes
```

## Useful Make Commands

```bash
make help          # Show all commands
make dev           # Run in dev mode
make build         # Build for current platform
make test          # Run tests
make lint          # Run linter
make release       # Cross-compile for all 6 platforms
make infra-up      # Start PostgreSQL + Redis
make infra-down    # Stop infrastructure
make infra-reset   # Reset infrastructure (destroy data)
make clean         # Clean build artifacts
```
