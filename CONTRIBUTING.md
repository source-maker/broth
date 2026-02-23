# Contributing to Broth

Thank you for your interest in contributing to Broth! This document provides guidelines for contributing to the project.

## Getting Started

### Prerequisites

- **Go 1.24+** ([download](https://go.dev/dl/))
- **PostgreSQL 15+** (for integration tests)
- **Docker** (optional, for local database via `docker compose`)

### Setup

```bash
git clone https://github.com/source-maker/broth.git
cd broth

# Start local database
make db-up

# Run tests
make test
```

## How to Contribute

### Reporting Bugs

Open a [bug report issue](https://github.com/source-maker/broth/issues/new?template=bug_report.md) with:
- Go version and OS
- Steps to reproduce
- Expected vs actual behavior
- Minimal reproduction code if possible

### Suggesting Features

Open a [feature request issue](https://github.com/source-maker/broth/issues/new?template=feature_request.md) with:
- Problem description (what's missing or inconvenient)
- Proposed solution
- Alternatives considered

### Architecture Proposals

For significant design changes, use **GitHub Discussions** (Architecture Proposals category):

1. Open a Discussion describing the context, options, and recommendation
2. The community discusses and reaches consensus
3. If accepted, the proposal is formalized as an [ADR](docs/adr/README.md)
4. Implementation follows as a PR referencing the ADR

See [docs/adr/README.md](docs/adr/README.md) for the full workflow.

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main` (`git checkout -b feature/my-feature`)
3. Follow the coding conventions (see below)
4. Write tests for your changes
5. Ensure all checks pass:
   ```bash
   make test
   make vet
   gofmt -w .
   ```
6. Commit with a clear, descriptive message
7. Open a Pull Request against `main`

## Coding Conventions

Broth follows strict architectural conventions documented in [CLAUDE.md](CLAUDE.md). Key rules:

### Layer Rules

```
handler.go → service.go → repository.go (interface)
                 │                ↑
                 ↓                │
             model.go    internal/store/ (impl)
```

- **handler.go**: HTTP concerns only (parse request, call service, render response)
- **service.go**: Business logic only (no `net/http` imports)
- **model.go**: Domain models and validation (no `database/sql` or `net/http` imports)
- **repository.go**: Interface definition (implementation in `internal/store/`)

### File Placement

Every concern has exactly ONE place to go. See the table in [CLAUDE.md](CLAUDE.md) for the complete mapping.

### Naming

| Target | Style | Example |
|--------|-------|---------|
| Package | lowercase singular | `account`, `article` |
| File | snake_case.go | `handler.go`, `service_auth.go` |
| Type | PascalCase | `User`, `Service` |
| Constructor | `New` + type | `NewService(...)` |
| Input type | `{Action}Input` | `RegisterInput` |
| Error format | `module: action: %w` | `fmt.Errorf("account: create: %w", err)` |

### Do NOT

- Put business logic in handlers
- Use `interface{}`/`any` in public APIs
- Use reflection outside `form`/`config`/`admin`
- Import across module `internal/` boundaries
- Use global variables for state

## Testing

| Layer | Approach |
|-------|----------|
| model | Pure unit tests |
| service | Mock repository interface |
| handler | `httptest` + mock service |
| internal/store | Integration test with test DB |

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
