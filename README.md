# Broth

**Django-like batteries-included web framework for Go**

Broth brings Rails/Django-style conventions and productivity to Go, without sacrificing Go idioms. It provides a full-stack, opinionated structure for building medium-scale web applications with strong type safety and AI-friendly code generation support.

> **Status**: v0.1.0-draft -- Core framework implemented with auth, session, security middleware, and reference account module. Active development in progress.

## Features

### Core Framework
- **Layered Architecture** -- 4-layer design (HTTP / Application / Domain / Data Access) with compile-time enforced boundaries
- **Module System** -- Feature-based module structure (`broth.Module` interface) with automatic route mounting
- **Standard Library First** -- Built on `net/http` (Go 1.22+ patterns), `database/sql`, `log/slog`, `html/template`
- **No Custom Context** -- Uses `context.Context` with generic type-safe accessors (`httputil.CtxKey[T]`)
- **Constructor Injection** -- No DI container; explicit `New*` constructors for all dependencies

### HTTP & Middleware
- **Router** -- `http.ServeMux` wrapper with middleware chain support and module-prefix mounting
- **Request Logging** -- Structured JSON logging with method, path, status, duration, and request ID
- **Panic Recovery** -- Catches panics, logs stack traces, returns 500
- **Request ID** -- Auto-generates or propagates `X-Request-ID` header
- **SSR/API Auto-Detection** -- 5-rule request context detection (Content-Type, Accept, Bearer token, XHR, default SSR)
- **Security Headers** -- X-Content-Type-Options, X-Frame-Options, Referrer-Policy, CSP (SSR only), optional HSTS
- **CSRF Protection** -- Synchronizer Token Pattern, auto-skipped for API context, constant-time comparison
- **CORS** -- Origin validation, preflight OPTIONS handling, API-context only
- **Rate Limiting** -- Per-IP Token Bucket algorithm with `Retry-After` header

### Authentication & Session
- **JWT Tokens** -- HMAC-SHA256 access tokens with `golang-jwt/jwt` v5, alg=none attack prevention
- **Password Hashing** -- bcrypt-based `PasswordHasher` interface with configurable cost
- **Session Management** -- AES-GCM encrypted cookie-based sessions with JSON serialization
- **Session Lifecycle** -- Regenerate (fixation prevention), Destroy, SetMaxAge (Remember Me)
- **Auth Middleware** -- Bearer token (API) / session-based (SSR) authentication
- **Authorization** -- `RequireAuth()` and `RequireRole(roles...)` middleware
- **RBAC** -- Role and Permission constants, generic `Policy[T]` interface

### Data Access
- **Database Management** -- Connection pool with health checks, configurable limits
- **Transaction Support** -- Nested transactions via SAVEPOINTs, context-propagated
- **Bob Integration** -- `bob.DB`/`bob.Executor` wrappers, `ExecutorFromContext` for transparent tx propagation, `bobgen-psql` code generation workflow
- **Migrations** -- SQL-file based migrations with goose (Up, Down, Status)

### Background Processing
- **Job Queue** -- Two-layer system: in-memory (fast) + PostgreSQL persistent backend (durable)
- **Worker Pool** -- Configurable concurrency with graceful shutdown
- **Persistent Backend** -- PostgreSQL-backed job storage with `FOR UPDATE SKIP LOCKED`, retry/dead-letter, stats, cleanup
- **Scheduler** -- Cron-based scheduling via `robfig/cron` with leader election, overlap prevention, timezone support
- **Leader Election** -- PostgreSQL advisory lock-based distributed leader election for multi-instance deployments

### Developer Experience
- **Config Binding** -- Environment variable binding via struct tags (`env:"KEY"`, `default:"val"`, `required:"true"`)
- **Form Binding** -- Struct tag-based form parsing with field-level validation
- **Template Rendering** -- Thread-safe HTML template rendering with layouts, components, and hot-reload
- **Test Utilities** -- `httptest` helpers and test database setup
- **Single Binary** -- `go build` produces one deployable binary with jobs and scheduler built-in
- **Reference Module** -- Complete account module (register, login, logout, profile, API login)

