# Broth Framework - AI Coding Guide

> Go web framework "Broth" -- Django-like batteries-included for Go
> Module path: `github.com/source-maker/broth`

## Design Principles (P1 > P2 > P3 > P4)

| Priority | Principle | Rule |
|---|---|---|
| **P1** | Go idiom | No custom Context, no DI container, no `interface{}`/`any` in public API, reflection only in form/config/admin |
| **P2** | AI convergence | "Where do I write this?" has exactly ONE answer |
| **P3** | Team ops (7+2) | Single binary, Secure by Default |
| **P4** | YAGNI | Phase 1 simple, reserve expansion paths |

## File Placement (ONE answer per concern)

| What to write | Where to put it |
|---|---|
| HTTP handler | `modules/{mod}/handler.go` |
| Business logic | `modules/{mod}/service.go` |
| Domain model + validation | `modules/{mod}/model.go` |
| Repository interface | `modules/{mod}/repository.go` |
| Repository impl (SQL/Bob) | `modules/{mod}/internal/store/postgres.go` |
| Routes | `modules/{mod}/routes.go` |
| Form binding | `modules/{mod}/forms.go` |
| Module init + DI wiring | `modules/{mod}/module.go` |
| Module templates | `modules/{mod}/templates/{mod}/*.html` |
| Shared layouts | `templates/layouts/*.html` |
| Shared components | `templates/components/*.html` |
| Migrations | `db/migrations/{NNN}_{name}.{up,down}.sql` |
| App/DB/Route config | `config/{app,database,routes,middleware}.go` |
| Entry point | `cmd/{project}/main.go` |
| Cross-module shared types | `modules/shared/*.go` |
| Bob codegen config | `config/bobgen.yaml` |
| Scheduled jobs | `modules/{mod}/jobs.go` |

## Layer Rules (STRICT)

```
handler.go ──→ service.go ──→ repository.go (interface)
                    │                 ↑
                    ↓                 │
                model.go    internal/store/ (impl)
```

**Allowed imports:**
- handler → service, render
- service → repository (interface), model, broth/db (TxManager)
- internal/store → model, bob, broth/db
- model → (no external deps)

**Forbidden imports:**
- handler → repository, internal/store, database/sql
- service → net/http
- model → database/sql, net/http

## Naming Conventions

| Target | Style | Example |
|---|---|---|
| Package | lowercase singular | `account`, `article` |
| File | snake_case.go | `handler.go`, `service_auth.go` |
| Type | PascalCase | `User`, `Service`, `Handler` |
| Constructor | `New` + type | `NewService(repo, txMgr, logger)` |
| Input type | `{Action}Input` | `RegisterInput`, `UpdateArticleInput` |
| Form type | `{Entity}Form` | `RegisterForm`, `LoginForm` |
| Template | snake_case.html | `login.html`, `user_profile.html` |
| URL | kebab-case | `/user-profile`, `/change-password` |
| Error format | `module: action: %w` | `fmt.Errorf("account: create: %w", err)` |

## Code Patterns

### Handler (presentation only)
```go
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request (form/JSON)
    // 2. Call service: result, err := h.svc.Register(r.Context(), input)
    // 3. Render response (HTML or JSON, never business logic here)
}
```

### Service (business logic only)
```go
func (s *Service) Register(ctx context.Context, input RegisterInput) (*User, error) {
    if err := input.Validate(); err != nil {
        return nil, fmt.Errorf("account: register: %w", err)
    }
    // Business rules here
    // Access s.repo for data (transaction via s.txMgr if needed)
    return user, nil
}
```

### Repository (interface in public, impl in internal/)
```go
// modules/account/repository.go -- interface definition
type Repository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id int64) (*User, error)
}

// modules/account/internal/store/postgres.go -- Bob-based implementation
type UserStore struct { exec bob.Executor }
func New(exec bob.Executor) *UserStore { return &UserStore{exec: exec} }
```

### Module registration
```go
func NewModule(database *db.Database, renderer *render.Renderer, logger *log.Logger) *Module {
    repo := store.New(database.BobDB()) // bob.Executor for type-safe queries
    svc := NewService(repo, database.TxManager(), logger)
    handler := NewHandler(svc, renderer)
    return &Module{handler: handler, service: svc}
}
var _ broth.Module = (*Module)(nil) // compile-time check
```

## Technology Stack

| Layer | Choice | Reason |
|---|---|---|
| Data access | **Bob** (`github.com/stephenafamo/bob`) | DB-first code gen, type-safe, no reflection, no `interface{}` |
| Migrations | **goose** (wrapped in `broth/migrate`) | SQL file based |
| Router | `net/http` (Go 1.22+ patterns) | Standard library |
| Template | `html/template` (extended) | Auto-escape, no external deps |
| Logging | `log/slog` | Standard library |
| JWT | `golang-jwt/jwt` v5 | Proven security library |
| Scheduler | `robfig/cron` v3 (wrapped in `broth/schedule`) | Cron parsing, overlap, leader election |
| Code gen | `go generate ./db/...` → `bobgen-psql` | Type-safe models + test factories |

## DO NOT

- Put business logic in handlers (fat controller)
- Use `interface{}`/`any` in public APIs
- Use reflection outside form/config/admin
- Import `database/sql` in `model.go`
- Import `net/http` in `service.go`
- Cross-module `internal/` imports
- Use global variables for state
- Use GORM (ADR-D007: rejected for `interface{}` proliferation + heavy reflection)
- Nest packages deeper than `modules/{name}/internal/store/`

## Testing

| Layer | Test approach |
|---|---|
| model | Pure unit tests (no mocks needed) |
| service | Mock repository interface |
| handler | `httptest` + mock service |
| internal/store | Integration test with test DB |
| factories | Bob test factories (`bobgen-psql` generated) |

## Design Documents

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) -- Core layered architecture
- [MODULE_DESIGN.md](docs/MODULE_DESIGN.md) -- Module boundaries
- [PROJECT_STRUCTURE.md](docs/PROJECT_STRUCTURE.md) -- Directory structure, CLI
- [CONCURRENCY_DESIGN.md](docs/CONCURRENCY_DESIGN.md) -- Jobs, scheduler
- [SECURITY_DESIGN.md](docs/SECURITY_DESIGN.md) -- Auth, CSRF
- [DATA_ACCESS_DESIGN.md](docs/DATA_ACCESS_DESIGN.md) -- DB access, Bob
- [DECISION_LOG.md](docs/DECISION_LOG.md) -- 36 ADRs
