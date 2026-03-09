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

```
auto-healing/
├── cmd/
│   ├── server/          # Main server entry point
│   └── init-admin/      # Admin initialization tool
├── internal/
│   ├── config/          # Configuration loading
│   ├── database/        # Database connection
│   ├── middleware/       # HTTP middleware (JWT, CORS, Audit)
│   ├── model/           # GORM data models
│   ├── repository/      # Database CRUD layer
│   ├── service/         # Business logic layer
│   ├── handler/         # HTTP handlers & DTOs
│   ├── adapter/         # External system adapters (ITSM/CMDB)
│   ├── engine/          # Execution engine (Ansible)
│   ├── scheduler/       # Background job scheduler
│   ├── secrets/         # Secrets management providers
│   ├── notification/    # Notification engine (Email/DingTalk/Webhook)
│   ├── gitclient/       # Git client wrapper
│   └── pkg/             # Internal utilities (logger, jwt, response)
├── migrations/          # SQL migration files
├── configs/             # Configuration templates
├── deployments/         # Docker Compose & deployment configs
├── docs/                # Documentation
├── docker/              # Dockerfile and executor images
└── web/                 # Frontend source (React)
```

## Architecture

### Layered Architecture

```
Handler → Service → Repository → Database
```

| Layer | Directory | Responsibility |
|-------|-----------|---------------|
| **Handler** | `internal/handler/` | HTTP request handling, DTO definition |
| **Service** | `internal/service/<module>/` | Business logic (accepts Model or primitives only) |
| **Repository** | `internal/repository/` | Database CRUD operations |
| **Model** | `internal/model/` | GORM struct definitions |

### Key Principles

- **DTO stays in Handler layer** — Service layer never accepts DTOs
- **DTOs must provide `ToModel()` method** for conversion
- **Backend Authority** — Frontend is display-only; all business logic resides in backend
- **Provider Pattern** — External integrations use `interface.go` + `provider/` structure

### Provider Pattern

```go
// interface.go — Interface definition + factory function
type Provider interface {
    Execute(ctx context.Context, params Params) (Result, error)
}

func NewProvider(providerType string) Provider {
    switch providerType {
    case "ansible":
        return &AnsibleProvider{}
    // ...
    }
}
```

```
module/
├── interface.go          # Interface + factory
├── types/                # Shared types (optional)
│   └── types.go
└── provider/             # Implementations
    ├── impl_a.go
    └── impl_b.go
```

## Adding New Features

### New API Endpoint

1. Define Model in `internal/model/<entity>.go`
2. Create Repository in `internal/repository/<entity>.go`
3. Create Service in `internal/service/<module>/service.go`
4. Create Handler in `internal/handler/<module>_handler.go`
5. Create DTOs in `internal/handler/<module>_dto.go`
6. Register routes in `internal/handler/routes.go`
7. Add migration in `migrations/`
8. Update `docs/openapi.yaml`

### New Provider Implementation

1. Create `internal/<module>/provider/<impl>.go`
2. Implement the Provider interface
3. Register in `interface.go` factory function

## Database Migrations

Migration files are in `migrations/` directory, executed sequentially on startup.

```bash
# Create new migration
touch migrations/$(date +%Y%m%d%H%M%S)_add_new_table.sql
```

**Rules:**
- New environments: Only modify migration files
- Existing environments: Migration file + manual `ALTER TABLE` execution
- Use `TIMESTAMPTZ` for all timestamps
- Use `UUID` for primary keys
- Use `JSONB` for dynamic data

## Configuration

Configuration via `configs/config.yaml`, overridable with environment variables prefixed `AH_`:

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

# Run specific package tests
go test ./internal/service/healing/... -v

# Run with race detector
go test ./... -race
```

## Code Style

- Run `golangci-lint` before committing: `make lint`
- Follow standard Go project layout
- Use `json:"-"` to hide fields from JSON output (don't delete the field)
- Use pointer types (`*Type`) for nullable fields
- Use `json:"field_name"` (no omitempty) to always show null values

## Commit Convention

```
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