## Quick Start

### Prerequisites

- **Go 1.24+** ([download](https://go.dev/dl/)) -- required for `go tool` directive
- **PostgreSQL 15+** (for database features)

### Installation

> **Note**: Broth is in pre-release and not yet published to a Go module proxy. Clone the repository and use a `replace` directive in your `go.mod`, or reference a specific commit.

```bash
# Option 1: Clone and use replace directive
git clone https://github.com/source-maker/broth.git
cd your-project
# Add to go.mod: replace github.com/source-maker/broth => ../broth

# Option 2: Reference a specific commit (once published)
go get github.com/source-maker/broth@<commit-hash>
```

### Create a New Project

```bash
mkdir myapp && cd myapp
go mod init myapp
```

### Minimal Application

Create `cmd/myapp/main.go`:

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"

    "github.com/source-maker/broth"
    "github.com/source-maker/broth/log"
    "github.com/source-maker/broth/middleware"
    "github.com/source-maker/broth/router"
)

func main() {
    logger := log.New(slog.LevelInfo)

    app := broth.New(logger)
    app.Use(
        middleware.RequestID,
        middleware.Recovery(logger.Slog()),
        middleware.Logger(logger.Slog()),
        middleware.ContextDetect(),
        middleware.SecurityHeaders(),
    )

    app.Router().Handle(router.Route{
        Pattern: "GET /health",
        Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            w.Write([]byte(`{"status":"ok"}`))
        }),
    })

    srv := &http.Server{Addr: ":8080", Handler: app.Handler()}
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    if err := app.Start(ctx); err != nil {
        slog.Error("app start failed", "error", err)
        os.Exit(1)
    }

    go func() {
        slog.Info("server starting", "addr", ":8080")
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            slog.Error("server error", "error", err)
        }
    }()

    <-ctx.Done()
    slog.Info("shutting down...")
    _ = app.Shutdown(context.Background())
    _ = srv.Shutdown(context.Background())
}
```

### Full Middleware Stack

For a production-ready setup with session, auth, CSRF, CORS, and rate limiting:

```go
// Set up session store (32-byte AES key)
cookieStore, _ := session.NewCookieStore([]byte("your-32-byte-secret-key-here!!"))

// Set up JWT token service
tokenSvc := auth.NewTokenService([]byte("your-jwt-secret"), 24*time.Hour)

app.Use(
    middleware.RequestID,
    middleware.Recovery(logger.Slog()),
    middleware.Logger(logger.Slog()),
    middleware.ContextDetect(),
    middleware.SecurityHeaders(),
    middleware.Session(cookieStore, middleware.SessionConfig{
        CookieName: "myapp_session",
        Path:       "/",
        Secure:     true,  // HTTPS only in production
        MaxAge:     86400, // 24 hours
    }),
    middleware.CSRF(),
    middleware.CORS(middleware.CORSConfig{
        AllowedOrigins: []string{"https://myapp.com"},
    }),
    middleware.RateLimit(middleware.RateLimitConfig{RPS: 100, Burst: 50}),
)
```

### Creating a Module

Modules encapsulate a business domain. Each module follows a fixed file structure:

```
modules/account/
├── module.go           # Module registration (implements broth.Module)
├── handler.go          # HTTP handlers (presentation layer)
├── service.go          # Business logic (application layer)
├── model.go            # Domain model + validation
├── repository.go       # Repository interface
├── routes.go           # Route definitions
├── forms.go            # Form binding definitions
├── internal/
│   └── store/
│       └── postgres.go # Repository implementation (SQL)
└── templates/
    └── account/
        └── login.html
```

Example module (see `modules/account/` for a complete reference implementation):

```go
// modules/account/module.go
package account

import (
    "github.com/source-maker/broth"
    "github.com/source-maker/broth/auth"
    "github.com/source-maker/broth/log"
    "github.com/source-maker/broth/render"
)

type Module struct {
    handler *Handler
    service *Service
}

func NewModule(repo Repository, hasher auth.PasswordHasher, tokenSvc *auth.TokenService, renderer *render.Renderer, logger *log.Logger) *Module {
    svc := NewService(repo, hasher, logger)
    svc.tokenService = tokenSvc
    handler := NewHandler(svc, renderer)
    return &Module{handler: handler, service: svc}
}

func (m *Module) Name() string { return "account" }

var _ broth.Module = (*Module)(nil) // compile-time check
```

## Configuration

Broth uses environment variables with struct tag binding. Create a `.env` file (or set environment variables directly):

```bash
# Server
APP_ADDR=:8080
APP_ENV=development

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/myapp?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m

# Logging
LOG_LEVEL=info

# Security (required in production)
# IMPORTANT: Generate with `openssl rand -hex 32` for production use
SESSION_KEY=your-32-byte-session-key-here!!!
JWT_SECRET=your-jwt-secret-key
CORS_ORIGINS=https://myapp.com
```

Bind configuration in your app:

```go
import "github.com/source-maker/broth/config"

type AppConfig struct {
    Addr       string `env:"APP_ADDR" default:":8080"`
    Env        string `env:"APP_ENV"  default:"development"`
    SessionKey string `env:"SESSION_KEY" default:"broth-dev-session-key-32b!"`
    JWTSecret  string `env:"JWT_SECRET" default:"broth-dev-jwt-secret-key-32byte"`
}

var cfg AppConfig
config.MustBind(&cfg)
```

## Running the Example Application

```bash
# Run the example (includes greeting module + account module)
go run ./cmd/example

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/greeting/
curl http://localhost:8080/greeting/World
curl -X POST -H "Content-Type: application/json" -d '{"name":"Gopher"}' http://localhost:8080/greeting/

# Account module endpoints
curl http://localhost:8080/account/login      # Login page (SSR)
curl http://localhost:8080/account/register   # Register page (SSR)
```

## Background Processing

### Job Queue & Worker

```go
enqueuer := job.NewEnqueuer(logger)
worker := job.NewWorker(enqueuer, job.WorkerConfig{
    Concurrency:     4,
    ShutdownTimeout: 30 * time.Second,
}, logger)
worker.Start(ctx)
defer worker.Shutdown(ctx)

// Enqueue a job from anywhere
enqueuer.Enqueue(ctx, myJob, job.WithQueue("emails"))
```

### Scheduler (Cron)

```go
scheduler := schedule.NewScheduler(enqueuer, logger, nil) // nil = no leader election
scheduler.Register(
    schedule.Definition{
        Name: "daily-cleanup",
        Cron: "0 3 * * *",        // 3 AM UTC daily
        Job:  &CleanupJob{},
        Overlap: false,            // Skip if previous run still running
    },
    schedule.Definition{
        Name:     "hourly-sync",
        Cron:     "0 * * * *",
        Job:      &SyncJob{},
        Timezone: jst,             // Per-definition timezone
    },
)
go scheduler.Start(ctx) // Blocks until shutdown
defer scheduler.Shutdown(ctx)
```

For multi-instance deployments, use `PgLeaderElector` (PostgreSQL advisory locks) to ensure only one instance runs schedules:

```go
leader := schedule.NewPgLeaderElector(db)
defer leader.Close(ctx)

scheduler := schedule.NewScheduler(enqueuer, logger, leader,
    schedule.WithOnError(func(name string, err error) {
        slog.Error("schedule failed", "job", name, "error", err)
    }),
)
```

### Development Mode

For development with auto-reload, use [air](https://github.com/air-verse/air):

```bash
go install github.com/air-verse/air@latest
air init
air
```

## Database

### Bob Code Generation (Type-Safe Data Access)

Broth uses [Bob](https://github.com/stephenafamo/bob) for database-first, type-safe code generation. The workflow is: **migrate → generate → compile**.

```bash
# 1. Apply migrations to your dev database
export PSQL_DSN="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
goose -dir db/migrations postgres "$PSQL_DSN" up

# 2. Generate type-safe models from the live schema
go generate ./db/...

# 3. Use generated models in your repository implementations
```

Repository implementations accept `bob.Executor` for transparent transaction participation:

```go
// modules/account/internal/store/postgres.go
type UserStore struct{ exec bob.Executor }

func New(exec bob.Executor) *UserStore { return &UserStore{exec: exec} }

func (s *UserStore) FindByID(ctx context.Context, id int64) (*User, error) {
    exec := db.ExecutorFromContext(ctx, s.exec)
    // exec is bob.Tx inside RunInTx, bob.DB otherwise
    return models.FindUser(ctx, exec, id)
}
```

Module wiring passes `database.BobDB()` as the executor:

```go
repo := store.New(database.BobDB())
```

Configuration: see [`config/bobgen.yaml`](config/bobgen.yaml) for generation options.

### Migrations

Broth wraps [goose](https://github.com/pressly/goose) for SQL-file based migrations via `broth/migrate`:

```go
import "github.com/source-maker/broth/migrate"

migrator := migrate.New(db, "db/migrations")
migrator.Up(ctx)     // Apply all pending migrations
migrator.Down(ctx)   // Rollback the last migration
migrator.Status(ctx) // List migration status
```

You can also use the goose CLI directly:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest

goose -dir db/migrations create create_users sql
goose -dir db/migrations postgres "$DATABASE_URL" up
goose -dir db/migrations postgres "$DATABASE_URL" down
goose -dir db/migrations postgres "$DATABASE_URL" status
```

Migration files follow the naming convention: `{NNN}_{name}.{up,down}.sql`

```sql
-- db/migrations/001_create_users.up.sql
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    password   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- db/migrations/001_create_users.down.sql
DROP TABLE IF EXISTS users;
```

## Testing

### Running Tests

```bash
# Run all unit tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Integration Tests (Database)

Integration tests require a PostgreSQL database. Set the `TEST_DATABASE_URL` environment variable:

```bash
export TEST_DATABASE_URL="postgres://user:pass@localhost:5432/myapp_test?sslmode=disable"
go test ./...
```

### Test Structure

| Layer | Test Approach | Location |
|-------|---------------|----------|
| **model** | Pure unit tests (no mocks) | `modules/{mod}/model_test.go` |
| **service** | Mock repository interface | `modules/{mod}/service_test.go` |
| **handler** | `httptest` + mock service | `modules/{mod}/handler_test.go` |
| **internal/store** | Integration with test DB | `modules/{mod}/internal/store/postgres_test.go` |

### Using Test Utilities

```go
import "github.com/source-maker/broth/testutil"

func TestMyHandler(t *testing.T) {
    handler := setupHandler()
    srv := testutil.NewTestServer(t, handler)  // auto-cleanup

    resp, err := http.Get(srv.URL + "/health")
    if err != nil {
        t.Fatal(err)
    }
    testutil.AssertStatus(t, resp, http.StatusOK)
}
```

## Architecture

Broth follows a 4-layer architecture with strict dependency rules:

```
┌──────────────────────────────────────────┐
│  HTTP Layer (Presentation)               │
│  handler.go / routes.go / middleware     │
├──────────────────────────────────────────┤
│  Application Layer (Service)             │
│  service.go  ← business logic goes HERE │
├──────────────────────────────────────────┤
│  Domain Layer                            │
│  model.go / forms.go                     │
├──────────────────────────────────────────┤
│  Data Access Layer (Infrastructure)      │
│  repository.go (interface)               │
│  internal/store/ (implementation)        │
└──────────────────────────────────────────┘
```

**Dependency rules:**
- Dependencies flow **downward only**
- `handler` -> `service`, `render` (never `repository` or `database/sql`)
- `service` -> `repository` (interface), `model` (never `net/http`)
- `model` -> no external dependencies

For detailed design documentation, see:
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) -- Core layered architecture
- [MODULE_DESIGN.md](docs/MODULE_DESIGN.md) -- Module boundaries and patterns
- [PROJECT_STRUCTURE.md](docs/PROJECT_STRUCTURE.md) -- Directory structure and CLI
- [SECURITY_DESIGN.md](docs/SECURITY_DESIGN.md) -- Auth, CSRF, security headers
- [DATA_ACCESS_DESIGN.md](docs/DATA_ACCESS_DESIGN.md) -- DB access, Bob, migrations
- [CONCURRENCY_DESIGN.md](docs/CONCURRENCY_DESIGN.md) -- Jobs, scheduler
- [DECISION_LOG.md](docs/DECISION_LOG.md) -- 36+ Architecture Decision Records

## Project Structure

```
myapp/
├── cmd/myapp/main.go           # Entry point (wiring + server startup)
├── config/
│   ├── app.go                  # Application config struct
│   ├── database.go             # Database config
│   ├── routes.go               # Route prefix mapping
│   ├── middleware.go           # Global middleware chain
│   └── bobgen.yaml             # Bob code generation config
├── modules/
│   ├── account/                # Feature module (account management)
│   │   ├── module.go           # Module registration
│   │   ├── handler.go          # HTTP handlers
│   │   ├── service.go          # Business logic
│   │   ├── model.go            # Domain models
│   │   ├── repository.go       # Repository interface
│   │   ├── routes.go           # Route definitions
│   │   ├── forms.go            # Form definitions
│   │   ├── internal/store/     # Repository implementations
│   │   └── templates/account/  # Module-specific templates
│   └── shared/                 # Cross-module shared types
├── db/migrations/              # SQL migration files
├── templates/                  # Shared layouts and components
│   ├── layouts/base.html
│   └── components/
├── static/                     # CSS, JS, images
├── CLAUDE.md                   # AI coding conventions
├── .env                        # Environment variables (gitignored)
├── .env.example                # Environment variable template
├── Makefile                    # Development tasks
└── go.mod
```

## Design Principles

| Priority | Principle | Rule |
|----------|-----------|------|
| **P1** | Go Idiom | No custom Context, no DI container, no `interface{}`/`any` in public API |
| **P2** | AI Convergence | "Where do I write this?" has exactly ONE answer |
| **P3** | Team Ops (7+2) | Single binary, Secure by Default |
| **P4** | YAGNI | Phase 1 simple, reserve expansion paths |

## Technology Stack

| Layer | Choice | Status |
|-------|--------|--------|
| Router | `net/http` (Go 1.22+) | Implemented |
| Template | `html/template` (thread-safe cache) | Implemented |
| Logging | `log/slog` | Implemented |
| JWT | `golang-jwt/jwt` v5 | Implemented |
| Password | `golang.org/x/crypto` (bcrypt) | Implemented |
| Session | AES-GCM encrypted cookies | Implemented |
| Rate Limit | `golang.org/x/time` (Token Bucket) | Implemented |
| Data Access | PostgreSQL + Bob (`bob.Executor`, codegen) | Implemented |
| Migrations | `pressly/goose` v3 | Implemented |
| Scheduler | `robfig/cron` v3 (leader election, overlap) | Implemented |
| Testing | `testing` + `httptest` | Implemented |

## Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| **Phase 1** | Core skeleton (router, middleware, db, config, module system) | Done |
| **Phase 2** | Auth (JWT, session, bcrypt), security middleware (CSRF, CORS, headers, rate limit), goose migrations, account module | Done |
| **Phase 3** | Bob integration (db wrappers, codegen workflow), scheduler (cron, leader election, overlap), persistent job backend (PostgreSQL), example migrations | Done |
| **Phase 4** | Admin panel, CLI tooling (`broth new`, `broth generate`), OpenAPI/Swagger, OpenTelemetry | Planned |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/my-feature`)
3. Follow the [CLAUDE.md](CLAUDE.md) coding conventions
4. Write tests for your changes
5. Run `go test ./...` and ensure all tests pass
6. Run `go vet ./...` and `gofmt -w .`
7. Commit your changes (`git commit -m 'Add my feature'`)
8. Push to the branch (`git push origin feature/my-feature`)
9. Open a Pull Request

## License

[MIT License](LICENSE)
